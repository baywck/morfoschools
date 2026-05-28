package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerClassSectionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/class-sections", a.handleListClassSections)
	mux.HandleFunc("POST /api/v1/class-sections", a.handleCreateClassSection)
	mux.HandleFunc("PATCH /api/v1/class-sections/{id}", a.handleUpdateClassSection)
	mux.HandleFunc("PATCH /api/v1/class-sections/{id}/archive", a.handleArchiveClassSection)
}

func (a *App) handleListClassSections(w http.ResponseWriter, r *http.Request) {
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

	countQuery := `SELECT COUNT(*) FROM class_sections WHERE tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if search != "" {
		countQuery += ` AND (name ILIKE $` + strconv.Itoa(argIdx) + ` OR grade_level ILIKE $` + strconv.Itoa(argIdx) + `)`
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
		a.logger.Error("count class sections failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "class_sections_lookup_failed", "Could not load class sections", r)
		return
	}

	query := `SELECT cs.id, cs.name, cs.grade_level, cs.homeroom_teacher_id, cs.capacity, cs.status, cs.created_at,
		COALESCE(u.display_name, '') as teacher_name
		FROM class_sections cs
		LEFT JOIN teachers t ON t.id = cs.homeroom_teacher_id
		LEFT JOIN users u ON u.id = t.user_id
		WHERE cs.tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if search != "" {
		query += ` AND (cs.name ILIKE $` + strconv.Itoa(argIdx) + ` OR cs.grade_level ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND cs.status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY cs.grade_level ASC, cs.name ASC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list class sections failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "class_sections_lookup_failed", "Could not load class sections", r)
		return
	}
	defer rows.Close()

	type ClassSectionRow struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		GradeLevel        string  `json:"gradeLevel"`
		HomeroomTeacherID *string `json:"homeroomTeacherId"`
		Capacity          *int    `json:"capacity"`
		Status            string  `json:"status"`
		CreatedAt         string  `json:"createdAt"`
		TeacherName       string  `json:"teacherName"`
	}

	var sections []ClassSectionRow
	for rows.Next() {
		var s ClassSectionRow
		if err := rows.Scan(&s.ID, &s.Name, &s.GradeLevel, &s.HomeroomTeacherID, &s.Capacity, &s.Status, &s.CreatedAt, &s.TeacherName); err != nil {
			a.logger.Error("scan class section failed", "error", err)
			continue
		}
		sections = append(sections, s)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(sections, p, total))
}

func (a *App) handleCreateClassSection(w http.ResponseWriter, r *http.Request) {
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
		Name              string  `json:"name"`
		GradeLevel        string  `json:"gradeLevel"`
		AcademicYearID    string  `json:"academicYearId"`
		HomeroomTeacherID *string `json:"homeroomTeacherId"`
		Capacity          *int    `json:"capacity"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	req.Name = strings.TrimSpace(req.Name)
	req.GradeLevel = strings.TrimSpace(req.GradeLevel)
	req.AcademicYearID = strings.TrimSpace(req.AcademicYearID)
	if req.Name == "" {
		fields["name"] = "Name is required"
	}
	if req.GradeLevel == "" {
		fields["gradeLevel"] = "Grade level is required"
	}
	if req.AcademicYearID == "" {
		fields["academicYearId"] = "Academic year is required"
	}
	for key, message := range a.validateTenantGradeLevel(r.Context(), tenantID, req.GradeLevel) {
		fields[key] = message
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// Validate homeroom teacher belongs to tenant
	if req.HomeroomTeacherID != nil && *req.HomeroomTeacherID != "" {
		var teacherExists bool
		_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM teachers WHERE id = $1 AND tenant_id = $2)`, *req.HomeroomTeacherID, tenantID).Scan(&teacherExists)
		if !teacherExists {
			writeValidationError(w, map[string]string{"homeroomTeacherId": "Teacher not found"}, r)
			return
		}
	}

	var sectionID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO class_sections (tenant_id, academic_year_id, name, grade_level, homeroom_teacher_id, capacity, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'active') RETURNING id`,
		tenantID, req.AcademicYearID, req.Name, req.GradeLevel, req.HomeroomTeacherID, req.Capacity,
	).Scan(&sectionID)
	if err != nil {
		a.logger.Error("insert class section failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create class section", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "class_sections.create", "class_section", sectionID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": sectionID, "name": req.Name, "gradeLevel": req.GradeLevel, "status": "active"})
}

func (a *App) handleUpdateClassSection(w http.ResponseWriter, r *http.Request) {
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

	sectionID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM class_sections WHERE id = $1 AND tenant_id = $2)`, sectionID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Class section not found", r)
		return
	}

	var req struct {
		Name              *string `json:"name"`
		GradeLevel        *string `json:"gradeLevel"`
		HomeroomTeacherID *string `json:"homeroomTeacherId"`
		Capacity          *int    `json:"capacity"`
		Status            *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Name != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE class_sections SET name = $1, updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Name), sectionID)
	}
	if req.GradeLevel != nil {
		gradeLevel := strings.TrimSpace(*req.GradeLevel)
		if fields := a.validateTenantGradeLevel(r.Context(), tenantID, gradeLevel); len(fields) > 0 {
			writeValidationError(w, fields, r)
			return
		}
		_, _ = a.db.ExecContext(r.Context(), `UPDATE class_sections SET grade_level = $1, updated_at = now() WHERE id = $2`, gradeLevel, sectionID)
	}
	if req.HomeroomTeacherID != nil {
		if *req.HomeroomTeacherID == "" {
			_, _ = a.db.ExecContext(r.Context(), `UPDATE class_sections SET homeroom_teacher_id = NULL, updated_at = now() WHERE id = $1`, sectionID)
		} else {
			_, _ = a.db.ExecContext(r.Context(), `UPDATE class_sections SET homeroom_teacher_id = $1, updated_at = now() WHERE id = $2`, *req.HomeroomTeacherID, sectionID)
		}
	}
	if req.Capacity != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE class_sections SET capacity = $1, updated_at = now() WHERE id = $2`, *req.Capacity, sectionID)
	}
	if req.Status != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE class_sections SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, sectionID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "class_sections.update", "class_section", sectionID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": sectionID, "status": "updated"})
}

func (a *App) handleArchiveClassSection(w http.ResponseWriter, r *http.Request) {
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

	sectionID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM class_sections WHERE id = $1 AND tenant_id = $2)`, sectionID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Class section not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE class_sections SET status = 'archived', updated_at = now() WHERE id = $1`, sectionID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "class_sections.archive", "class_section", sectionID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}
