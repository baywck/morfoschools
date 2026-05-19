package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users", a.handleListUsers)
	mux.HandleFunc("POST /api/v1/users", a.handleCreateUser)
	mux.HandleFunc("PATCH /api/v1/users/{id}", a.handleUpdateUser)
	mux.HandleFunc("PATCH /api/v1/users/{id}/archive", a.handleArchiveUser)
	mux.HandleFunc("PATCH /api/v1/users/{id}/restore", a.handleRestoreUser)
}

// --- List Users ---

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
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
	roleFilter := httpx.QueryString(r, "role", "")

	// Count
	countQuery := `SELECT COUNT(*) FROM users u JOIN tenant_memberships tm ON tm.user_id = u.id WHERE tm.tenant_id = $1`
	countArgs := []any{tenantID}
	argIdx := 2

	if roleFilter != "" {
		countQuery += ` AND EXISTS(SELECT 1 FROM user_roles ur JOIN roles rl ON rl.id = ur.role_id WHERE ur.tenant_id = $1 AND ur.user_id = u.id AND rl.slug = $` + itoa(argIdx) + `)`
		countArgs = append(countArgs, roleFilter)
		argIdx++
	}

	if search != "" {
		countQuery += ` AND (u.display_name ILIKE $` + itoa(argIdx) + ` OR u.email ILIKE $` + itoa(argIdx) + `)`
		countArgs = append(countArgs, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		countQuery += ` AND u.status = $` + itoa(argIdx)
		countArgs = append(countArgs, status)
		argIdx++
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		a.logger.Error("count users failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "users_lookup_failed", "Could not load users", r)
		return
	}

	// Query
	query := `SELECT u.id, u.email, u.display_name, u.status, u.is_platform_admin, u.created_at
		FROM users u
		JOIN tenant_memberships tm ON tm.user_id = u.id
		WHERE tm.tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if roleFilter != "" {
		query += ` AND EXISTS(SELECT 1 FROM user_roles ur JOIN roles rl ON rl.id = ur.role_id WHERE ur.tenant_id = $1 AND ur.user_id = u.id AND rl.slug = $` + itoa(argIdx) + `)`
		args = append(args, roleFilter)
		argIdx++
	}

	if search != "" {
		query += ` AND (u.display_name ILIKE $` + itoa(argIdx) + ` OR u.email ILIKE $` + itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND u.status = $` + itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY u.created_at DESC LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list users failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "users_lookup_failed", "Could not load users", r)
		return
	}
	defer rows.Close()

	type UserRow struct {
		ID              string `json:"id"`
		Email           string `json:"email"`
		DisplayName     string `json:"displayName"`
		Status          string `json:"status"`
		IsPlatformAdmin bool   `json:"isPlatformAdmin"`
		CreatedAt       string `json:"createdAt"`
	}

	var users []UserRow
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.IsPlatformAdmin, &u.CreatedAt); err != nil {
			a.logger.Error("scan user failed", "error", err)
			continue
		}
		users = append(users, u)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(users, p, total))
}

// --- Create User ---

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
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
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
		RoleSlug    string `json:"roleSlug"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	// Validate
	fields := map[string]string{}
	if req.Email == "" {
		fields["email"] = "Email is required"
	}
	if req.DisplayName == "" {
		fields["displayName"] = "Display name is required"
	}
	if req.Password == "" {
		fields["password"] = "Password is required"
	}
	if len(req.Password) > 0 && len(req.Password) < 6 {
		fields["password"] = "Password must be at least 6 characters"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	if req.RoleSlug == "" {
		req.RoleSlug = "school_admin"
	}

	// Check email uniqueness
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
	if exists {
		writeValidationError(w, map[string]string{"email": "Email already in use"}, r)
		return
	}

	// Create user
	auth := AuthFromContext(r.Context())
	hash := hashPassword(req.Password)

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.logger.Error("begin tx failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}
	defer tx.Rollback()

	var userID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO users (email, display_name, status) VALUES ($1, $2, 'active') RETURNING id`,
		req.Email, req.DisplayName,
	).Scan(&userID)
	if err != nil {
		a.logger.Error("insert user failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}

	// Password
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`,
		userID, hash,
	)
	if err != nil {
		a.logger.Error("insert password failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}

	// Tenant membership
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary) VALUES ($1, $2, 'active', true)`,
		tenantID, userID,
	)
	if err != nil {
		a.logger.Error("insert membership failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}

	// Assign role if provided
	if req.RoleSlug != "" {
		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO user_roles (tenant_id, user_id, role_id)
			 SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = $3`,
			tenantID, userID, req.RoleSlug,
		)
		if err != nil {
			a.logger.Error("assign role failed", "error", err)
		}
	}

	if err := tx.Commit(); err != nil {
		a.logger.Error("commit failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}

	// Audit
	a.audit(r.Context(), &tenantID, auth.UserID, "users.create", "user", userID, r)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          userID,
		"email":       req.Email,
		"displayName": req.DisplayName,
		"status":      "active",
	})
}

// --- Update User ---

func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
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

	userID := r.PathValue("id")
	if userID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "User ID is required", r)
		return
	}

	// Verify user belongs to tenant
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, userID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "User not found", r)
		return
	}

	var req struct {
		DisplayName *string `json:"displayName"`
		Email       *string `json:"email"`
		Password    *string `json:"password"`
		Status      *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	// Build update
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email == "" {
			writeValidationError(w, map[string]string{"email": "Email is required"}, r)
			return
		}
		// Check uniqueness
		var emailTaken bool
		_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id != $2)`, email, userID).Scan(&emailTaken)
		if emailTaken {
			writeValidationError(w, map[string]string{"email": "Email already in use"}, r)
			return
		}
		_, err := a.db.ExecContext(r.Context(), `UPDATE users SET email = $1, updated_at = now() WHERE id = $2`, email, userID)
		if err != nil {
			a.logger.Error("update user email failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update email", r)
			return
		}
	}
	if req.DisplayName != nil {
		_, err := a.db.ExecContext(r.Context(), `UPDATE users SET display_name = $1, updated_at = now() WHERE id = $2`, *req.DisplayName, userID)
		if err != nil {
			a.logger.Error("update user name failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update user", r)
			return
		}
	}
	if req.Password != nil {
		password := *req.Password
		if len(password) < 6 {
			writeValidationError(w, map[string]string{"password": "Password must be at least 6 characters"}, r)
			return
		}
		hash := hashPassword(password)
		_, err := a.db.ExecContext(r.Context(), `UPDATE password_credentials SET password_hash = $1 WHERE user_id = $2`, hash, userID)
		if err != nil {
			a.logger.Error("update user password failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update password", r)
			return
		}
	}
	if req.Status != nil {
		_, err := a.db.ExecContext(r.Context(), `UPDATE users SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, userID)
		if err != nil {
			a.logger.Error("update user status failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update user", r)
			return
		}
	}

	// Audit
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "users.update", "user", userID, r)

	// Return updated user
	var email, displayName, status string
	var createdAt string
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT email, display_name, status, created_at FROM users WHERE id = $1`, userID,
	).Scan(&email, &displayName, &status, &createdAt)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          userID,
		"email":       email,
		"displayName": displayName,
		"status":      status,
	})
}

// --- Archive User ---

func (a *App) handleArchiveUser(w http.ResponseWriter, r *http.Request) {
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

	userID := r.PathValue("id")
	if userID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "User ID is required", r)
		return
	}

	// Verify user belongs to tenant
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, userID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "User not found", r)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.logger.Error("begin tx failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive user", r)
		return
	}
	defer tx.Rollback()

	if err := archiveUser(r.Context(), tx, userID); err != nil {
		a.logger.Error("archive user failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive user", r)
		return
	}
	// User-level archive cascades to all profile rows so their dashboards
	// disappear from active lists too. Profile-level archive does NOT cascade
	// up; that direction is handled by cascadeArchiveUserIfOrphan in the
	// per-profile handlers.
	if err := archiveAllProfilesForUser(r.Context(), tx, userID); err != nil {
		a.logger.Error("archive cascade profiles failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive user", r)
		return
	}
	if err := tx.Commit(); err != nil {
		a.logger.Error("commit archive failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive user", r)
		return
	}

	// Audit
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "users.archive", "user", userID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
}

// --- Restore User ---

func (a *App) handleRestoreUser(w http.ResponseWriter, r *http.Request) {
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

	userID := r.PathValue("id")
	if userID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "User ID is required", r)
		return
	}

	// Verify user belongs to tenant
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, userID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "User not found", r)
		return
	}

	// Optional admin override when the original email is already taken by
	// somebody else (e.g. school re-issued it after archive).
	var req struct {
		Email *string `json:"email"`
	}
	_ = readJSON(r, &req)
	override := ""
	if req.Email != nil {
		override = strings.TrimSpace(*req.Email)
	}

	resolved, taken, err := restoreUser(r.Context(), a.db, userID, override)
	if err != nil {
		a.logger.Error("restore user failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore user", r)
		return
	}
	if taken {
		writeValidationError(w, map[string]string{
			"email": "Email " + resolved + " is already in use. Provide a different email to restore this user.",
		}, r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "users.restore", "user", userID, r)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     userID,
		"status": "active",
		"email":  resolved,
	})
}

// --- Helpers ---

func itoa(n int) string {
	return strconv.Itoa(n)
}

func ptrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
