package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Blueprint slots — the structural definition of an assessment unit.
// Per ADR-0010, slots exist in two parallel tables:
//
//   blueprint_template_slots — owned by a blueprint_template (library)
//   exam_blueprint_slots     — cloned snapshot owned by exam_blueprints
//
// Mutation patterns differ:
//   - Template slots are edited in the library before any clone
//   - Exam slots can be edited only while the exam_blueprint is 'draft'
//     (locked once the parent exam is published)
//
// Both share the same field shape (competency, level, difficulty, type,
// points, AKM dimensions, optional stimulus_id), so we share validation
// and SET-clause assembly. The parent table is supplied at the handler
// boundary.

func (a *App) registerBlueprintSlotRoutes(mux *http.ServeMux) {
	// Template slots
	mux.HandleFunc("GET /api/v1/blueprint-templates/{id}/slots", a.handleListTemplateSlots)
	mux.HandleFunc("POST /api/v1/blueprint-templates/{id}/slots", a.handleCreateTemplateSlot)
	mux.HandleFunc("POST /api/v1/blueprint-templates/{id}/slots/bulk", a.handleBulkAddTemplateSlots)
	mux.HandleFunc("PATCH /api/v1/blueprint-template-slots/{slotId}", a.handleUpdateTemplateSlot)
	mux.HandleFunc("DELETE /api/v1/blueprint-template-slots/{slotId}", a.handleDeleteTemplateSlot)

	// Exam blueprint slots (mutate the cloned snapshot inside an exam)
	mux.HandleFunc("GET /api/v1/exams/{id}/blueprint", a.handleGetExamBlueprint)
	mux.HandleFunc("GET /api/v1/exams/{id}/blueprint/slots", a.handleListExamBlueprintSlots)
	mux.HandleFunc("GET /api/v1/exams/{id}/slots-with-questions", a.handleListSlotsWithQuestions)
	mux.HandleFunc("POST /api/v1/exams/{id}/blueprint/slots", a.handleCreateExamBlueprintSlot)
	mux.HandleFunc("PATCH /api/v1/exam-blueprint-slots/{slotId}", a.handleUpdateExamBlueprintSlot)
	mux.HandleFunc("DELETE /api/v1/exam-blueprint-slots/{slotId}", a.handleDeleteExamBlueprintSlot)
	mux.HandleFunc("PATCH /api/v1/exam-blueprint-slots/{slotId}/assign-question", a.handleAssignQuestionToSlot)
}

type slotPayload struct {
	Position              *int     `json:"position,omitempty"`
	CompetencyID          *string  `json:"competencyId,omitempty"`
	CompetencyCode        *string  `json:"competencyCode,omitempty"`
	CompetencyDescription *string  `json:"competencyDescription,omitempty"`
	Materi                *string  `json:"materi,omitempty"`
	Indikator             *string  `json:"indikator,omitempty"`
	CognitiveLevel        *string  `json:"cognitiveLevel,omitempty"`
	Difficulty            *string  `json:"difficulty,omitempty"`
	QuestionType          *string  `json:"questionType,omitempty"`
	Points                *float64 `json:"points,omitempty"`
	StimulusID            *string  `json:"stimulusId,omitempty"`
	CPElementID           *string  `json:"cpElementId,omitempty"`
	CapaianPembelajaran   *string  `json:"capaianPembelajaran,omitempty"`
	ElemenCP              *string  `json:"elemenCp,omitempty"`
	TujuanPembelajaran    *string  `json:"tujuanPembelajaran,omitempty"`
	MateriPokok           *string  `json:"materiPokok,omitempty"`
	Kelas                 *string  `json:"kelas,omitempty"`
	Semester              *string  `json:"semester,omitempty"`
	IndikatorSoal         *string  `json:"indikatorSoal,omitempty"`
}

type slotRow struct {
	ID                    string  `json:"id"`
	Position              int     `json:"position"`
	CompetencyID          *string `json:"competencyId,omitempty"`
	CompetencyCode        *string `json:"competencyCode,omitempty"`
	CompetencyDescription *string `json:"competencyDescription,omitempty"`
	Materi                *string `json:"materi,omitempty"`
	Indikator             *string `json:"indikator,omitempty"`
	CognitiveLevel        *string `json:"cognitiveLevel,omitempty"`
	Difficulty            *string `json:"difficulty,omitempty"`
	QuestionType          *string `json:"questionType,omitempty"`
	Points                float64 `json:"points"`
	StimulusID            *string `json:"stimulusId,omitempty"`
	CPElementID           *string `json:"cpElementId,omitempty"`
	CapaianPembelajaran   *string `json:"capaianPembelajaran,omitempty"`
	ElemenCP              *string `json:"elemenCp,omitempty"`
	TujuanPembelajaran    *string `json:"tujuanPembelajaran,omitempty"`
	MateriPokok           *string `json:"materiPokok,omitempty"`
	Kelas                 *string `json:"kelas,omitempty"`
	Semester              *string `json:"semester,omitempty"`
	IndikatorSoal         *string `json:"indikatorSoal,omitempty"`
	// Only populated for exam_blueprint_slots:
	QuestionID *string `json:"questionId,omitempty"`
	Filled     bool    `json:"filled"`
	CreatedAt  string  `json:"createdAt"`
}

// validateSlot returns a (fields, ok) tuple for slot payloads.
func validateSlot(p slotPayload, isCreate bool) map[string]string {
	_ = isCreate // reserved for create-only checks
	return validateMerdekaKisiKisiPayload(p)
}

// buildSlotInsertSQL constructs an INSERT statement for either
// blueprint_template_slots or exam_blueprint_slots. Returns the SQL,
// args, and any validation errors.
func buildSlotInsertSQL(table, parentCol, parentID string, position int, p slotPayload) (string, []any) {
	cols := []string{parentCol, "position", "points"}
	vals := []string{"$1", "$2", "$3"}
	args := []any{parentID, position, defaultFloat(p.Points, 1)}
	idx := 4

	addOpt := func(col string, val any) {
		cols = append(cols, col)
		vals = append(vals, "$"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}

	if p.CompetencyID != nil && *p.CompetencyID != "" {
		addOpt("competency_id", *p.CompetencyID)
	}
	if p.CompetencyCode != nil {
		addOpt("competency_code", *p.CompetencyCode)
	}
	if p.CompetencyDescription != nil {
		addOpt("competency_description", *p.CompetencyDescription)
	}
	if p.Materi != nil {
		addOpt("materi", *p.Materi)
	}
	if p.Indikator != nil {
		addOpt("indikator", *p.Indikator)
	}
	if p.CognitiveLevel != nil && *p.CognitiveLevel != "" {
		addOpt("cognitive_level", *p.CognitiveLevel)
	}
	if p.Difficulty != nil && *p.Difficulty != "" {
		addOpt("difficulty", *p.Difficulty)
	}
	if p.QuestionType != nil && *p.QuestionType != "" {
		addOpt("question_type", *p.QuestionType)
	}
	if p.StimulusID != nil && *p.StimulusID != "" {
		addOpt("stimulus_id", *p.StimulusID)
	}
	if p.CPElementID != nil && *p.CPElementID != "" {
		addOpt("cp_element_id", *p.CPElementID)
	}
	if p.CapaianPembelajaran != nil {
		addOpt("capaian_pembelajaran", *p.CapaianPembelajaran)
	}
	if p.ElemenCP != nil {
		addOpt("elemen_cp", *p.ElemenCP)
	}
	if p.TujuanPembelajaran != nil {
		addOpt("tujuan_pembelajaran", *p.TujuanPembelajaran)
	}
	if p.MateriPokok != nil {
		addOpt("materi_pokok", *p.MateriPokok)
	}
	if p.Kelas != nil {
		addOpt("kelas", *p.Kelas)
	}
	if p.Semester != nil {
		addOpt("semester", *p.Semester)
	}
	if p.IndikatorSoal != nil {
		addOpt("indikator_soal", *p.IndikatorSoal)
	}
	q := "INSERT INTO " + table + " (" + strings.Join(cols, ", ") +
		") VALUES (" + strings.Join(vals, ", ") + ") RETURNING id::text"
	return q, args
}

func defaultFloat(p *float64, d float64) float64 {
	if p == nil {
		return d
	}
	return *p
}

// updateSlotFields builds a partial UPDATE SQL for the same field shape.
// Returns "" if nothing to update.
func buildSlotUpdateSQL(table, slotID string, p slotPayload) (string, []any) {
	parts := []string{}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}
	addOptStr := func(col string, val *string) {
		if val == nil {
			return
		}
		if *val == "" {
			parts = append(parts, col+" = NULL")
		} else {
			add(col, *val)
		}
	}
	if p.Position != nil {
		add("position", *p.Position)
	}
	addOptStr("competency_id", p.CompetencyID)
	addOptStr("competency_code", p.CompetencyCode)
	addOptStr("competency_description", p.CompetencyDescription)
	addOptStr("materi", p.Materi)
	addOptStr("indikator", p.Indikator)
	addOptStr("cognitive_level", p.CognitiveLevel)
	addOptStr("difficulty", p.Difficulty)
	addOptStr("question_type", p.QuestionType)
	addOptStr("stimulus_id", p.StimulusID)
	addOptStr("cp_element_id", p.CPElementID)
	addOptStr("capaian_pembelajaran", p.CapaianPembelajaran)
	addOptStr("elemen_cp", p.ElemenCP)
	addOptStr("tujuan_pembelajaran", p.TujuanPembelajaran)
	addOptStr("materi_pokok", p.MateriPokok)
	addOptStr("kelas", p.Kelas)
	addOptStr("semester", p.Semester)
	addOptStr("indikator_soal", p.IndikatorSoal)
	if p.Points != nil {
		add("points", *p.Points)
	}
	if len(parts) == 0 && len(args) == 0 {
		return "", nil
	}
	q := "UPDATE " + table + " SET " + strings.Join(parts, ", ") +
		" WHERE id = $" + strconv.Itoa(idx)
	args = append(args, slotID)
	return q, args
}

// nextSlotPosition returns max(position)+1 for the parent.
func (a *App) nextSlotPosition(r *http.Request, table, parentCol, parentID string) int {
	var pos int
	_ = a.db.QueryRowContext(r.Context(),
		"SELECT COALESCE(MAX(position), -1) + 1 FROM "+table+" WHERE "+parentCol+" = $1",
		parentID,
	).Scan(&pos)
	return pos
}

// recomputeBlueprintTotals refreshes total_slots and total_points
// after slot insert/update/delete. Called by both template and exam
// blueprint slot handlers.
func (a *App) recomputeBlueprintTotals(r *http.Request, parentTable, slotsTable, parentCol, parentID string) {
	_, _ = a.db.ExecContext(r.Context(), `
		UPDATE `+parentTable+` SET
		    total_slots = (SELECT COUNT(*) FROM `+slotsTable+` WHERE `+parentCol+` = $1),
		    total_points = (SELECT COALESCE(SUM(points), 0) FROM `+slotsTable+` WHERE `+parentCol+` = $1),
		    updated_at = now()
		 WHERE id = $1`, parentID)
}

// =========================================================================
// Template slot handlers
// =========================================================================

func (a *App) handleListTemplateSlots(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:read") {
		return
	}
	templateID := r.PathValue("id")
	if !a.requireBlueprintAccess(w, r, templateID, ActionRead) {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id::text, position, competency_id::text, competency_code,
		       competency_description, materi, indikator, cognitive_level,
		       difficulty, question_type, points, stimulus_id::text,
		       cp_element_id::text, capaian_pembelajaran, elemen_cp,
		       tujuan_pembelajaran, materi_pokok, kelas, semester, indikator_soal,
		       created_at::text
		  FROM blueprint_template_slots
		 WHERE template_id = $1
		 ORDER BY position`, templateID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load slots", r)
		return
	}
	defer rows.Close()

	out := make([]slotRow, 0)
	for rows.Next() {
		var s slotRow
		var compID, stimID sql.NullString
		if err := rows.Scan(&s.ID, &s.Position,
			&compID, &s.CompetencyCode,
			&s.CompetencyDescription, &s.Materi, &s.Indikator,
			&s.CognitiveLevel, &s.Difficulty, &s.QuestionType,
			&s.Points, &stimID,
			&s.CPElementID, &s.CapaianPembelajaran, &s.ElemenCP,
			&s.TujuanPembelajaran, &s.MateriPokok, &s.Kelas, &s.Semester, &s.IndikatorSoal,
			&s.CreatedAt); err == nil {
			if compID.Valid {
				v := compID.String
				s.CompetencyID = &v
			}
			if stimID.Valid {
				v := stimID.String
				s.StimulusID = &v
			}
			out = append(out, s)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *App) handleCreateTemplateSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	templateID := r.PathValue("id")
	if !a.requireBlueprintAccess(w, r, templateID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	var p slotPayload
	if err := readJSON(r, &p); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, p); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}

	pos := defaultInt(p.Position, a.nextSlotPosition(r, "blueprint_template_slots", "template_id", templateID))
	q, args := buildSlotInsertSQL("blueprint_template_slots", "template_id", templateID, pos, p)
	var id string
	if err := a.db.QueryRowContext(r.Context(), q, args...).Scan(&id); err != nil {
		a.logger.Error("create template slot failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
			"Could not create slot", r)
		return
	}

	a.recomputeBlueprintTotals(r, "blueprint_templates", "blueprint_template_slots",
		"template_id", templateID)
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates.slot_added", "blueprint_template_slot", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "position": pos})
}

func defaultInt(p *int, d int) int {
	if p == nil {
		return d
	}
	return *p
}

// handleBulkAddTemplateSlots accepts an array of slot payloads and
// inserts them in a single transaction. Position is auto-assigned per
// slot if not provided. Returns the IDs in input order.
func (a *App) handleBulkAddTemplateSlots(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	templateID := r.PathValue("id")
	if !a.requireBlueprintAccess(w, r, templateID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	var req struct {
		Slots []slotPayload `json:"slots"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	if len(req.Slots) == 0 {
		writeValidationError(w, map[string]string{"slots": "At least one slot is required"}, r)
		return
	}
	if len(req.Slots) > 200 {
		writeValidationError(w, map[string]string{"slots": "Maximum 200 slots per bulk request"}, r)
		return
	}

	// Validate every payload first so we don't leave a half-inserted set.
	for i, p := range req.Slots {
		if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, p); len(errs) > 0 {
			fields := map[string]string{"slots": "Invalid slot at index " + strconv.Itoa(i) + ": see field errors"}
			for k, v := range errs {
				fields["slots."+strconv.Itoa(i)+"."+k] = v
			}
			writeValidationError(w, fields, r)
			return
		}
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
			"Could not start transaction", r)
		return
	}
	defer tx.Rollback()

	// Lock the next position. We compute once and increment locally to
	// avoid a query per slot.
	var nextPos int
	if err := tx.QueryRowContext(r.Context(),
		"SELECT COALESCE(MAX(position), -1) + 1 FROM blueprint_template_slots WHERE template_id = $1",
		templateID,
	).Scan(&nextPos); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
			"Could not determine slot positions", r)
		return
	}

	ids := make([]string, 0, len(req.Slots))
	for _, p := range req.Slots {
		pos := defaultInt(p.Position, nextPos)
		nextPos = pos + 1
		q, args := buildSlotInsertSQL("blueprint_template_slots", "template_id", templateID, pos, p)
		var id string
		if err := tx.QueryRowContext(r.Context(), q, args...).Scan(&id); err != nil {
			a.logger.Error("bulk slot insert failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
				"Could not insert slot at position "+strconv.Itoa(pos), r)
			return
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
			"Could not finalize bulk insert", r)
		return
	}

	a.recomputeBlueprintTotals(r, "blueprint_templates", "blueprint_template_slots",
		"template_id", templateID)
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates.slots_bulk_added", "blueprint_template", templateID, r)
	writeJSON(w, http.StatusCreated, map[string]any{"ids": ids, "count": len(ids)})
}

func (a *App) handleUpdateTemplateSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	slotID := r.PathValue("slotId")

	// Resolve parent template + tenant for access check
	var templateID, tenantID string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT t.id::text, t.tenant_id::text
		  FROM blueprint_template_slots s
		  JOIN blueprint_templates t ON t.id = s.template_id
		 WHERE s.id = $1`, slotID,
	).Scan(&templateID, &tenantID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Slot not found", r)
		return
	}
	if !a.requireBlueprintAccess(w, r, templateID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	var p slotPayload
	if err := readJSON(r, &p); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	current, err := a.loadSlotPayload(r.Context(), "blueprint_template_slots", slotID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Slot not found", r)
		return
	}
	if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, mergeSlotPayload(current, p)); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}

	q, args := buildSlotUpdateSQL("blueprint_template_slots", slotID, p)
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"id": slotID, "status": "no_change"})
		return
	}
	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed",
			"Could not update slot", r)
		return
	}
	a.recomputeBlueprintTotals(r, "blueprint_templates", "blueprint_template_slots",
		"template_id", templateID)
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates.slot_updated", "blueprint_template_slot", slotID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": slotID, "status": "updated"})
}

func (a *App) handleDeleteTemplateSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	slotID := r.PathValue("slotId")

	var templateID, tenantID string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT t.id::text, t.tenant_id::text
		  FROM blueprint_template_slots s
		  JOIN blueprint_templates t ON t.id = s.template_id
		 WHERE s.id = $1`, slotID,
	).Scan(&templateID, &tenantID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Slot not found", r)
		return
	}
	if !a.requireBlueprintAccess(w, r, templateID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	if _, err := a.db.ExecContext(r.Context(),
		`DELETE FROM blueprint_template_slots WHERE id = $1`, slotID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed",
			"Could not delete slot", r)
		return
	}
	a.recomputeBlueprintTotals(r, "blueprint_templates", "blueprint_template_slots",
		"template_id", templateID)
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates.slot_deleted", "blueprint_template_slot", slotID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": slotID, "status": "deleted"})
}

// handleListSlotsWithQuestions powers the slot-first canvas (ADR-0012).
// Returns every slot for an exam plus the linked question's content +
// type + points + stimulus/group summaries, ordered by slot position.
// Questions that have no blueprint binding are returned in the
// `unlinked` array so the canvas can render an "Unlinked Questions"
// section at the bottom.
//
// Response also carries the blueprint header (curriculum, blueprint
// type) so the frontend can render AKM-aware labels without a separate
// blueprint fetch.
//
// Response shape:
//
//	{
//	  "blueprintType": "reguler" | null,
//	  "slots": [{ slot fields..., question: {...} | null }],
//	  "unlinked": [{ id, content, questionType, points, sortOrder,
//	                 stimulus?, group? }],
//	  "coverage": { filled, total }
//	}
func (a *App) handleListSlotsWithQuestions(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionRead) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	type stimulusRef struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	type groupRef struct {
		ID            string  `json:"id"`
		StimulusTitle *string `json:"stimulusTitle,omitempty"`
	}
	type slotQuestion struct {
		ID           string       `json:"id"`
		Content      string       `json:"content"`
		QuestionType string       `json:"questionType"`
		Points       float64      `json:"points"`
		SortOrder    int          `json:"sortOrder"`
		Stimulus     *stimulusRef `json:"stimulus,omitempty"`
		Group        *groupRef    `json:"group,omitempty"`
	}
	type slotWithQuestion struct {
		slotRow
		Question *slotQuestion `json:"question"`
	}

	// Pull source template id up-front so response can mark template-cloned slots.
	var bpTemplateID sql.NullString
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT source_template_id::text FROM exam_blueprints WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&bpTemplateID)

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT s.id::text, s.position, s.competency_id::text, s.competency_code,
		       s.competency_description, s.materi, s.indikator, s.cognitive_level,
		       s.difficulty, s.question_type, s.points, s.stimulus_id::text,
		       s.cp_element_id::text, s.capaian_pembelajaran, s.elemen_cp,
		       s.tujuan_pembelajaran, s.materi_pokok, s.kelas, s.semester, s.indikator_soal,
		       s.created_at::text,
		       q.id::text, q.content, q.question_type, q.points, q.sort_order,
		       q.stimulus_id::text, q.group_id::text,
		       st.title, g.stimulus_title_snapshot
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		  LEFT JOIN exam_questions q ON q.blueprint_slot_id = s.id
		  LEFT JOIN stimuli st ON st.id = q.stimulus_id
		  LEFT JOIN exam_question_groups g ON g.id = q.group_id
		 WHERE b.exam_id = $1 AND b.tenant_id = $2
		 ORDER BY s.position`,
		examID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load slots with questions", r)
		return
	}
	defer rows.Close()

	slots := make([]slotWithQuestion, 0)
	filled := 0
	for rows.Next() {
		var s slotWithQuestion
		var compID, stimID sql.NullString
		var qID, qContent, qType sql.NullString
		var qPoints sql.NullFloat64
		var qSortOrder sql.NullInt64
		var qStimulusID, qGroupID, qStimulusTitle, qGroupSnapTitle sql.NullString
		if err := rows.Scan(&s.ID, &s.Position,
			&compID, &s.CompetencyCode,
			&s.CompetencyDescription, &s.Materi, &s.Indikator,
			&s.CognitiveLevel, &s.Difficulty, &s.QuestionType,
			&s.Points, &stimID,
			&s.CPElementID, &s.CapaianPembelajaran, &s.ElemenCP,
			&s.TujuanPembelajaran, &s.MateriPokok, &s.Kelas, &s.Semester, &s.IndikatorSoal,
			&s.CreatedAt,
			&qID, &qContent, &qType, &qPoints, &qSortOrder,
			&qStimulusID, &qGroupID,
			&qStimulusTitle, &qGroupSnapTitle); err != nil {
			continue
		}
		if compID.Valid {
			v := compID.String
			s.CompetencyID = &v
		}
		if stimID.Valid {
			v := stimID.String
			s.StimulusID = &v
		}
		if qID.Valid {
			s.QuestionID = &qID.String
			s.Filled = true
			filled++
			s.Question = &slotQuestion{
				ID:      qID.String,
				Content: qContent.String,
			}
			if qType.Valid {
				s.Question.QuestionType = qType.String
			}
			if qPoints.Valid {
				s.Question.Points = qPoints.Float64
			}
			if qSortOrder.Valid {
				s.Question.SortOrder = int(qSortOrder.Int64)
			}
			if qStimulusID.Valid {
				ref := stimulusRef{ID: qStimulusID.String}
				if qStimulusTitle.Valid {
					ref.Title = qStimulusTitle.String
				}
				s.Question.Stimulus = &ref
			}
			if qGroupID.Valid {
				ref := groupRef{ID: qGroupID.String}
				if qGroupSnapTitle.Valid {
					t := qGroupSnapTitle.String
					ref.StimulusTitle = &t
				}
				s.Question.Group = &ref
			}
		}
		slots = append(slots, s)
	}

	// Unlinked questions: any question in this exam without a
	// blueprint_slot_id. Carries stimulus + group summaries so the
	// canvas's "unlinked" pane can render the same affordances.
	unlinkedRows, uerr := a.db.QueryContext(r.Context(), `
		SELECT q.id::text, q.content, q.question_type, q.points, q.sort_order,
		       q.stimulus_id::text, q.group_id::text,
		       st.title, g.stimulus_title_snapshot
		  FROM exam_questions q
		  LEFT JOIN stimuli st ON st.id = q.stimulus_id
		  LEFT JOIN exam_question_groups g ON g.id = q.group_id
		 WHERE q.exam_id = $1 AND q.tenant_id = $2 AND q.blueprint_slot_id IS NULL
		 ORDER BY q.sort_order, q.created_at`,
		examID, tenantID,
	)
	type unlinkedRow struct {
		ID           string       `json:"id"`
		Content      string       `json:"content"`
		QuestionType string       `json:"questionType"`
		Points       float64      `json:"points"`
		SortOrder    int          `json:"sortOrder"`
		Stimulus     *stimulusRef `json:"stimulus,omitempty"`
		Group        *groupRef    `json:"group,omitempty"`
	}
	unlinked := make([]unlinkedRow, 0)
	if uerr == nil {
		defer unlinkedRows.Close()
		for unlinkedRows.Next() {
			var u unlinkedRow
			var stimID, groupID, stimTitle, groupTitle sql.NullString
			if err := unlinkedRows.Scan(&u.ID, &u.Content, &u.QuestionType, &u.Points, &u.SortOrder,
				&stimID, &groupID, &stimTitle, &groupTitle); err == nil {
				if stimID.Valid {
					ref := stimulusRef{ID: stimID.String}
					if stimTitle.Valid {
						ref.Title = stimTitle.String
					}
					u.Stimulus = &ref
				}
				if groupID.Valid {
					ref := groupRef{ID: groupID.String}
					if groupTitle.Valid {
						t := groupTitle.String
						ref.StimulusTitle = &t
					}
					u.Group = &ref
				}
				unlinked = append(unlinked, u)
			}
		}
	}

	resp := map[string]any{
		"slots":    slots,
		"unlinked": unlinked,
		"coverage": map[string]int{
			"filled": filled,
			"total":  len(slots),
		},
	}
	if bpTemplateID.Valid && bpTemplateID.String != "" {
		resp["sourceTemplateId"] = bpTemplateID.String
	}
	writeJSON(w, http.StatusOK, resp)
}

// =========================================================================
// Exam blueprint slot handlers
// =========================================================================

func (a *App) handleGetExamBlueprint(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:read") {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionRead) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	type examBlueprintRow struct {
		ID                    string  `json:"id"`
		ExamID                string  `json:"examId"`
		SourceTemplateID      *string `json:"sourceTemplateId,omitempty"`
		SourceTemplateVersion *int    `json:"sourceTemplateVersion,omitempty"`
		CreatedVia            string  `json:"createdVia"`
		Title                 string  `json:"title"`
		Description           *string `json:"description,omitempty"`
		CurriculumCode        string  `json:"curriculumCode"`
		CompetencyLabel       string  `json:"competencyLabel"`
		BlueprintType         string  `json:"blueprintType"`
		TotalSlots            int     `json:"totalSlots"`
		TotalPoints           float64 `json:"totalPoints"`
		StrictCoverage        bool    `json:"strictCoverage"`
		Status                string  `json:"status"`
		FilledSlots           int     `json:"filledSlots"`
		Coverage              float64 `json:"coverage"`
		CreatedAt             string  `json:"createdAt"`
	}

	var bp examBlueprintRow
	var srcID, srcVer sql.NullString
	err := a.db.QueryRowContext(r.Context(), `
		SELECT b.id::text, b.exam_id::text, b.source_template_id::text,
		       b.source_template_version::text, b.created_via,
		       b.title, b.description, c.code, c.competency_label,
		       b.blueprint_type, b.total_slots, b.total_points, b.strict_coverage,
		       b.status, b.created_at::text
		  FROM exam_blueprints b
		  JOIN curricula c ON c.id = b.curriculum_id
		 WHERE b.exam_id = $1 AND b.tenant_id = $2`,
		examID, tenantID,
	).Scan(&bp.ID, &bp.ExamID, &srcID, &srcVer, &bp.CreatedVia,
		&bp.Title, &bp.Description, &bp.CurriculumCode, &bp.CompetencyLabel,
		&bp.BlueprintType, &bp.TotalSlots, &bp.TotalPoints, &bp.StrictCoverage,
		&bp.Status, &bp.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"blueprint": nil})
		return
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load exam blueprint", r)
		return
	}
	if srcID.Valid {
		v := srcID.String
		bp.SourceTemplateID = &v
	}
	if srcVer.Valid {
		// version stored as INTEGER but ::text gives string; parse
		if v, err := strconv.Atoi(srcVer.String); err == nil {
			bp.SourceTemplateVersion = &v
		}
	}

	// Coverage computation
	_ = a.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM exam_blueprint_slots s
		  JOIN exam_questions q ON q.blueprint_slot_id = s.id
		 WHERE s.exam_blueprint_id = $1`,
		bp.ID,
	).Scan(&bp.FilledSlots)
	if bp.TotalSlots > 0 {
		bp.Coverage = float64(bp.FilledSlots) / float64(bp.TotalSlots)
	}

	writeJSON(w, http.StatusOK, map[string]any{"blueprint": bp})
}

func (a *App) handleListExamBlueprintSlots(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:read") {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionRead) {
		return
	}

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT s.id::text, s.position, s.competency_id::text, s.competency_code,
		       s.competency_description, s.materi, s.indikator, s.cognitive_level,
		       s.difficulty, s.question_type, s.points, s.stimulus_id::text,
		       s.cp_element_id::text, s.capaian_pembelajaran, s.elemen_cp,
		       s.tujuan_pembelajaran, s.materi_pokok, s.kelas, s.semester, s.indikator_soal,
		       s.created_at::text,
		       (SELECT q.id::text FROM exam_questions q WHERE q.blueprint_slot_id = s.id) AS question_id
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE b.exam_id = $1
		 ORDER BY s.position`, examID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load slots", r)
		return
	}
	defer rows.Close()

	out := make([]slotRow, 0)
	for rows.Next() {
		var s slotRow
		var compID, stimID, qID sql.NullString
		if err := rows.Scan(&s.ID, &s.Position,
			&compID, &s.CompetencyCode,
			&s.CompetencyDescription, &s.Materi, &s.Indikator,
			&s.CognitiveLevel, &s.Difficulty, &s.QuestionType,
			&s.Points, &stimID,
			&s.CPElementID, &s.CapaianPembelajaran, &s.ElemenCP,
			&s.TujuanPembelajaran, &s.MateriPokok, &s.Kelas, &s.Semester, &s.IndikatorSoal,
			&s.CreatedAt, &qID); err == nil {
			if compID.Valid {
				v := compID.String
				s.CompetencyID = &v
			}
			if stimID.Valid {
				v := stimID.String
				s.StimulusID = &v
			}
			if qID.Valid {
				v := qID.String
				s.QuestionID = &v
				s.Filled = true
			}
			out = append(out, s)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *App) handleCreateExamBlueprintSlot(w http.ResponseWriter, r *http.Request) {
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

	// Resolve exam_blueprint_id
	var blueprintID, status string
	err := a.db.QueryRowContext(r.Context(),
		`SELECT id::text, status FROM exam_blueprints WHERE exam_id = $1`, examID,
	).Scan(&blueprintID, &status)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found",
			"Exam has no blueprint. Create or clone one first.", r)
		return
	}
	if status == "locked" {
		writeErrorJSON(w, http.StatusConflict, "invalid_state",
			"Blueprint is locked (parent exam is published)", r)
		return
	}

	var p slotPayload
	if err := readJSON(r, &p); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, p); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}
	pos := defaultInt(p.Position, a.nextSlotPosition(r, "exam_blueprint_slots", "exam_blueprint_id", blueprintID))
	q, args := buildSlotInsertSQL("exam_blueprint_slots", "exam_blueprint_id", blueprintID, pos, p)
	var id string
	if err := a.db.QueryRowContext(r.Context(), q, args...).Scan(&id); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
			"Could not create slot", r)
		return
	}
	a.recomputeBlueprintTotals(r, "exam_blueprints", "exam_blueprint_slots",
		"exam_blueprint_id", blueprintID)
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"exam_blueprints.slot_added", "exam_blueprint_slot", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "position": pos})
}

func (a *App) handleUpdateExamBlueprintSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	slotID := r.PathValue("slotId")
	examID, blueprintID, blueprintStatus, tenantID, ok := a.resolveExamSlotParent(w, r, slotID)
	if !ok {
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	if blueprintStatus == "locked" {
		writeErrorJSON(w, http.StatusConflict, "invalid_state",
			"Blueprint is locked (parent exam is published)", r)
		return
	}

	var p slotPayload
	if err := readJSON(r, &p); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	current, err := a.loadSlotPayload(r.Context(), "exam_blueprint_slots", slotID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Slot not found", r)
		return
	}
	if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, mergeSlotPayload(current, p)); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}
	q, args := buildSlotUpdateSQL("exam_blueprint_slots", slotID, p)
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"id": slotID, "status": "no_change"})
		return
	}
	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed",
			"Could not update slot", r)
		return
	}
	a.recomputeBlueprintTotals(r, "exam_blueprints", "exam_blueprint_slots",
		"exam_blueprint_id", blueprintID)
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"exam_blueprints.slot_updated", "exam_blueprint_slot", slotID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": slotID, "status": "updated"})
}

func (a *App) handleDeleteExamBlueprintSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	slotID := r.PathValue("slotId")
	examID, blueprintID, blueprintStatus, tenantID, ok := a.resolveExamSlotParent(w, r, slotID)
	if !ok {
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	if blueprintStatus == "locked" {
		writeErrorJSON(w, http.StatusConflict, "invalid_state",
			"Blueprint is locked", r)
		return
	}
	if _, err := a.db.ExecContext(r.Context(),
		`DELETE FROM exam_blueprint_slots WHERE id = $1`, slotID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed",
			"Could not delete slot", r)
		return
	}
	a.recomputeBlueprintTotals(r, "exam_blueprints", "exam_blueprint_slots",
		"exam_blueprint_id", blueprintID)
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"exam_blueprints.slot_deleted", "exam_blueprint_slot", slotID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": slotID, "status": "deleted"})
}

// handleAssignQuestionToSlot atomically links a question to a slot,
// unlinking any prior question from the same slot. Per ADR-0010 atomic
// slot-question swap.
//
// Body: { "questionId": "uuid" | null }
//   - questionId provided → unlink any existing question on this slot,
//     then link the new question. Both must belong to the same exam.
//   - questionId null → just clear the link.
func (a *App) handleAssignQuestionToSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	slotID := r.PathValue("slotId")
	examID, _, blueprintStatus, tenantID, ok := a.resolveExamSlotParent(w, r, slotID)
	if !ok {
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	if blueprintStatus == "locked" {
		writeErrorJSON(w, http.StatusConflict, "invalid_state",
			"Blueprint is locked", r)
		return
	}

	var req struct {
		QuestionID *string `json:"questionId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "assign_failed",
			"Could not start transaction", r)
		return
	}
	defer tx.Rollback()

	// Always clear any existing link to this slot first.
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE exam_questions SET blueprint_slot_id = NULL WHERE blueprint_slot_id = $1`,
		slotID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "assign_failed",
			"Could not clear existing link", r)
		return
	}

	if req.QuestionID != nil && *req.QuestionID != "" {
		// Verify the question belongs to the same exam
		var qExamID string
		if err := tx.QueryRowContext(r.Context(),
			`SELECT exam_id::text FROM exam_questions WHERE id = $1`, *req.QuestionID,
		).Scan(&qExamID); err != nil {
			writeErrorJSON(w, http.StatusNotFound, "not_found",
				"Question not found", r)
			return
		}
		if qExamID != examID {
			writeErrorJSON(w, http.StatusBadRequest, "invalid_request",
				"Question belongs to a different exam", r)
			return
		}
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE exam_questions SET blueprint_slot_id = $1 WHERE id = $2`,
			slotID, *req.QuestionID,
		); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "assign_failed",
				"Could not link question to slot", r)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "assign_failed",
			"Could not finalize assignment", r)
		return
	}
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"exam_blueprints.slot_assigned", "exam_blueprint_slot", slotID, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"slotId":     slotID,
		"questionId": req.QuestionID,
		"status":     "assigned",
	})
}

// resolveExamSlotParent returns (examID, blueprintID, blueprintStatus,
// tenantID, ok) for an exam_blueprint_slot lookup. Writes the response
// and returns ok=false on error.
func (a *App) resolveExamSlotParent(
	w http.ResponseWriter, r *http.Request, slotID string,
) (examID, blueprintID, blueprintStatus, tenantID string, ok bool) {
	err := a.db.QueryRowContext(r.Context(), `
		SELECT b.exam_id::text, b.id::text, b.status, b.tenant_id::text
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE s.id = $1`, slotID,
	).Scan(&examID, &blueprintID, &blueprintStatus, &tenantID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Slot not found", r)
		return "", "", "", "", false
	}
	return examID, blueprintID, blueprintStatus, tenantID, true
}
