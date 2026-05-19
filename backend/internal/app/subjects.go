package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerSubjectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/subjects", a.handleListSubjects)
	mux.HandleFunc("POST /api/v1/subjects", a.handleCreateSubject)
	mux.HandleFunc("PATCH /api/v1/subjects/{id}", a.handleUpdateSubject)
	mux.HandleFunc("PATCH /api/v1/subjects/{id}/archive", a.handleArchiveSubject)
	mux.HandleFunc("GET /api/v1/subjects/{id}/teachers", a.handleListSubjectTeachers)
}

func (a *App) handleListSubjects(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	status := httpx.QueryString(r, "status", "")

	countQuery := `SELECT COUNT(*) FROM subjects WHERE tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if search != "" {
		countQuery += ` AND (name ILIKE $` + strconv.Itoa(argIdx) + ` OR code ILIKE $` + strconv.Itoa(argIdx) + `)`
		countArgs = append(countArgs, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		countQuery += ` AND status = $` + strconv.Itoa(argIdx)
		countArgs = append(countArgs, status)
		argIdx++
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		a.logger.Error("count subjects failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "subjects_lookup_failed", "Could not load subjects", r)
		return
	}

	query := `SELECT id, code, name, description, status, created_at FROM subjects WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if search != "" {
		query += ` AND (name ILIKE $` + strconv.Itoa(argIdx) + ` OR code ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY name ASC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list subjects failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "subjects_lookup_failed", "Could not load subjects", r)
		return
	}
	defer rows.Close()

	type SubjectRow struct {
		ID          string  `json:"id"`
		Code        string  `json:"code"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
		Status      string  `json:"status"`
		CreatedAt   string  `json:"createdAt"`
	}

	var subjects []SubjectRow
	for rows.Next() {
		var s SubjectRow
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.Description, &s.Status, &s.CreatedAt); err != nil {
			continue
		}
		subjects = append(subjects, s)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(subjects, p, total))
}

func (a *App) handleCreateSubject(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Code == "" {
		fields["code"] = "Code is required"
	}
	if req.Name == "" {
		fields["name"] = "Name is required"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// Uniqueness
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM subjects WHERE tenant_id = $1 AND code = $2)`, tenantID, req.Code).Scan(&exists)
	if exists {
		writeValidationError(w, map[string]string{"code": "Code already in use"}, r)
		return
	}

	var subjectID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO subjects (tenant_id, code, name, description, status) VALUES ($1, $2, $3, NULLIF($4,''), 'active') RETURNING id`,
		tenantID, req.Code, req.Name, strings.TrimSpace(req.Description),
	).Scan(&subjectID)
	if err != nil {
		a.logger.Error("insert subject failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create subject", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "subjects.create", "subject", subjectID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": subjectID, "code": req.Code, "name": req.Name, "status": "active"})
}

func (a *App) handleUpdateSubject(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	subjectID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM subjects WHERE id = $1 AND tenant_id = $2)`, subjectID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Subject not found", r)
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Name != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE subjects SET name = $1, updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Name), subjectID)
	}
	if req.Description != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE subjects SET description = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Description), subjectID)
	}
	if req.Status != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE subjects SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, subjectID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "subjects.update", "subject", subjectID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": subjectID, "status": "updated"})
}

func (a *App) handleArchiveSubject(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	subjectID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM subjects WHERE id = $1 AND tenant_id = $2)`, subjectID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Subject not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE subjects SET status = 'archived', updated_at = now() WHERE id = $1`, subjectID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "subjects.archive", "subject", subjectID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}

func (a *App) handleListSubjectTeachers(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	subjectID := r.PathValue("id")

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT u.id, u.display_name FROM teacher_subjects ts
		 JOIN teachers t ON t.id = ts.teacher_id
		 JOIN users u ON u.id = t.user_id
		 WHERE ts.tenant_id = $1 AND ts.subject_id = $2 AND ts.status = 'active'
		 ORDER BY u.display_name`,
		tenantID, subjectID,
	)
	if err != nil {
		a.logger.Error("list subject teachers failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed", "Could not load teachers", r)
		return
	}
	defer rows.Close()

	type TeacherRef struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	}

	var teachers []TeacherRef
	for rows.Next() {
		var t TeacherRef
		if err := rows.Scan(&t.ID, &t.DisplayName); err != nil {
			continue
		}
		teachers = append(teachers, t)
	}
	if teachers == nil {
		teachers = []TeacherRef{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": teachers})
}
