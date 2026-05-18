package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerCourseRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/courses", a.handleListCourses)
	mux.HandleFunc("POST /api/v1/courses", a.handleCreateCourse)
	mux.HandleFunc("PATCH /api/v1/courses/{id}", a.handleUpdateCourse)
	mux.HandleFunc("PATCH /api/v1/courses/{id}/archive", a.handleArchiveCourse)
	mux.HandleFunc("PATCH /api/v1/courses/{id}/publish", a.handlePublishCourse)
}

func (a *App) handleListCourses(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "courses:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	status := httpx.QueryString(r, "status", "")

	countQuery := `SELECT COUNT(*) FROM courses WHERE tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if search != "" {
		countQuery += ` AND (title ILIKE $` + strconv.Itoa(argIdx) + `)`
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
		a.logger.Error("count courses failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "courses_lookup_failed", "Could not load courses", r)
		return
	}

	query := `SELECT id, title, description, status, published_at, created_at FROM courses WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if search != "" {
		query += ` AND (title ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list courses failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "courses_lookup_failed", "Could not load courses", r)
		return
	}
	defer rows.Close()

	type CourseRow struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description *string `json:"description"`
		Status      string  `json:"status"`
		PublishedAt *string `json:"publishedAt"`
		CreatedAt   string  `json:"createdAt"`
	}

	var courses []CourseRow
	for rows.Next() {
		var c CourseRow
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Status, &c.PublishedAt, &c.CreatedAt); err != nil {
			continue
		}
		courses = append(courses, c)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(courses, p, total))
}

func (a *App) handleCreateCourse(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "courses:write") {
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
		Title       string `json:"title"`
		Description string `json:"description"`
		SubjectID   string `json:"subjectId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeValidationError(w, map[string]string{"title": "Title is required"}, r)
		return
	}

	auth := AuthFromContext(r.Context())
	var courseID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO courses (tenant_id, title, description, subject_id, created_by, status)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,'')::uuid, $5, 'draft') RETURNING id`,
		tenantID, req.Title, strings.TrimSpace(req.Description), req.SubjectID, auth.UserID,
	).Scan(&courseID)
	if err != nil {
		a.logger.Error("insert course failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create course", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "courses.create", "course", courseID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": courseID, "title": req.Title, "status": "draft"})
}

func (a *App) handleUpdateCourse(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "courses:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	courseID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1 AND tenant_id = $2)`, courseID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Course not found", r)
		return
	}

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Title != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE courses SET title = $1, updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Title), courseID)
	}
	if req.Description != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE courses SET description = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Description), courseID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "courses.update", "course", courseID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": courseID, "status": "updated"})
}

func (a *App) handlePublishCourse(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "courses:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	courseID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1 AND tenant_id = $2)`, courseID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Course not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE courses SET status = 'published', published_at = now(), updated_at = now() WHERE id = $1`, courseID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "courses.publish", "course", courseID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": courseID, "status": "published"})
}

func (a *App) handleArchiveCourse(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "courses:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	courseID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1 AND tenant_id = $2)`, courseID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Course not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE courses SET status = 'archived', archived_at = now(), updated_at = now() WHERE id = $1`, courseID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "courses.archive", "course", courseID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}
