package app

import (
	"net/http"
	"strings"
)

// RequirePermission checks if the authenticated user has the given permission.
// Returns true if authorized, false if it wrote a 403 response.
func (a *App) RequirePermission(w http.ResponseWriter, r *http.Request, permission string) bool {
	auth := AuthFromContext(r.Context())
	if auth == nil {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated", "Not authenticated", r)
		return false
	}

	for _, p := range auth.Permissions {
		if p == permission {
			return true
		}
	}

	writeErrorJSON(w, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", r)
	return false
}

// hasPermission is the non-HTTP predicate version of RequirePermission. It
// answers "does this auth context hold the given permission?" without
// emitting a response. Use this for branching logic (e.g. authors bypass
// gate-window enforcement).
func hasPermission(auth *AuthContext, permission string) bool {
	if auth == nil {
		return false
	}
	for _, p := range auth.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// RequireEffectiveTenant checks if the request has an effective tenant context.
// Returns the tenant ID if present, empty string + 403 if not.
func (a *App) RequireEffectiveTenant(w http.ResponseWriter, r *http.Request) string {
	auth := AuthFromContext(r.Context())
	if auth == nil {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated", "Not authenticated", r)
		return ""
	}

	if auth.EffectiveTenantID == nil || *auth.EffectiveTenantID == "" {
		writeErrorJSON(w, http.StatusForbidden, "tenant_required", "A tenant context is required for this action", r)
		return ""
	}

	return *auth.EffectiveTenantID
}

// RequireCSRF validates the CSRF token for the current request.
// Returns true if valid, false if it wrote a 403 response.
func (a *App) RequireCSRF(w http.ResponseWriter, r *http.Request) bool {
	// Only check unsafe methods
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}

	csrfCookie, err := r.Cookie("csrf_token")
	if err != nil || csrfCookie.Value == "" {
		writeErrorJSON(w, http.StatusForbidden, "csrf_missing", "CSRF token missing", r)
		return false
	}

	csrfHeader := r.Header.Get("X-CSRF-Token")
	if csrfHeader == "" || csrfHeader != csrfCookie.Value {
		writeErrorJSON(w, http.StatusForbidden, "csrf_invalid", "CSRF token invalid", r)
		return false
	}

	return true
}

// SwitchTenant allows a platform admin to switch effective tenant context.
func (a *App) SwitchTenant(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated", "Not authenticated", r)
		return
	}

	if !auth.IsPlatformAdmin {
		writeErrorJSON(w, http.StatusForbidden, "forbidden", "Only platform admins can switch tenant", r)
		return
	}

	var req struct {
		TenantID string `json:"tenantId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if strings.TrimSpace(req.TenantID) == "" {
		writeValidationError(w, map[string]string{"tenantId": "Tenant ID is required"}, r)
		return
	}

	// Verify tenant exists
	var exists bool
	err := a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1 AND status = 'active')`,
		req.TenantID,
	).Scan(&exists)
	if err != nil || !exists {
		writeErrorJSON(w, http.StatusNotFound, "tenant_not_found", "Tenant not found", r)
		return
	}

	// Update session
	_, err = a.db.ExecContext(r.Context(),
		`UPDATE sessions SET effective_tenant_id = $1 WHERE id = $2`,
		req.TenantID, auth.SessionID,
	)
	if err != nil {
		a.logger.Error("switch tenant failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "switch_failed", "Failed to switch tenant", r)
		return
	}

	// Audit
	a.audit(r.Context(), &req.TenantID, auth.UserID, "tenant.switch", "session", auth.SessionID, r)

	writeJSON(w, http.StatusOK, map[string]any{
		"effectiveTenantId": req.TenantID,
	})
}
