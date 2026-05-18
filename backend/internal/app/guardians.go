package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerGuardianRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/guardians", a.handleListGuardians)
	mux.HandleFunc("POST /api/v1/guardians", a.handleCreateGuardian)
	mux.HandleFunc("PATCH /api/v1/guardians/{id}", a.handleUpdateGuardian)
	mux.HandleFunc("PATCH /api/v1/guardians/{id}/archive", a.handleArchiveGuardian)
	mux.HandleFunc("POST /api/v1/guardians/{id}/link-student", a.handleLinkStudentGuardian)
}

func (a *App) handleListGuardians(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "users:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	status := httpx.QueryString(r, "status", "")

	countQuery := `SELECT COUNT(*) FROM guardians WHERE tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if search != "" {
		countQuery += ` AND (name ILIKE $` + strconv.Itoa(argIdx) + ` OR email ILIKE $` + strconv.Itoa(argIdx) + `)`
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
		a.logger.Error("count guardians failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "guardians_lookup_failed", "Could not load guardians", r)
		return
	}

	query := `SELECT id, user_id, name, phone, email, relationship, status, created_at FROM guardians WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if search != "" {
		query += ` AND (name ILIKE $` + strconv.Itoa(argIdx) + ` OR email ILIKE $` + strconv.Itoa(argIdx) + `)`
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
		a.logger.Error("list guardians failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "guardians_lookup_failed", "Could not load guardians", r)
		return
	}
	defer rows.Close()

	type GuardianRow struct {
		ID           string  `json:"id"`
		UserID       *string `json:"userId"`
		Name         string  `json:"name"`
		Phone        *string `json:"phone"`
		Email        *string `json:"email"`
		Relationship *string `json:"relationship"`
		Status       string  `json:"status"`
		CreatedAt    string  `json:"createdAt"`
	}

	var guardians []GuardianRow
	for rows.Next() {
		var g GuardianRow
		if err := rows.Scan(&g.ID, &g.UserID, &g.Name, &g.Phone, &g.Email, &g.Relationship, &g.Status, &g.CreatedAt); err != nil {
			a.logger.Error("scan guardian failed", "error", err)
			continue
		}
		guardians = append(guardians, g)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(guardians, p, total))
}

func (a *App) handleCreateGuardian(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "users:write") {
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
		Name         string  `json:"name"`
		Phone        string  `json:"phone"`
		Email        string  `json:"email"`
		Relationship string  `json:"relationship"`
		UserID       *string `json:"userId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		fields["name"] = "Name is required"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	var guardianID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO guardians (tenant_id, user_id, name, phone, email, relationship, status)
		 VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), 'active') RETURNING id`,
		tenantID, req.UserID, strings.TrimSpace(req.Name), strings.TrimSpace(req.Phone), strings.TrimSpace(req.Email), strings.TrimSpace(req.Relationship),
	).Scan(&guardianID)
	if err != nil {
		a.logger.Error("insert guardian failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create guardian", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "guardians.create", "guardian", guardianID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": guardianID, "name": req.Name, "status": "active"})
}

func (a *App) handleUpdateGuardian(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "users:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	guardianID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM guardians WHERE id = $1 AND tenant_id = $2)`,
		guardianID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Guardian not found", r)
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Phone        *string `json:"phone"`
		Email        *string `json:"email"`
		Relationship *string `json:"relationship"`
		Status       *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Name != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE guardians SET name = $1, updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Name), guardianID)
	}
	if req.Phone != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE guardians SET phone = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Phone), guardianID)
	}
	if req.Email != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE guardians SET email = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Email), guardianID)
	}
	if req.Relationship != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE guardians SET relationship = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Relationship), guardianID)
	}
	if req.Status != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE guardians SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, guardianID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "guardians.update", "guardian", guardianID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": guardianID, "status": "updated"})
}

func (a *App) handleArchiveGuardian(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "users:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	guardianID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM guardians WHERE id = $1 AND tenant_id = $2)`,
		guardianID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Guardian not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE guardians SET status = 'archived', updated_at = now() WHERE id = $1`, guardianID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "guardians.archive", "guardian", guardianID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}

// --- Link Student-Guardian ---

func (a *App) handleLinkStudentGuardian(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "users:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	guardianID := r.PathValue("id")
	var guardianExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM guardians WHERE id = $1 AND tenant_id = $2)`,
		guardianID, tenantID,
	).Scan(&guardianExists)
	if !guardianExists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Guardian not found", r)
		return
	}

	var req struct {
		StudentID string `json:"studentId"`
		IsPrimary bool   `json:"isPrimary"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	if strings.TrimSpace(req.StudentID) == "" {
		fields["studentId"] = "Student ID is required"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// Verify student belongs to tenant
	var studentExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM students WHERE id = $1 AND tenant_id = $2)`,
		req.StudentID, tenantID,
	).Scan(&studentExists)
	if !studentExists {
		writeValidationError(w, map[string]string{"studentId": "Student not found in this tenant"}, r)
		return
	}

	// If setting as primary, unset existing primary
	if req.IsPrimary {
		_, _ = a.db.ExecContext(r.Context(),
			`UPDATE student_guardians SET is_primary = false WHERE tenant_id = $1 AND student_id = $2 AND is_primary = true`,
			tenantID, req.StudentID,
		)
	}

	var linkID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO student_guardians (tenant_id, student_id, guardian_id, is_primary)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (tenant_id, student_id, guardian_id) DO UPDATE SET is_primary = EXCLUDED.is_primary
		 RETURNING id`,
		tenantID, req.StudentID, guardianID, req.IsPrimary,
	).Scan(&linkID)
	if err != nil {
		a.logger.Error("link student-guardian failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "link_failed", "Could not link student and guardian", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "guardians.link_student", "student_guardian", linkID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": linkID, "studentId": req.StudentID, "guardianId": guardianID, "isPrimary": req.IsPrimary})
}
