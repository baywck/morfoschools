package app

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

// Blueprint clone — apply a blueprint_template to an exam, creating an
// exam_blueprints + exam_blueprint_slots snapshot per ADR-0010.
//
// This is the bridge between the library (templates) and per-exam
// authoring (exam blueprints). After clone:
//   - exam.exam_blueprints row exists with source_template_id +
//     source_template_version pointing to the template at clone time
//   - exam_blueprint_slots mirror the template_slots field-by-field
//   - subsequent template edits do NOT propagate to the exam blueprint

// handleCloneBlueprintToExam implements:
//   POST /api/v1/exams/{id}/blueprint/clone
//   Body: { "templateId": "uuid", "replace": false }
//
// Errors:
//   - 404 if template or exam not found / no access
//   - 409 if exam already has a blueprint and replace=false
//   - 409 if exam is not in 'draft' status (cannot mutate published exams)

func (a *App) handleCloneBlueprintToExam(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	var req struct {
		TemplateID string `json:"templateId"`
		Replace    bool   `json:"replace"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	if req.TemplateID == "" {
		writeValidationError(w, map[string]string{"templateId": "templateId is required"}, r)
		return
	}

	// Verify caller has read access to the source template
	if !a.requireBlueprintAccess(w, r, req.TemplateID, ActionRead) {
		return
	}

	// Confirm the exam is in a clone-eligible status
	var examStatus string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT status FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&examStatus); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found", r)
		return
	}
	if examStatus != "draft" {
		writeErrorJSON(w, http.StatusConflict, "invalid_state",
			"Cannot apply blueprint to a non-draft exam", r)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
			"Could not start transaction", r)
		return
	}
	defer tx.Rollback()

	// Existing blueprint?
	var existingID string
	_ = tx.QueryRowContext(r.Context(),
		`SELECT id::text FROM exam_blueprints WHERE exam_id = $1`, examID,
	).Scan(&existingID)
	if existingID != "" {
		if !req.Replace {
			writeErrorJSON(w, http.StatusConflict, "blueprint_exists",
				"Exam already has a blueprint. Pass replace=true to overwrite.", r)
			return
		}
		// Replace: delete existing blueprint (slots cascade). Question
		// FK to old slots is set to NULL by ON DELETE SET NULL on the
		// exam_questions.blueprint_slot_id column.
		if _, err := tx.ExecContext(r.Context(),
			`DELETE FROM exam_blueprints WHERE id = $1`, existingID,
		); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
				"Could not replace existing blueprint", r)
			return
		}
	}

	// Snapshot the source template
	var (
		title, description, blueprintType, status string
		curriculumID, subjectCode, gradeOrPhase   string
		strictCoverage                            bool
		version                                   int
	)
	err = tx.QueryRowContext(r.Context(), `
		SELECT title, COALESCE(description, ''), curriculum_id::text,
		       COALESCE(subject_code, ''), COALESCE(grade_or_phase, ''),
		       blueprint_type, strict_coverage, status, version
		  FROM blueprint_templates
		 WHERE id = $1 AND tenant_id = $2`,
		req.TemplateID, tenantID,
	).Scan(&title, &description, &curriculumID, &subjectCode, &gradeOrPhase,
		&blueprintType, &strictCoverage, &status, &version)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Template not found", r)
		return
	}

	// Insert exam blueprint
	var newBlueprintID string
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO exam_blueprints (
		    tenant_id, exam_id, source_template_id, source_template_version,
		    created_via, title, description, curriculum_id,
		    subject_code, grade_or_phase, blueprint_type,
		    strict_coverage, status
		) VALUES ($1, $2, $3, $4, 'template_clone', $5, NULLIF($6, ''),
		          $7, NULLIF($8, ''), NULLIF($9, ''), $10, $11, 'draft')
		RETURNING id::text`,
		tenantID, examID, req.TemplateID, version,
		title, description, curriculumID,
		subjectCode, gradeOrPhase, blueprintType, strictCoverage,
	).Scan(&newBlueprintID)
	if err != nil {
		a.logger.Error("clone blueprint header failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
			"Could not create exam blueprint", r)
		return
	}

	// Snapshot slots. INSERT ... SELECT preserves all field values
	// including stimulus_id (live FK to library, not copied content).
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO exam_blueprint_slots (
		    exam_blueprint_id, position,
		    competency_id, competency_code, competency_description,
		    materi, indikator, cognitive_level, difficulty,
		    question_type, points, stimulus_id,
		    akm_konten, akm_konteks, akm_proses, akm_level
		)
		SELECT $1, position,
		       competency_id, competency_code, competency_description,
		       materi, indikator, cognitive_level, difficulty,
		       question_type, points, stimulus_id,
		       akm_konten, akm_konteks, akm_proses, akm_level
		  FROM blueprint_template_slots
		 WHERE template_id = $2
		 ORDER BY position`,
		newBlueprintID, req.TemplateID,
	); err != nil {
		a.logger.Error("clone blueprint slots failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
			"Could not snapshot slots", r)
		return
	}

	// Refresh totals on the new exam blueprint
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE exam_blueprints SET
		    total_slots  = (SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1),
		    total_points = (SELECT COALESCE(SUM(points), 0) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1)
		 WHERE id = $1`, newBlueprintID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
			"Could not finalize blueprint totals", r)
		return
	}

	// Auto-create one empty question per slot (Phase 9.9 UX). Each
	// question carries the slot's pedagogical metadata via the
	// blueprint_slot_id FK; question_type and points are copied from
	// the slot at create time so the canvas can render an editable row
	// pre-filled with type/points without round-tripping. Slot's
	// kisi-kisi metadata (KD/materi/indikator/etc.) lives on the slot
	// row and is read through the FK — no need to denormalize.
	//
	// Place all auto-created questions in the exam's first section
	// (section_id is NOT NULL since migration 000018). Sort order
	// follows slot.position so the canvas displays them in template
	// order. Skipped silently when the exam already has questions —
	// avoids duplicating during a replace=true reapply.
	var firstSectionID string
	if err := tx.QueryRowContext(r.Context(),
		`SELECT id::text FROM exam_sections
		  WHERE exam_id = $1 AND tenant_id = $2
		  ORDER BY sort_order ASC, created_at ASC LIMIT 1`,
		examID, tenantID,
	).Scan(&firstSectionID); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
			"Could not resolve default section for auto-created questions", r)
		return
	}

	var existingQuestionCount int
	_ = tx.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM exam_questions WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&existingQuestionCount)
	if existingQuestionCount == 0 {
		auth := AuthFromContext(r.Context())
		// One round-trip: insert N questions from the slot rows. The
		// SELECT pulls slot id + question_type + points and the INSERT
		// fills the rest with sensible defaults (empty content, sort
		// order = position so canvas renders in template order, scoring
		// mode = correct_all default).
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO exam_questions (
			    tenant_id, exam_id, section_id, blueprint_slot_id,
			    question_type, content, points, sort_order, scoring_mode,
			    content_hash, created_by
			)
			SELECT $1, $2, $3::uuid, s.id,
			       s.question_type, '', s.points, s.position, 'correct_all',
			       NULL, $4
			  FROM exam_blueprint_slots s
			 WHERE s.exam_blueprint_id = $5
			 ORDER BY s.position`,
			tenantID, examID, firstSectionID, auth.UserID, newBlueprintID,
		); err != nil {
			a.logger.Error("auto-create questions for slots failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
				"Could not auto-create questions for cloned slots", r)
			return
		}
	}

	// Coverage (filled vs total) is computed at READ time by
	// /api/v1/exams/{id}/slots-with-questions — there is no
	// exam_blueprints.filled_slots column. The auto-created shells
	// above already establish the slot → question links, so the
	// coverage badge will read 100% on the next refresh.

	// Auto-set the exam's kisi-kisi toggle (ADR-0012). Applying a
	// blueprint always flips uses_kisi_kisi=true so the canvas switches
	// to slot-first rendering. AKM detection now reads from
	// blueprint.blueprint_type at the consumer site; there is no
	// exam-level AKM column.
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE exams SET uses_kisi_kisi = true, updated_at = now()
		  WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
			"Could not enable kisi-kisi on exam", r)
		return
	}

	// AKM auto-grouping (ADR-0012). When the cloned blueprint is AKM,
	// scan the cloned slots for stimulus_id references and pre-create
	// exam_question_groups for each unique stimulus. This is best-effort
	// — templates without stimulus pre-assignments fall through silently
	// and the user creates groups manually from the canvas toolbar.
	if blueprintType == "akm_literasi" || blueprintType == "akm_numerasi" {
		if err := a.autoCreateAkmGroups(r.Context(), tx, tenantID, examID, newBlueprintID); err != nil {
			a.logger.Error("akm auto-grouping failed (non-fatal)", "error", err)
			// Non-fatal: clone succeeds even if grouping fails.
		}
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "clone_failed",
			"Could not commit clone", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"exam_blueprints.cloned", "exam_blueprint", newBlueprintID, r)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                    newBlueprintID,
		"sourceTemplateId":      req.TemplateID,
		"sourceTemplateVersion": version,
		"createdVia":            "template_clone",
	})
}

// autoCreateAkmGroups scans the just-cloned exam_blueprint_slots for
// stimulus_id references, dedupes, and creates one
// exam_question_groups row per unique stimulus. The questions that
// will eventually fill those slots can attach to the matching group.
//
// Best-effort: failures are logged at the call site and the clone
// commits regardless. Templates without stimulus pre-assignments fall
// through silently — callers handle the empty case (no groups
// created, user creates manually). Documented in ADR-0012.
func (a *App) autoCreateAkmGroups(
	ctx context.Context, tx *sql.Tx, tenantID, examID, blueprintID string,
) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT s.stimulus_id::text
		  FROM exam_blueprint_slots s
		 WHERE s.exam_blueprint_id = $1
		   AND s.stimulus_id IS NOT NULL`,
		blueprintID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var stimulusIDs []string
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err == nil && sid != "" {
			stimulusIDs = append(stimulusIDs, sid)
		}
	}
	if len(stimulusIDs) == 0 {
		return nil
	}

	nextOrder := 0
	_ = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(display_order), -1) + 1 FROM exam_question_groups WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&nextOrder)

	for _, sid := range stimulusIDs {
		var title, body string
		if err := tx.QueryRowContext(ctx,
			`SELECT title, content FROM stimuli WHERE id = $1 AND tenant_id = $2`,
			sid, tenantID,
		).Scan(&title, &body); err != nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO exam_question_groups (
			    tenant_id, exam_id, stimulus_id,
			    stimulus_title_snapshot, stimulus_body_snapshot,
			    group_type, display_order
			) VALUES ($1, $2, $3::uuid, $4, $5, 'stimulus', $6)`,
			tenantID, examID, sid, title, body, nextOrder,
		); err != nil {
			return err
		}
		nextOrder++
	}
	return nil
}

// handleExportExamBlueprintToTemplate (Phase 9.10) — reverse of
// handleCloneBlueprintToExam: takes the exam's current
// exam_blueprint_slots snapshot and materializes a NEW
// blueprint_templates row + blueprint_template_slots in 'draft'
// status, owned by the caller. Useful when teachers iterate on a
// kisi-kisi inside a sandbox exam and want to lift the result back to
// the library for reuse.
//
//   POST /api/v1/exams/{id}/blueprint/export
//   Body: { "title": string, "description"?: string,
//           "subjectCode"?: string, "gradeOrPhase"?: string }
//
// The new template inherits curriculum / blueprint_type /
// strict_coverage from the source exam blueprint; everything else can
// be overridden in the request body. Slot snapshot mirrors the
// existing ones field-by-field. Stimulus FKs are preserved (the new
// template references the same library entries).
func (a *App) handleExportExamBlueprintToTemplate(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionRead) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	var req struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		SubjectCode  string `json:"subjectCode"`
		GradeOrPhase string `json:"gradeOrPhase"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeValidationError(w, map[string]string{"title": "title is required"}, r)
		return
	}

	// Resolve source exam_blueprint and inherit curriculum + type +
	// strictness. If the exam has no blueprint, refuse with a hint.
	var (
		srcID                                              string
		curriculumID, blueprintType, srcSubject, srcGrade  string
		strictCoverage                                     bool
	)
	err := a.db.QueryRowContext(r.Context(), `
		SELECT id::text, curriculum_id::text, blueprint_type,
		       COALESCE(subject_code, ''), COALESCE(grade_or_phase, ''),
		       strict_coverage
		  FROM exam_blueprints
		 WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&srcID, &curriculumID, &blueprintType, &srcSubject, &srcGrade, &strictCoverage)
	if err != nil {
		writeErrorJSON(w, http.StatusConflict, "no_blueprint",
			"Exam does not have a blueprint to export. Apply or build one first.", r)
		return
	}

	// Caller-supplied subject/grade override the inherited values.
	if s := strings.TrimSpace(req.SubjectCode); s != "" {
		srcSubject = s
	}
	if g := strings.TrimSpace(req.GradeOrPhase); g != "" {
		srcGrade = g
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "export_failed",
			"Could not start transaction", r)
		return
	}
	defer tx.Rollback()

	auth := AuthFromContext(r.Context())
	var newTemplateID string
	if err := tx.QueryRowContext(r.Context(), `
		INSERT INTO blueprint_templates (
		    tenant_id, owner_user_id, title, description,
		    curriculum_id, subject_code, grade_or_phase, blueprint_type,
		    strict_coverage, status, version
		) VALUES ($1, $2, $3, NULLIF($4,''),
		          $5, NULLIF($6,''), NULLIF($7,''), $8,
		          $9, 'draft', 1)
		RETURNING id::text`,
		tenantID, auth.UserID, req.Title, strings.TrimSpace(req.Description),
		curriculumID, srcSubject, srcGrade, blueprintType,
		strictCoverage,
	).Scan(&newTemplateID); err != nil {
		a.logger.Error("export blueprint template insert failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "export_failed",
			"Could not create destination template", r)
		return
	}

	// Mirror exam_blueprint_slots into blueprint_template_slots. Same
	// columns minus the FK direction. Position is preserved so the
	// template ordering matches the exam.
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO blueprint_template_slots (
		    template_id, position,
		    competency_id, competency_code, competency_description,
		    materi, indikator, cognitive_level, difficulty,
		    question_type, points, stimulus_id,
		    akm_konten, akm_konteks, akm_proses, akm_level
		)
		SELECT $1, position,
		       competency_id, competency_code, competency_description,
		       materi, indikator, cognitive_level, difficulty,
		       question_type, points, stimulus_id,
		       akm_konten, akm_konteks, akm_proses, akm_level
		  FROM exam_blueprint_slots
		 WHERE exam_blueprint_id = $2
		 ORDER BY position`,
		newTemplateID, srcID,
	); err != nil {
		a.logger.Error("export blueprint template slots failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "export_failed",
			"Could not snapshot slots into template", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "export_failed",
			"Could not commit export", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates.exported_from_exam", "blueprint_template", newTemplateID, r)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     newTemplateID,
		"status": "draft",
	})
}
