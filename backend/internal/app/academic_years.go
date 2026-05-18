package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerAcademicYearRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/academic-years", a.handleListAcademicYears)
	mux.HandleFunc("POST /api/v1/academic-years", a.handleCreateAcademicYear)
	mux.HandleFunc("PATCH /api/v1/academic-years/{id}", a.handleUpdateAcademicYear)
	mux.HandleFunc("PATCH /api/v1/academic-years/{id}/archive", a.handleArchiveAcademicYear)
}

func (a *App) handleListAcademicYears(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	p := httpx.ParsePagination(r)
	status := httpx.QueryString(r, "status", "")

	countQuery := `SELECT COUNT(*) FROM academic_years WHERE tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if status != "" {
		countQuery += ` AND status = $` + strconv.Itoa(argIdx)
		countArgs = append(countArgs, status)
		argIdx++
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		a.logger.Error("count academic years failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "academic_years_lookup_failed", "Could not load academic years", r)
		return
	}

	query := `SELECT id, code, name, starts_on, ends_on, status, created_at FROM academic_years WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if status != "" {
		query += ` AND status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list academic years failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "academic_years_lookup_failed", "Could not load academic years", r)
		return
	}
	defer rows.Close()

	type AcademicYearRow struct {
		ID        string  `json:"id"`
		Code      string  `json:"code"`
		Name      string  `json:"name"`
		StartsOn  *string `json:"startsOn"`
		EndsOn    *string `json:"endsOn"`
		Status    string  `json:"status"`
		CreatedAt string  `json:"createdAt"`
	}

	var years []AcademicYearRow
	for rows.Next() {
		var y AcademicYearRow
		if err := rows.Scan(&y.ID, &y.Code, &y.Name, &y.StartsOn, &y.EndsOn, &y.Status, &y.CreatedAt); err != nil {
			continue
		}
		years = append(years, y)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(years, p, total))
}

func (a *App) handleCreateAcademicYear(w http.ResponseWriter, r *http.Request) {
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
		Code     string `json:"code"`
		Name     string `json:"name"`
		StartsOn string `json:"startsOn"`
		EndsOn   string `json:"endsOn"`
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

	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academic_years WHERE tenant_id = $1 AND code = $2)`, tenantID, req.Code).Scan(&exists)
	if exists {
		writeValidationError(w, map[string]string{"code": "Code already in use"}, r)
		return
	}

	var yearID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO academic_years (tenant_id, code, name, starts_on, ends_on, status)
		 VALUES ($1, $2, $3, NULLIF($4,'')::date, NULLIF($5,'')::date, 'active') RETURNING id`,
		tenantID, req.Code, req.Name, req.StartsOn, req.EndsOn,
	).Scan(&yearID)
	if err != nil {
		a.logger.Error("insert academic year failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create academic year", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "academic_years.create", "academic_year", yearID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": yearID, "code": req.Code, "name": req.Name, "status": "active"})
}

func (a *App) handleUpdateAcademicYear(w http.ResponseWriter, r *http.Request) {
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

	yearID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academic_years WHERE id = $1 AND tenant_id = $2)`, yearID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Academic year not found", r)
		return
	}

	var req struct {
		Name     *string `json:"name"`
		StartsOn *string `json:"startsOn"`
		EndsOn   *string `json:"endsOn"`
		Status   *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.Name != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE academic_years SET name = $1, updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Name), yearID)
	}
	if req.StartsOn != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE academic_years SET starts_on = NULLIF($1,'')::date, updated_at = now() WHERE id = $2`, *req.StartsOn, yearID)
	}
	if req.EndsOn != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE academic_years SET ends_on = NULLIF($1,'')::date, updated_at = now() WHERE id = $2`, *req.EndsOn, yearID)
	}
	if req.Status != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE academic_years SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, yearID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "academic_years.update", "academic_year", yearID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": yearID, "status": "updated"})
}

func (a *App) handleArchiveAcademicYear(w http.ResponseWriter, r *http.Request) {
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

	yearID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM academic_years WHERE id = $1 AND tenant_id = $2)`, yearID, tenantID).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Academic year not found", r)
		return
	}

	_, _ = a.db.ExecContext(r.Context(), `UPDATE academic_years SET status = 'archived', updated_at = now() WHERE id = $1`, yearID)

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "academic_years.archive", "academic_year", yearID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}
