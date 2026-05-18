package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

var jsonUnmarshal = json.Unmarshal

func (a *App) registerProgramRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/programs", a.handleListPrograms)
	mux.HandleFunc("POST /api/v1/programs", a.handleCreateProgram)
	mux.HandleFunc("PATCH /api/v1/programs/{id}", a.handleUpdateProgram)
	mux.HandleFunc("PATCH /api/v1/programs/{id}/archive", a.handleArchiveProgram)
	mux.HandleFunc("PATCH /api/v1/programs/{id}/publish", a.handlePublishProgram)
	// Sections
	mux.HandleFunc("GET /api/v1/programs/{id}/sections", a.handleListProgramSections)
	mux.HandleFunc("POST /api/v1/programs/{id}/sections", a.handleCreateProgramSection)
	mux.HandleFunc("PATCH /api/v1/program-sections/{sectionId}", a.handleUpdateProgramSection)
	mux.HandleFunc("DELETE /api/v1/program-sections/{sectionId}", a.handleDeleteProgramSection)
	// Items
	mux.HandleFunc("POST /api/v1/program-sections/{sectionId}/items", a.handleCreateProgramItem)
	mux.HandleFunc("PATCH /api/v1/program-items/{itemId}", a.handleUpdateProgramItem)
	mux.HandleFunc("DELETE /api/v1/program-items/{itemId}", a.handleDeleteProgramItem)
}

// --- List Programs ---

func (a *App) handleListPrograms(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	status := httpx.QueryString(r, "status", "")

	countQuery := `SELECT COUNT(*) FROM programs WHERE tenant_id = $1`
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
		a.logger.Error("count programs failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "programs_lookup_failed", "Could not load programs", r)
		return
	}

	query := `SELECT id, title, description, kind, status, grade_level, published_at, created_at FROM programs WHERE tenant_id = $1`
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
		a.logger.Error("list programs failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "programs_lookup_failed", "Could not load programs", r)
		return
	}
	defer rows.Close()

	type ProgramRow struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description *string `json:"description"`
		Kind        string  `json:"kind"`
		Status      string  `json:"status"`
		GradeLevel  *string `json:"gradeLevel"`
		PublishedAt *string `json:"publishedAt"`
		CreatedAt   string  `json:"createdAt"`
	}

	var programs []ProgramRow
	for rows.Next() {
		var p ProgramRow
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Kind, &p.Status, &p.GradeLevel, &p.PublishedAt, &p.CreatedAt); err != nil {
			continue
		}
		programs = append(programs, p)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(programs, p, total))
}

// --- Create Program ---

func (a *App) handleCreateProgram(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
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
		Kind        string `json:"kind"`
		GradeLevel  string `json:"gradeLevel"`
		SubjectID   string `json:"subjectId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		fields["title"] = "Title is required"
	}
	if req.Kind == "" {
		req.Kind = "regular"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	auth := AuthFromContext(r.Context())
	var programID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO programs (tenant_id, title, description, kind, status, grade_level, subject_id, created_by)
		 VALUES ($1, $2, NULLIF($3,''), $4, 'draft', NULLIF($5,''), NULLIF($6,'')::uuid, $7) RETURNING id`,
		tenantID, req.Title, strings.TrimSpace(req.Description), req.Kind, strings.TrimSpace(req.GradeLevel), req.SubjectID, auth.UserID,
	).Scan(&programID)
	if err != nil {
		a.logger.Error("insert program failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create program", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "programs.create", "program", programID, r)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     programID,
		"title":  req.Title,
		"kind":   req.Kind,
		"status": "draft",
	})
}

// --- Update Program ---

func (a *App) handleUpdateProgram(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	programID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM programs WHERE id = $1 AND tenant_id = $2)`, programID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Program not found", r)
		return
	}

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Kind        *string `json:"kind"`
		GradeLevel  *string `json:"gradeLevel"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Title != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE programs SET title = $1, updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Title), programID)
	}
	if req.Description != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE programs SET description = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Description), programID)
	}
	if req.Kind != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE programs SET kind = $1, updated_at = now() WHERE id = $2`, *req.Kind, programID)
	}
	if req.GradeLevel != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE programs SET grade_level = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.GradeLevel), programID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "programs.update", "program", programID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": programID, "status": "updated"})
}

// --- Publish Program ---

func (a *App) handlePublishProgram(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	programID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM programs WHERE id = $1 AND tenant_id = $2)`, programID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Program not found", r)
		return
	}

	_, err := a.db.ExecContext(r.Context(), `UPDATE programs SET status = 'published', published_at = now(), updated_at = now() WHERE id = $1`, programID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "publish_failed", "Could not publish program", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "programs.publish", "program", programID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": programID, "status": "published"})
}

// --- Archive Program ---

func (a *App) handleArchiveProgram(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	programID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM programs WHERE id = $1 AND tenant_id = $2)`, programID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Program not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE programs SET status = 'archived', archived_at = now(), updated_at = now() WHERE id = $1`, programID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "programs.archive", "program", programID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}

// --- Program Sections ---

func (a *App) handleListProgramSections(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	programID := r.PathValue("id")

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT ps.id, ps.title, ps.sort_order, ps.unlock_mode, ps.is_required,
		 (SELECT json_agg(json_build_object('id', pi.id, 'itemType', pi.item_type, 'itemId', pi.item_id, 'sortOrder', pi.sort_order, 'isRequired', pi.is_required, 'passingGrade', pi.passing_grade, 'maxAttempts', pi.max_attempts) ORDER BY pi.sort_order)
		  FROM program_items pi WHERE pi.section_id = ps.id) as items
		 FROM program_sections ps
		 WHERE ps.program_id = $1 AND ps.tenant_id = $2
		 ORDER BY ps.sort_order`,
		programID, tenantID,
	)
	if err != nil {
		a.logger.Error("list program sections failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "sections_lookup_failed", "Could not load sections", r)
		return
	}
	defer rows.Close()

	type SectionRow struct {
		ID         string  `json:"id"`
		Title      string  `json:"title"`
		SortOrder  int     `json:"sortOrder"`
		UnlockMode string  `json:"unlockMode"`
		IsRequired bool    `json:"isRequired"`
		Items      *string `json:"items"`
	}

	var sections []map[string]any
	for rows.Next() {
		var s SectionRow
		if err := rows.Scan(&s.ID, &s.Title, &s.SortOrder, &s.UnlockMode, &s.IsRequired, &s.Items); err != nil {
			continue
		}
		section := map[string]any{
			"id": s.ID, "title": s.Title, "sortOrder": s.SortOrder,
			"unlockMode": s.UnlockMode, "isRequired": s.IsRequired, "items": nil,
		}
		if s.Items != nil {
			// Parse JSON array
			var items []map[string]any
			if err := jsonUnmarshal([]byte(*s.Items), &items); err == nil {
				section["items"] = items
			}
		}
		if section["items"] == nil {
			section["items"] = []map[string]any{}
		}
		sections = append(sections, section)
	}
	if sections == nil {
		sections = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": sections})
}

func (a *App) handleCreateProgramSection(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	programID := r.PathValue("id")

	var req struct {
		Title      string `json:"title"`
		UnlockMode string `json:"unlockMode"`
		IsRequired *bool  `json:"isRequired"`
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
	if req.UnlockMode == "" {
		req.UnlockMode = "sequential"
	}
	isRequired := true
	if req.IsRequired != nil {
		isRequired = *req.IsRequired
	}

	// Get next sort order
	var maxOrder int
	_ = a.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(sort_order), 0) FROM program_sections WHERE program_id = $1`, programID).Scan(&maxOrder)

	var sectionID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO program_sections (tenant_id, program_id, title, sort_order, unlock_mode, is_required)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		tenantID, programID, req.Title, maxOrder+1, req.UnlockMode, isRequired,
	).Scan(&sectionID)
	if err != nil {
		a.logger.Error("insert program section failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create section", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "programs.create_section", "program_section", sectionID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": sectionID, "title": req.Title, "sortOrder": maxOrder + 1})
}

func (a *App) handleUpdateProgramSection(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
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

	var req struct {
		Title      *string `json:"title"`
		SortOrder  *int    `json:"sortOrder"`
		UnlockMode *string `json:"unlockMode"`
		IsRequired *bool   `json:"isRequired"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Title != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_sections SET title = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, strings.TrimSpace(*req.Title), sectionID, tenantID)
	}
	if req.SortOrder != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_sections SET sort_order = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, *req.SortOrder, sectionID, tenantID)
	}
	if req.UnlockMode != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_sections SET unlock_mode = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, *req.UnlockMode, sectionID, tenantID)
	}
	if req.IsRequired != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_sections SET is_required = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, *req.IsRequired, sectionID, tenantID)
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": sectionID, "status": "updated"})
}

func (a *App) handleDeleteProgramSection(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
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
	_, _ = a.db.ExecContext(r.Context(), `DELETE FROM program_sections WHERE id = $1 AND tenant_id = $2`, sectionID, tenantID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "programs.delete_section", "program_section", sectionID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// --- Program Items ---

func (a *App) handleCreateProgramItem(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
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

	var req struct {
		ItemType     string `json:"itemType"`
		ItemID       string `json:"itemId"`
		IsRequired   *bool  `json:"isRequired"`
		PassingGrade *int   `json:"passingGrade"`
		MaxAttempts  *int   `json:"maxAttempts"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	if req.ItemType != "course" && req.ItemType != "exam" {
		fields["itemType"] = "Must be 'course' or 'exam'"
	}
	if strings.TrimSpace(req.ItemID) == "" {
		fields["itemId"] = "Item ID is required"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// Get program_id from section
	var programID string
	err := a.db.QueryRowContext(r.Context(), `SELECT program_id FROM program_sections WHERE id = $1 AND tenant_id = $2`, sectionID, tenantID).Scan(&programID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Section not found", r)
		return
	}

	isRequired := true
	if req.IsRequired != nil {
		isRequired = *req.IsRequired
	}
	maxAttempts := 1
	if req.MaxAttempts != nil {
		maxAttempts = *req.MaxAttempts
	}

	var maxOrder int
	_ = a.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(sort_order), 0) FROM program_items WHERE section_id = $1`, sectionID).Scan(&maxOrder)

	var itemID string
	err = a.db.QueryRowContext(r.Context(),
		`INSERT INTO program_items (tenant_id, program_id, section_id, item_type, item_id, sort_order, is_required, passing_grade, max_attempts)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		tenantID, programID, sectionID, req.ItemType, req.ItemID, maxOrder+1, isRequired, req.PassingGrade, maxAttempts,
	).Scan(&itemID)
	if err != nil {
		a.logger.Error("insert program item failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create item", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "programs.create_item", "program_item", itemID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": itemID, "itemType": req.ItemType, "sortOrder": maxOrder + 1})
}

func (a *App) handleUpdateProgramItem(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	itemID := r.PathValue("itemId")

	var req struct {
		SortOrder    *int  `json:"sortOrder"`
		IsRequired   *bool `json:"isRequired"`
		PassingGrade *int  `json:"passingGrade"`
		MaxAttempts  *int  `json:"maxAttempts"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.SortOrder != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_items SET sort_order = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, *req.SortOrder, itemID, tenantID)
	}
	if req.IsRequired != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_items SET is_required = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, *req.IsRequired, itemID, tenantID)
	}
	if req.PassingGrade != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_items SET passing_grade = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, *req.PassingGrade, itemID, tenantID)
	}
	if req.MaxAttempts != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE program_items SET max_attempts = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`, *req.MaxAttempts, itemID, tenantID)
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": itemID, "status": "updated"})
}

func (a *App) handleDeleteProgramItem(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "programs:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	itemID := r.PathValue("itemId")
	_, _ = a.db.ExecContext(r.Context(), `DELETE FROM program_items WHERE id = $1 AND tenant_id = $2`, itemID, tenantID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "programs.delete_item", "program_item", itemID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}
