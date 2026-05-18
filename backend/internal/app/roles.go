package app

import (
	"net/http"
)

func (a *App) registerRoleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/roles", a.handleListRoles)
}

func (a *App) handleListRoles(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "users:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT id, slug, name FROM roles WHERE tenant_id = $1 ORDER BY name`, tenantID,
	)
	if err != nil {
		a.logger.Error("list roles failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "roles_lookup_failed", "Could not load roles", r)
		return
	}
	defer rows.Close()

	type RoleRow struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
		Name string `json:"name"`
	}

	var roles []RoleRow
	for rows.Next() {
		var role RoleRow
		if err := rows.Scan(&role.ID, &role.Slug, &role.Name); err != nil {
			continue
		}
		roles = append(roles, role)
	}

	if roles == nil {
		roles = []RoleRow{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": roles})
}
