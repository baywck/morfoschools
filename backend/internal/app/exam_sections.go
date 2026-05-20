package app

import (
	"net/http"
	"strconv"
)

// Exam Sections — group of questions within an exam. Mirrors program_sections
// in spirit but is owned by an exam rather than a program.
//
// RBAC: same as exams.go — exams:read for list, exams:write for mutations.
// Subject-based teacher restriction is enforced via the parent exam.

func (a *App) registerExamSectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/exams/{id}/sections", a.handleListExamSections)
	mux.HandleFunc("POST /api/v1/exams/{id}/sections", a.handleCreateExamSection)
	mux.HandleFunc("PATCH /api/v1/exam-sections/{sectionId}", a.handleUpdateExamSection)
	mux.HandleFunc("DELETE /api/v1/exam-sections/{sectionId}", a.handleDeleteExamSection)
}

type examSectionRow struct {
	ID            string  `json:"id"`
	ExamID        string  `json:"examId"`
	Title         string  `json:"title"`
	Description   *string `json:"description"`
	SortOrder     int     `json:"sortOrder"`
	QuestionCount int     `json:"questionCount"`
	CreatedAt     string  `json:"createdAt"`
}

func (a *App) handleListExamSections(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	examID := r.PathValue("id")

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT s.id, s.exam_id, s.title, s.description, s.sort_order,
		       COALESCE((SELECT COUNT(*) FROM exam_questions q WHERE q.section_id = s.id), 0) AS question_count,
		       s.created_at
		  FROM exam_sections s
		 WHERE s.exam_id = $1 AND s.tenant_id = $2
		 ORDER BY s.sort_order ASC, s.created_at ASC`,
		examID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "sections_lookup_failed", "Could not load sections", r)
		return
	}
	defer rows.Close()

	out := make([]examSectionRow, 0)
	for rows.Next() {
		var s examSectionRow
		if err := rows.Scan(&s.ID, &s.ExamID, &s.Title, &s.Description, &s.SortOrder, &s.QuestionCount, &s.CreatedAt); err == nil {
			out = append(out, s)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *App) handleCreateExamSection(w http.ResponseWriter, r *http.Request) {
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

	// Layered access check (ADR-0009). Replaces prior subject-only gate.
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	auth := AuthFromContext(r.Context())

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		SortOrder   *int   `json:"sortOrder"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	if req.Title == "" {
		writeValidationError(w, map[string]string{"title": "Title is required"}, r)
		return
	}

	// Default sort_order = max + 1 so new sections land at the end.
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	} else {
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM exam_sections WHERE exam_id = $1`, examID,
		).Scan(&sortOrder)
	}

	var id string
	if err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO exam_sections (tenant_id, exam_id, title, description, sort_order)
		VALUES ($1, $2, $3, NULLIF($4,''), $5) RETURNING id`,
		tenantID, examID, req.Title, req.Description, sortOrder,
	).Scan(&id); err != nil {
		a.logger.Error("create exam section failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create section", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exam_sections.create", "exam_section", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) handleUpdateExamSection(w http.ResponseWriter, r *http.Request) {
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
	sectionID := r.PathValue("sectionId")

	auth := AuthFromContext(r.Context())
	// Resolve parent exam for layered access check.
	var examID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT exam_id::text FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
		sectionID, tenantID,
	).Scan(&examID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Section not found", r)
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		SortOrder   *int    `json:"sortOrder"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	parts := []string{}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}
	if req.Title != nil {
		add("title", *req.Title)
	}
	if req.Description != nil {
		if *req.Description == "" {
			parts = append(parts, "description = NULL")
		} else {
			add("description", *req.Description)
		}
	}
	if req.SortOrder != nil {
		add("sort_order", *req.SortOrder)
	}
	if len(parts) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"id": sectionID, "status": "no_change"})
		return
	}

	q := "UPDATE exam_sections SET " + joinComma(parts) +
		" WHERE id = $" + strconv.Itoa(idx) +
		" AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, sectionID, tenantID)

	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update section", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exam_sections.update", "exam_section", sectionID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": sectionID, "status": "updated"})
}

func (a *App) handleDeleteExamSection(w http.ResponseWriter, r *http.Request) {
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
	sectionID := r.PathValue("sectionId")

	auth := AuthFromContext(r.Context())
	// Resolve parent exam for layered access check.
	var examID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT exam_id::text FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
		sectionID, tenantID,
	).Scan(&examID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Section not found", r)
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}

	// Phase 9.8 rule: every exam must have at least one section. Block
	// the delete and audit the attempt so we have a paper trail when a
	// teacher tries to remove the last container.
	var sectionCount int
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM exam_sections WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&sectionCount)
	if sectionCount <= 1 {
		a.audit(r.Context(), &tenantID, auth.UserID, "exam_sections.delete_blocked", "exam_section", sectionID, r)
		writeValidationError(w, map[string]string{
			"section": "Cannot delete the last section. An exam must have at least one section.",
		}, r)
		return
	}

	// Reassign members to the next section before deleting so the
	// NOT NULL invariant on exam_questions.section_id holds. We also
	// rebind orphan groups; both happen inside a single transaction.
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete section", r)
		return
	}
	defer tx.Rollback()

	var fallbackSectionID string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT id FROM exam_sections
		 WHERE exam_id = $1 AND tenant_id = $2 AND id <> $3
		 ORDER BY sort_order ASC, created_at ASC
		 LIMIT 1`,
		examID, tenantID, sectionID,
	).Scan(&fallbackSectionID); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not resolve fallback section", r)
		return
	}

	if _, err := tx.ExecContext(r.Context(),
		`UPDATE exam_questions SET section_id = $1, updated_at = now()
		  WHERE section_id = $2 AND tenant_id = $3`,
		fallbackSectionID, sectionID, tenantID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not reassign questions", r)
		return
	}
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE exam_question_groups SET section_id = $1, updated_at = now()
		  WHERE section_id = $2 AND tenant_id = $3`,
		fallbackSectionID, sectionID, tenantID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not reassign groups", r)
		return
	}

	res, err := tx.ExecContext(r.Context(),
		`DELETE FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
		sectionID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete section", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Section not found", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not finalize section delete", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exam_sections.delete", "exam_section", sectionID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": sectionID, "status": "deleted"})
}
