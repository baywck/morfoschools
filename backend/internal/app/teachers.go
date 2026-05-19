package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerTeacherRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/teachers", a.handleListTeachers)
	mux.HandleFunc("POST /api/v1/teachers", a.handleCreateTeacher)
	mux.HandleFunc("PATCH /api/v1/teachers/{id}", a.handleUpdateTeacher)
	mux.HandleFunc("PATCH /api/v1/teachers/{id}/archive", a.handleArchiveTeacher)
	mux.HandleFunc("PATCH /api/v1/teachers/{id}/restore", a.handleRestoreTeacher)
}

func (a *App) handleListTeachers(w http.ResponseWriter, r *http.Request) {
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

	countQuery := `SELECT COUNT(*) FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if search != "" {
		countQuery += ` AND (u.display_name ILIKE $` + strconv.Itoa(argIdx) + ` OR u.email ILIKE $` + strconv.Itoa(argIdx) + `)`
		countArgs = append(countArgs, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		countQuery += ` AND t.status = $` + strconv.Itoa(argIdx)
		countArgs = append(countArgs, status)
		argIdx++
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		a.logger.Error("count teachers failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "teachers_lookup_failed", "Could not load teachers", r)
		return
	}

	query := `SELECT t.id, t.user_id, u.email, u.display_name, t.employee_id, t.specialization, t.status, t.created_at
		FROM teachers t JOIN users u ON u.id = t.user_id
		WHERE t.tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if search != "" {
		query += ` AND (u.display_name ILIKE $` + strconv.Itoa(argIdx) + ` OR u.email ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND t.status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY t.created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list teachers failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "teachers_lookup_failed", "Could not load teachers", r)
		return
	}
	defer rows.Close()

	type TeacherRow struct {
		ID             string  `json:"id"`
		UserID         string  `json:"userId"`
		Email          string  `json:"email"`
		DisplayName    string  `json:"displayName"`
		EmployeeID     *string `json:"employeeId"`
		Specialization *string `json:"specialization"`
		Status         string  `json:"status"`
		CreatedAt      string  `json:"createdAt"`
	}

	var teachers []TeacherRow
	for rows.Next() {
		var t TeacherRow
		if err := rows.Scan(&t.ID, &t.UserID, &t.Email, &t.DisplayName, &t.EmployeeID, &t.Specialization, &t.Status, &t.CreatedAt); err != nil {
			a.logger.Error("scan teacher failed", "error", err)
			continue
		}
		teachers = append(teachers, t)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(teachers, p, total))
}

func (a *App) handleCreateTeacher(w http.ResponseWriter, r *http.Request) {
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
		UserID         string `json:"userId"`
		EmployeeID     string `json:"employeeId"`
		Specialization string `json:"specialization"`
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

	// Verify user belongs to tenant
	var userExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, req.UserID,
	).Scan(&userExists)
	if !userExists {
		writeValidationError(w, map[string]string{"userId": "User not found in this tenant"}, r)
		return
	}

	// Check not already a teacher
	var alreadyExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM teachers WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, req.UserID,
	).Scan(&alreadyExists)
	if alreadyExists {
		writeValidationError(w, map[string]string{"userId": "User is already registered as a teacher"}, r)
		return
	}

	var teacherID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO teachers (tenant_id, user_id, employee_id, specialization, status)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), 'active') RETURNING id`,
		tenantID, req.UserID, strings.TrimSpace(req.EmployeeID), strings.TrimSpace(req.Specialization),
	).Scan(&teacherID)
	if err != nil {
		a.logger.Error("insert teacher failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create teacher", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "teachers.create", "teacher", teacherID, r)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     teacherID,
		"userId": req.UserID,
		"status": "active",
	})
}

func (a *App) handleUpdateTeacher(w http.ResponseWriter, r *http.Request) {
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

	teacherID := r.PathValue("id")
	if teacherID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Teacher ID is required", r)
		return
	}

	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM teachers WHERE id = $1 AND tenant_id = $2)`,
		teacherID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Teacher not found", r)
		return
	}

	var req struct {
		EmployeeID     *string `json:"employeeId"`
		Specialization *string `json:"specialization"`
		Status         *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.EmployeeID != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE teachers SET employee_id = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.EmployeeID), teacherID)
	}
	if req.Specialization != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE teachers SET specialization = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.Specialization), teacherID)
	}
	if req.Status != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE teachers SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, teacherID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "teachers.update", "teacher", teacherID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": teacherID, "status": "updated"})
}

func (a *App) handleArchiveTeacher(w http.ResponseWriter, r *http.Request) {
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

	teacherID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM teachers WHERE id = $1 AND tenant_id = $2)`,
		teacherID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Teacher not found", r)
		return
	}

	userID, _ := userIDForProfile(r.Context(), a.db, "teachers", teacherID)

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.logger.Error("begin tx failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive teacher", r)
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(),
		`UPDATE teachers SET status = 'archived', updated_at = now() WHERE id = $1`, teacherID,
	); err != nil {
		a.logger.Error("archive teacher failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive teacher", r)
		return
	}

	cascaded, err := cascadeArchiveUserIfOrphan(r.Context(), tx, userID)
	if err != nil {
		a.logger.Error("cascade archive user failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive teacher", r)
		return
	}
	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive teacher", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "teachers.archive", "teacher", teacherID, r)
	if cascaded {
		a.audit(r.Context(), &tenantID, auth.UserID, "users.archive_cascade", "user", userID, r)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "archived",
		"userArchived": cascaded,
	})
}

func (a *App) handleRestoreTeacher(w http.ResponseWriter, r *http.Request) {
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

	teacherID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM teachers WHERE id = $1 AND tenant_id = $2)`,
		teacherID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Teacher not found", r)
		return
	}

	userID, _ := userIDForProfile(r.Context(), a.db, "teachers", teacherID)

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore teacher", r)
		return
	}
	defer tx.Rollback()

	if err := restoreProfile(r.Context(), tx, "teachers", teacherID, "active"); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore teacher", r)
		return
	}
	resolved, taken, err := restoreUser(r.Context(), tx, userID, "")
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore teacher", r)
		return
	}
	if taken {
		writeValidationError(w, map[string]string{
			"email": "Email " + resolved + " is already in use. Restore the user account manually with a new email first.",
		}, r)
		return
	}
	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore teacher", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "teachers.restore", "teacher", teacherID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": teacherID, "status": "active"})
}
