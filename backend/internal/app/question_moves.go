package app

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

// Question moves + question groups (ADR-0012).
//
// The drag-and-drop canvas needs an atomic per-question reposition that
// updates section_id / group_id / sort_order in one round-trip without
// touching the slot binding (kisi-kisi anchoring is pedagogical, not
// visual). Plus light CRUD for exam_question_groups so users can create
// stimulus clusters and the AKM auto-grouping path has somewhere to
// hang its writes.
//
// Reordering policy: the caller (frontend) is responsible for sortOrder
// values. When multiple questions need to shift, the frontend computes
// the new order locally and sends one move call per affected question.
// The backend does NOT propagate transitive shifts; that keeps each
// move O(1) and avoids the "reorder one, server rewrites N rows"
// pattern that fights optimistic UI.

func (a *App) registerQuestionMoveRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/exams/{id}/questions/move", a.handleMoveQuestion)
	mux.HandleFunc("GET /api/v1/exams/{id}/groups", a.handleListExamGroups)
	mux.HandleFunc("POST /api/v1/exams/{id}/groups", a.handleCreateQuestionGroup)
	mux.HandleFunc("PATCH /api/v1/groups/{groupId}", a.handleUpdateQuestionGroup)
	mux.HandleFunc("DELETE /api/v1/groups/{groupId}", a.handleDeleteQuestionGroup)
}

// GET /api/v1/exams/{id}/groups
//
// Returns the exam_question_groups for this exam plus stimulus snapshot
// summary and a question_count per group. Used by the inline accordion
// canvas (ADR-0012 UX rewrite) to render group cards and let users pick
// existing groups when binding a question.
func (a *App) handleListExamGroups(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionRead) {
		return
	}

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT g.id::text, g.section_id::text, g.stimulus_id::text,
		       g.stimulus_title_snapshot, g.stimulus_body_snapshot,
		       g.group_type, g.display_order, g.created_at::text,
		       (SELECT COUNT(*) FROM exam_questions q WHERE q.group_id = g.id)
		  FROM exam_question_groups g
		 WHERE g.exam_id = $1 AND g.tenant_id = $2
		 ORDER BY g.display_order, g.created_at`,
		examID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load groups", r)
		return
	}
	defer rows.Close()

	type groupRow struct {
		ID                    string  `json:"id"`
		SectionID             *string `json:"sectionId,omitempty"`
		StimulusID            *string `json:"stimulusId,omitempty"`
		StimulusTitleSnapshot *string `json:"stimulusTitleSnapshot,omitempty"`
		StimulusBodySnapshot  *string `json:"stimulusBodySnapshot,omitempty"`
		GroupType             string  `json:"groupType"`
		DisplayOrder          int     `json:"displayOrder"`
		QuestionCount         int     `json:"questionCount"`
		CreatedAt             string  `json:"createdAt"`
	}

	out := make([]groupRow, 0)
	for rows.Next() {
		var g groupRow
		var sectionID, stimulusID, titleSnap, bodySnap sql.NullString
		if err := rows.Scan(
			&g.ID, &sectionID, &stimulusID,
			&titleSnap, &bodySnap,
			&g.GroupType, &g.DisplayOrder, &g.CreatedAt,
			&g.QuestionCount,
		); err != nil {
			continue
		}
		if sectionID.Valid && sectionID.String != "" {
			v := sectionID.String
			g.SectionID = &v
		}
		if stimulusID.Valid && stimulusID.String != "" {
			v := stimulusID.String
			g.StimulusID = &v
		}
		if titleSnap.Valid {
			v := titleSnap.String
			g.StimulusTitleSnapshot = &v
		}
		if bodySnap.Valid {
			v := bodySnap.String
			g.StimulusBodySnapshot = &v
		}
		out = append(out, g)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// POST /api/v1/exams/{id}/questions/move
//
// Body: { questionId, sectionId?, groupId?, sortOrder? }
//
// Atomic move. sectionId / groupId / sortOrder are all individually
// optional; pass empty string to clear (set NULL on section_id /
// group_id). blueprint_slot_id is intentionally untouched.
func (a *App) handleMoveQuestion(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	auth := AuthFromContext(r.Context())

	var req struct {
		QuestionID string  `json:"questionId"`
		SectionID  *string `json:"sectionId"`
		GroupID    *string `json:"groupId"`
		SortOrder  *int    `json:"sortOrder"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	if req.QuestionID == "" {
		writeValidationError(w, map[string]string{"questionId": "questionId is required"}, r)
		return
	}

	// Verify the question belongs to this exam in this tenant.
	var qExamID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT exam_id::text FROM exam_questions WHERE id = $1 AND tenant_id = $2`,
		req.QuestionID, tenantID,
	).Scan(&qExamID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Question not found", r)
		return
	}
	if qExamID != examID {
		writeValidationError(w, map[string]string{"questionId": "Question does not belong to this exam"}, r)
		return
	}

	// Validate referenced section / group are owned by this exam (and
	// tenant) when set. Empty string = clear.
	if req.SectionID != nil && *req.SectionID != "" {
		var sectionExamID string
		if err := a.db.QueryRowContext(r.Context(),
			`SELECT exam_id::text FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
			*req.SectionID, tenantID,
		).Scan(&sectionExamID); err != nil || sectionExamID != examID {
			writeValidationError(w, map[string]string{"sectionId": "Section does not belong to this exam"}, r)
			return
		}
	}
	if req.GroupID != nil && *req.GroupID != "" {
		if errs := a.validateGroupForExam(r.Context(), tenantID, examID, *req.GroupID); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}

	// Single transaction: section, group, sortOrder. Slot binding is
	// preserved by omission (we never touch blueprint_slot_id here).
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "move_failed", "Could not move question", r)
		return
	}
	defer tx.Rollback()

	parts := []string{"updated_at = now()"}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}
	if req.SectionID != nil {
		if *req.SectionID == "" {
			parts = append(parts, "section_id = NULL")
		} else {
			add("section_id", *req.SectionID)
		}
	}
	if req.GroupID != nil {
		if *req.GroupID == "" {
			parts = append(parts, "group_id = NULL")
		} else {
			add("group_id", *req.GroupID)
		}
	}
	if req.SortOrder != nil {
		add("sort_order", *req.SortOrder)
	}
	if len(args) == 0 && req.SectionID == nil && req.GroupID == nil && req.SortOrder == nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": req.QuestionID, "status": "no_change"})
		return
	}

	q := "UPDATE exam_questions SET " + joinComma(parts) +
		" WHERE id = $" + strconv.Itoa(idx) +
		" AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, req.QuestionID, tenantID)
	if _, err := tx.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "move_failed", "Could not move question", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "move_failed", "Could not finalize move", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "questions.move", "exam_question", req.QuestionID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": req.QuestionID, "status": "moved"})
}

// POST /api/v1/exams/{id}/groups
//
// Body: { sectionId?, stimulusId?, name?, sortOrder? }
//
// Creates an exam_question_groups row. The group lives inside a
// section (Phase 9.8 — every group belongs to a section). When
// sectionId is omitted the handler defaults to the exam's first
// section (lowest sort_order, oldest created_at) so the canvas always
// renders new groups inside a usable container. When stimulusId is set
// we snapshot the stimulus title + body at this instant (matches the
// snapshot-on-use lifecycle from ADR-0010). The lifecycle of the
// snapshot follows the parent exam's status; refresh is via a separate
// endpoint (handleSyncStimulusSnapshot in stimuli.go).
func (a *App) handleCreateQuestionGroup(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	auth := AuthFromContext(r.Context())

	var req struct {
		SectionID  *string `json:"sectionId"`
		StimulusID *string `json:"stimulusId"`
		Name       string  `json:"name"`
		SortOrder  *int    `json:"sortOrder"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	// Resolve section. Caller may pass an explicit sectionId; otherwise
	// fall back to the exam's first section (matches Phase 9.8
	// section-mandatory rule). Empty string is treated the same as
	// missing so the frontend can use a single helper signature.
	sectionID := ""
	if req.SectionID != nil && *req.SectionID != "" {
		var sectionExamID string
		if err := a.db.QueryRowContext(r.Context(),
			`SELECT exam_id::text FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
			*req.SectionID, tenantID,
		).Scan(&sectionExamID); err != nil || sectionExamID != examID {
			writeValidationError(w, map[string]string{
				"sectionId": "Section does not belong to this exam",
			}, r)
			return
		}
		sectionID = *req.SectionID
	} else {
		if err := a.db.QueryRowContext(r.Context(), `
			SELECT id::text FROM exam_sections
			 WHERE exam_id = $1 AND tenant_id = $2
			 ORDER BY sort_order ASC, created_at ASC
			 LIMIT 1`,
			examID, tenantID,
		).Scan(&sectionID); err != nil {
			writeValidationError(w, map[string]string{
				"sectionId": "Exam has no section to host this group",
			}, r)
			return
		}
	}

	// Resolve display order. Default to MAX + 1.
	displayOrder := 0
	if req.SortOrder != nil {
		displayOrder = *req.SortOrder
	} else {
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT COALESCE(MAX(display_order), -1) + 1 FROM exam_question_groups WHERE exam_id = $1 AND tenant_id = $2`,
			examID, tenantID,
		).Scan(&displayOrder)
	}

	groupType := "standalone"
	stimulusIDArg := ""
	titleSnap := ""
	bodySnap := ""
	if req.StimulusID != nil && *req.StimulusID != "" {
		if errs := a.validateStimulusForExam(r.Context(), tenantID, examID, *req.StimulusID); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
		// Snapshot title + body at create time.
		if err := a.db.QueryRowContext(r.Context(),
			`SELECT title, content FROM stimuli WHERE id = $1 AND tenant_id = $2`,
			*req.StimulusID, tenantID,
		).Scan(&titleSnap, &bodySnap); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not load stimulus snapshot", r)
			return
		}
		stimulusIDArg = *req.StimulusID
		groupType = "stimulus"
	}

	var id string
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO exam_question_groups (
		    tenant_id, exam_id, section_id, stimulus_id,
		    stimulus_title_snapshot, stimulus_body_snapshot,
		    group_type, display_order
		) VALUES ($1, $2, $3, NULLIF($4,'')::uuid,
		          NULLIF($5,''), NULLIF($6,''),
		          $7, $8)
		RETURNING id`,
		tenantID, examID, sectionID, stimulusIDArg,
		titleSnap, bodySnap,
		groupType, displayOrder,
	).Scan(&id)
	if err != nil {
		a.logger.Error("create question group failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create group", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "groups.create", "exam_question_group", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           id,
		"sectionId":    sectionID,
		"groupType":    groupType,
		"displayOrder": displayOrder,
	})
}

// PATCH /api/v1/groups/{groupId}
//
// Update display_order, stimulus reassignment, or re-snapshot from
// library. Name is not yet a column (kept minimal); reserved for a
// later migration if users ask for explicit group titles.
func (a *App) handleUpdateQuestionGroup(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	groupID := r.PathValue("groupId")

	// Resolve parent exam for layered access check.
	var examID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT exam_id::text FROM exam_question_groups WHERE id = $1 AND tenant_id = $2`,
		groupID, tenantID,
	).Scan(&examID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Group not found", r)
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	auth := AuthFromContext(r.Context())

	var req struct {
		SectionID     *string `json:"sectionId"`
		StimulusID    *string `json:"stimulusId"`
		ResyncSnap    bool    `json:"resyncSnapshot"`
		SortOrder     *int    `json:"sortOrder"`
		TitleSnapshot *string `json:"titleSnapshot"`
		BodySnapshot  *string `json:"bodySnapshot"`
		// saveToLibrary=true: after applying any snapshot edits, create
		// a new shared stimulus row from the resulting title+body and
		// link this group to it. Lets users opt their group's passage
		// into the cross-exam library without leaving the group editor.
		// saveToLibrary=false: only update local snapshot, no library row.
		SaveToLibrary bool `json:"saveToLibrary"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	parts := []string{"updated_at = now()"}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}

	// Optional section move. Empty string clears the binding (group
	// becomes section-less); a non-empty value must reference a
	// section owned by the same exam + tenant. This unblocks the
	// deferred "drag a group between sections" item from Phase 9.7.
	if req.SectionID != nil {
		if *req.SectionID == "" {
			parts = append(parts, "section_id = NULL")
		} else {
			var sectionExamID string
			if err := a.db.QueryRowContext(r.Context(),
				`SELECT exam_id::text FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
				*req.SectionID, tenantID,
			).Scan(&sectionExamID); err != nil || sectionExamID != examID {
				writeValidationError(w, map[string]string{
					"sectionId": "Section does not belong to this exam",
				}, r)
				return
			}
			add("section_id", *req.SectionID)
		}
	}

	if req.SortOrder != nil {
		add("display_order", *req.SortOrder)
	}
	if req.StimulusID != nil {
		if *req.StimulusID == "" {
			parts = append(parts, "stimulus_id = NULL")
			parts = append(parts, "stimulus_title_snapshot = NULL")
			parts = append(parts, "stimulus_body_snapshot = NULL")
			parts = append(parts, "group_type = 'standalone'")
		} else {
			if errs := a.validateStimulusForExam(r.Context(), tenantID, examID, *req.StimulusID); len(errs) > 0 {
				writeValidationError(w, errs, r)
				return
			}
			var title, body string
			if err := a.db.QueryRowContext(r.Context(),
				`SELECT title, content FROM stimuli WHERE id = $1 AND tenant_id = $2`,
				*req.StimulusID, tenantID,
			).Scan(&title, &body); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not load stimulus snapshot", r)
				return
			}
			add("stimulus_id", *req.StimulusID)
			add("stimulus_title_snapshot", title)
			add("stimulus_body_snapshot", body)
			parts = append(parts, "group_type = 'stimulus'")
		}
	} else if req.ResyncSnap {
		// Re-snapshot from the existing stimulus_id (only on draft exams;
		// the trigger / handler chain guarantees we only land here when
		// the parent allows it). For simplicity we rely on the parent
		// access check covering authoring rights.
		var stimID, title, body string
		if err := a.db.QueryRowContext(r.Context(), `
			SELECT s.id::text, s.title, s.content
			  FROM exam_question_groups g
			  JOIN stimuli s ON s.id = g.stimulus_id
			 WHERE g.id = $1 AND g.tenant_id = $2`,
			groupID, tenantID,
		).Scan(&stimID, &title, &body); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Group has no stimulus to resync", r)
			return
		}
		add("stimulus_title_snapshot", title)
		add("stimulus_body_snapshot", body)
	}

	// Direct snapshot edit — lets the user edit the group's stimulus
	// content inline without round-tripping through the master stimuli
	// row. Snapshot model already isolates this from other groups that
	// reference the same master row, so it's safe.
	if req.TitleSnapshot != nil {
		add("stimulus_title_snapshot", *req.TitleSnapshot)
	}
	if req.BodySnapshot != nil {
		add("stimulus_body_snapshot", *req.BodySnapshot)
		// If the user is writing inline body without a master stimulus,
		// flip group_type to 'stimulus' so it shows the stimulus header.
		parts = append(parts, "group_type = 'stimulus'")
	}

	if len(args) == 0 && req.StimulusID == nil && req.SectionID == nil && req.TitleSnapshot == nil && req.BodySnapshot == nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": groupID, "status": "no_change"})
		return
	}

	q := "UPDATE exam_question_groups SET " + joinComma(parts) +
		" WHERE id = $" + strconv.Itoa(idx) +
		" AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, groupID, tenantID)
	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update group", r)
		return
	}

	// Optional library promotion. After snapshot is committed, read it
	// back, create a shared stimulus row, and link the group to it. This
	// is the explicit 'masukkan ke library' opt-in path. Idempotent: if
	// already linked to a shared stimulus we update the master row
	// instead of duplicating.
	var libraryStimulusID *string
	if req.SaveToLibrary {
		var curTitle, curBody string
		var curStimulusID sql.NullString
		var curLifecycle sql.NullString
		_ = a.db.QueryRowContext(r.Context(), `
			SELECT COALESCE(g.stimulus_title_snapshot,''), COALESCE(g.stimulus_body_snapshot,''), g.stimulus_id, s.lifecycle
			  FROM exam_question_groups g
			  LEFT JOIN stimuli s ON s.id = g.stimulus_id
			 WHERE g.id = $1 AND g.tenant_id = $2`,
			groupID, tenantID,
		).Scan(&curTitle, &curBody, &curStimulusID, &curLifecycle)
		if strings.TrimSpace(curTitle) == "" || strings.TrimSpace(curBody) == "" {
			writeErrorJSON(w, http.StatusBadRequest, "invalid_request",
				"Stimulus title + body harus diisi sebelum dimasukkan ke library", r)
			return
		}
		if curStimulusID.Valid && curLifecycle.String == "shared" {
			// Already in library — update master row in place.
			_, _ = a.db.ExecContext(r.Context(),
				`UPDATE stimuli SET title=$1, content=$2, updated_at=now() WHERE id=$3 AND tenant_id=$4`,
				curTitle, curBody, curStimulusID.String, tenantID)
			libraryStimulusID = &curStimulusID.String
		} else {
			// Create new shared stimulus + relink group.
			var newStimID string
			if err := a.db.QueryRowContext(r.Context(), `
				INSERT INTO stimuli (tenant_id, owner_user_id, type, title, content, lifecycle)
				VALUES ($1, $2, 'text', $3, $4, 'shared')
				RETURNING id::text`,
				tenantID, auth.UserID, curTitle, curBody,
			).Scan(&newStimID); err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "library_promote_failed",
					"Could not create shared stimulus", r)
				return
			}
			_, _ = a.db.ExecContext(r.Context(),
				`UPDATE exam_question_groups SET stimulus_id = $1, group_type = 'stimulus', updated_at = now()
				 WHERE id = $2 AND tenant_id = $3`,
				newStimID, groupID, tenantID)
			libraryStimulusID = &newStimID
		}
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "groups.update", "exam_question_group", groupID, r)
	resp := map[string]any{"id": groupID, "status": "updated"}
	if libraryStimulusID != nil {
		resp["libraryStimulusId"] = *libraryStimulusID
	}
	writeJSON(w, http.StatusOK, resp)
}

// DELETE /api/v1/groups/{groupId}
//
// Allowed only while the parent exam is in draft. ON DELETE SET NULL
// on exam_questions.group_id releases the linked questions; they
// remain in the exam at root level.
func (a *App) handleDeleteQuestionGroup(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	groupID := r.PathValue("groupId")

	var examID, examStatus string
	if err := a.db.QueryRowContext(r.Context(), `
		SELECT g.exam_id::text, e.status
		  FROM exam_question_groups g
		  JOIN exams e ON e.id = g.exam_id
		 WHERE g.id = $1 AND g.tenant_id = $2`,
		groupID, tenantID,
	).Scan(&examID, &examStatus); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Group not found", r)
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	if examStatus != "draft" {
		writeErrorJSON(w, http.StatusConflict, "invalid_state", "Cannot delete groups on a non-draft exam", r)
		return
	}
	auth := AuthFromContext(r.Context())

	res, err := a.db.ExecContext(r.Context(),
		`DELETE FROM exam_question_groups WHERE id = $1 AND tenant_id = $2`,
		groupID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete group", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Group not found", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "groups.delete", "exam_question_group", groupID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": groupID, "status": "deleted"})
}
