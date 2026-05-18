package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerStaffRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/staff", a.handleListStaff)
	mux.HandleFunc("POST /api/v1/staff", a.handleCreateStaff)
	mux.HandleFunc("PATCH /api/v1/staff/{id}", a.handleUpdateStaff)
	mux.HandleFunc("PATCH /api/v1/staff/{id}/archive", a.handleArchiveStaff)
}

func (a *App) handleListStaff(w http.ResponseWriter, r *http.Request) {
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

	countQuery := `SELECT COUNT(*) FROM staff_profiles sp JOIN users u ON u.id = sp.user_id WHERE sp.tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if search != "" {
		countQuery += ` AND (u.display_name ILIKE $` + strconv.Itoa(argIdx) + ` OR u.email ILIKE $` + strconv.Itoa(argIdx) + `)`
		countArgs = append(countArgs, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		countQuery += ` AND sp.status = $` + strconv.Itoa(argIdx)
		countArgs = append(countArgs, status)
		argIdx++
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		a.logger.Error("count staff failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "staff_lookup_failed", "Could not load staff", r)
		return
	}

	query := `SELECT sp.id, sp.user_id, u.email, u.display_name, sp.employee_id, sp.department, sp.position, sp.status, sp.created_at
		FROM staff_profiles sp JOIN users u ON u.id = sp.user_id
		WHERE sp.tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if search != "" {
		query += ` AND (u.display_name ILIKE $` + strconv.Itoa(argIdx) + ` OR u.email ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND sp.status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY sp.created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list staff failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "staff_lookup_failed", "Could not load staff", r)
		return
	}
	defer rows.Close()

	type StaffRow struct {
		ID          string  `json:"id"`
		UserID      string  `json:"userId"`
		Email       string  `json:"email"`
		DisplayName string  `json:"displayName"`
		EmployeeID  *string `json:"employeeId"`
		Department  *string `json:"department"`
		Position    *string `json:"position"`
		Status      string  `json:"status"`
		CreatedAt   string  `json:"createdAt"`
	}

	var staff []StaffRow
	for rows.Next() {
		var s StaffRow
		if err := rows.Scan(&s.ID, &s.UserID, &s.Email, &s.DisplayName, &s.EmployeeID, &s.Department, &s.Position, &s.Status, &s.CreatedAt); err != nil {
			a.logger.Error("scan staff failed", "error", err)
			continue
		}
		staff = append(staff, s)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(staff, p, total))
}

func (a *App) handleCreateStaff(w http.ResponseWriter, r *http.Request) {
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
		UserID     string `json:"userId"`
		EmployeeID string `json:"employeeId"`
		Department string `json:"department"`
		Position   string `json:"position"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	if strings.TrimSpace(req.UserID) == "" {
		fields["userId"] = "User ID is required"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	var userExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, req.UserID,
	).Scan(&userExists)
	if !userExists {
		writeValidationError(w, map[string]string{"userId": "User not found in this tenant"}, r)
		return
	}

	var alreadyExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM staff_profiles WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, req.UserID,
	).Scan(&alreadyExists)
	if alreadyExists {
		writeValidationError(w, map[string]string{"userId": "User is already registered as staff"}, r)
		return
	}

	var staffID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO staff_profiles (tenant_id, user_id, employee_id, department, position, status)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), 'active') RETURNING id`,
		tenantID, req.UserID, strings.TrimSpace(req.EmployeeID), strings.TrimSpace(req.Department), strings.TrimSpace(req.Position),
	).Scan(&staffID)
	if err != nil {
		a.logger.Error("insert staff failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create staff", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "staff.create", "staff", staffID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": staffID, "userId": req.UserID, "status": "active"})
}

func (a *App) handleUpdateStaff(w http.ResponseWriter, r *http.Request) {
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

	staffID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM staff_profiles WHERE id = $1 AND tenant_id = $2)`,
		staffID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Staff not found", r)
		return
	}

	var req struct {
		EmployeeID *string `json:"employeeId"`
		Department *string `json:"department"`
		Position   *string `json:"position"`
		Status     *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.EmployeeID != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE staff_profiles SET employee_id = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.EmployeeID), staffID)
	}
	if req.Department != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE staff_profiles SET department = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Department), staffID)
	}
	if req.Position != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE staff_profiles SET position = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Position), staffID)
	}
	if req.Status != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE staff_profiles SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, staffID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "staff.update", "staff", staffID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": staffID, "status": "updated"})
}

func (a *App) handleArchiveStaff(w http.ResponseWriter, r *http.Request) {
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

	staffID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM staff_profiles WHERE id = $1 AND tenant_id = $2)`,
		staffID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Staff not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE staff_profiles SET status = 'archived', updated_at = now() WHERE id = $1`, staffID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "staff.archive", "staff", staffID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}
