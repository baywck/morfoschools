package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerTenantRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/tenants", a.handleListTenants)
	mux.HandleFunc("POST /api/v1/tenants", a.handleCreateTenant)
	mux.HandleFunc("PATCH /api/v1/tenants/{id}", a.handleUpdateTenant)
	mux.HandleFunc("PATCH /api/v1/tenants/{id}/archive", a.handleArchiveTenant)
}

// --- List Tenants (platform admin only) ---

func (a *App) handleListTenants(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "tenants:read") {
		return
	}

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	status := httpx.QueryString(r, "status", "")

	// Count
	countQuery := `SELECT COUNT(*) FROM tenants WHERE 1=1`
	countArgs := []any{}
	argIdx := 1

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
		a.logger.Error("count tenants failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "tenants_lookup_failed", "Could not load tenants", r)
		return
	}

	// Query
	query := `SELECT id, name, code, status, logo_url, created_at FROM tenants WHERE 1=1`
	args := []any{}
	argIdx = 1

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

	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list tenants failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "tenants_lookup_failed", "Could not load tenants", r)
		return
	}
	defer rows.Close()

	type TenantRow struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Code      string  `json:"code"`
		Status    string  `json:"status"`
		LogoURL   *string `json:"logoUrl"`
		CreatedAt string  `json:"createdAt"`
	}

	var tenants []TenantRow
	for rows.Next() {
		var t TenantRow
		if err := rows.Scan(&t.ID, &t.Name, &t.Code, &t.Status, &t.LogoURL, &t.CreatedAt); err != nil {
			a.logger.Error("scan tenant failed", "error", err)
			continue
		}
		tenants = append(tenants, t)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(tenants, p, total))
}

// --- Create Tenant ---

func (a *App) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "tenants:write") {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	var req struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	// Validate
	fields := map[string]string{}
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	if req.Name == "" {
		fields["name"] = "Name is required"
	}
	if req.Code == "" {
		fields["code"] = "Code is required"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// Check code uniqueness
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tenants WHERE code = $1)`, req.Code).Scan(&exists)
	if exists {
		writeValidationError(w, map[string]string{"code": "Code already in use"}, r)
		return
	}

	// Insert
	auth := AuthFromContext(r.Context())
	var tenantID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO tenants (name, code, status) VALUES ($1, $2, 'active') RETURNING id`,
		req.Name, req.Code,
	).Scan(&tenantID)
	if err != nil {
		a.logger.Error("insert tenant failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create tenant", r)
		return
	}

	// Create default theme
	_, _ = a.db.ExecContext(r.Context(),
		`INSERT INTO tenant_theme_settings (tenant_id, preset, primary_color, accent_color, brand_name)
		 VALUES ($1, 'default', 'oklch(0.55 0.15 250)', 'oklch(0.6 0.18 145)', $2)`,
		tenantID, req.Name,
	)

	// Seed default roles for the new tenant
	defaultRoles := []struct{ slug, name string }{
		{"master_admin", "Master Admin"},
		{"school_admin", "School Admin"},
		{"academic_admin", "Academic Admin"},
		{"teacher", "Teacher"},
		{"student", "Student"},
		{"staff", "Staff"},
		{"guardian", "Guardian"},
	}
	for _, role := range defaultRoles {
		_, _ = a.db.ExecContext(r.Context(),
			`INSERT INTO roles (tenant_id, slug, name) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			tenantID, role.slug, role.name,
		)
	}

	// Audit
	a.audit(r.Context(), &tenantID, auth.UserID, "tenants.create", "tenant", tenantID, r)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     tenantID,
		"name":   req.Name,
		"code":   req.Code,
		"status": "active",
	})
}

// --- Update Tenant ---

func (a *App) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "tenants:write") {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	tenantID := r.PathValue("id")
	if tenantID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Tenant ID is required", r)
		return
	}

	// Verify tenant exists
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1)`, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Tenant not found", r)
		return
	}

	var req struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Name != nil {
		_, err := a.db.ExecContext(r.Context(), `UPDATE tenants SET name = $1, updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Name), tenantID)
		if err != nil {
			a.logger.Error("update tenant name failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update tenant", r)
			return
		}
	}
	if req.Status != nil {
		_, err := a.db.ExecContext(r.Context(), `UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, tenantID)
		if err != nil {
			a.logger.Error("update tenant status failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update tenant", r)
			return
		}
	}

	// Audit
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "tenants.update", "tenant", tenantID, r)

	// Return updated
	var name, code, status string
	_ = a.db.QueryRowContext(r.Context(), `SELECT name, code, status FROM tenants WHERE id = $1`, tenantID).Scan(&name, &code, &status)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     tenantID,
		"name":   name,
		"code":   code,
		"status": status,
	})
}

// --- Archive Tenant ---

func (a *App) handleArchiveTenant(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "tenants:write") {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	tenantID := r.PathValue("id")
	if tenantID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Tenant ID is required", r)
		return
	}

	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1)`, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Tenant not found", r)
		return
	}

	_, err := a.db.ExecContext(r.Context(), `UPDATE tenants SET status = 'archived', updated_at = now() WHERE id = $1`, tenantID)
	if err != nil {
		a.logger.Error("archive tenant failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive tenant", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "tenants.archive", "tenant", tenantID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}
