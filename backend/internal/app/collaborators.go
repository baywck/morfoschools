package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

// Collaboration helpers per ADR-0009.
//
// Three resources have an explicit ownership + collaborator model:
//   - exams                  via exam_collaborators
//   - courses                via course_collaborators
//   - blueprint_templates    via blueprint_template_collaborators
//
// For exams ONLY, subject-based RBAC is preserved as an institutional
// read-only fallback. A teacher whose teacher_subjects includes the
// exam's subject_id can read the exam (full detail) but cannot write.
// This honors the implicit authority that waka kurikulum / ketua MGMP
// hold in Indonesian schools without forcing per-exam invitations.
//
// Take-exam endpoints (Phase 10: /api/v1/programs/.../exams/.../take)
// MUST NOT call these helpers — student access is enrollment-based
// and these helpers would 404 students on their own assigned exams.

// AccessAction is one of: "read", "write", "manage", "delete".
//
//	read    — list + detail
//	write   — edit metadata, content, blueprint
//	manage  — invite/remove collaborators, transfer ownership
//	delete  — archive, restore, hard-delete
type AccessAction string

const (
	ActionRead   AccessAction = "read"
	ActionWrite  AccessAction = "write"
	ActionManage AccessAction = "manage"
	ActionDelete AccessAction = "delete"
)

// CollabRole is the granted role on a resource.
type CollabRole string

const (
	RoleOwner          CollabRole = "owner"
	RoleEditor         CollabRole = "editor"
	RoleViewer         CollabRole = "viewer"
	RoleSubjectFallback CollabRole = "subject_fallback"
	RoleAdmin          CollabRole = "admin"
	RoleNone           CollabRole = ""
)

// isTenantAdmin returns true when the auth context belongs to one of
// the tenant-wide admin roles. These actors override the collaborator
// model on every resource within their tenant. master_admin works
// across tenants; school_admin and academic_admin within their tenant.
func isTenantAdmin(auth *AuthContext) bool {
	if auth == nil {
		return false
	}
	for _, r := range auth.Roles {
		switch r {
		case "master_admin", "school_admin", "academic_admin":
			return true
		}
	}
	return false
}

// ResourceAccess describes the highest-priority role the caller has on
// a given resource, plus a boolean per action it implies. Returned by
// the resolver functions and consumed by the require* helpers.
type ResourceAccess struct {
	Role     CollabRole
	CanRead  bool
	CanWrite bool
	CanManage bool
	CanDelete bool
}

func (a ResourceAccess) Allows(action AccessAction) bool {
	switch action {
	case ActionRead:
		return a.CanRead
	case ActionWrite:
		return a.CanWrite
	case ActionManage:
		return a.CanManage
	case ActionDelete:
		return a.CanDelete
	}
	return false
}

// roleAccess maps a role to its action set. Centralized here so the
// access matrix stays in one place.
func roleAccess(role CollabRole) ResourceAccess {
	switch role {
	case RoleAdmin:
		return ResourceAccess{Role: role, CanRead: true, CanWrite: true, CanManage: true, CanDelete: true}
	case RoleOwner:
		return ResourceAccess{Role: role, CanRead: true, CanWrite: true, CanManage: true, CanDelete: true}
	case RoleEditor:
		return ResourceAccess{Role: role, CanRead: true, CanWrite: true}
	case RoleViewer:
		return ResourceAccess{Role: role, CanRead: true}
	case RoleSubjectFallback:
		return ResourceAccess{Role: role, CanRead: true}
	default:
		return ResourceAccess{Role: RoleNone}
	}
}

// resolveExamAccess computes the highest-priority role the caller has
// on the given exam. Order: tenant admin > owner > editor > viewer >
// subject institutional fallback > none.
//
// Returns RoleNone if the resource is in a different tenant or does
// not exist; the caller maps that to 404.
func (a *App) resolveExamAccess(
	ctx context.Context, tenantID string, auth *AuthContext, examID string,
) (ResourceAccess, error) {
	if tenantID == "" || examID == "" || auth == nil || auth.UserID == "" {
		return ResourceAccess{}, nil
	}

	// Existence + tenant scope. Owner UID + subject ID retrieved in one query.
	var ownerID string
	var subjectID sql.NullString
	err := a.db.QueryRowContext(ctx,
		`SELECT owner_user_id::text, subject_id::text FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&ownerID, &subjectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResourceAccess{}, nil
		}
		return ResourceAccess{}, err
	}

	if isTenantAdmin(auth) {
		return roleAccess(RoleAdmin), nil
	}
	if ownerID == auth.UserID {
		return roleAccess(RoleOwner), nil
	}

	// Explicit collaborator?
	var role string
	err = a.db.QueryRowContext(ctx,
		`SELECT role FROM exam_collaborators WHERE exam_id = $1 AND user_id = $2`,
		examID, auth.UserID,
	).Scan(&role)
	switch {
	case err == nil:
		return roleAccess(CollabRole(role)), nil
	case !errors.Is(err, sql.ErrNoRows):
		return ResourceAccess{}, err
	}

	// Subject institutional fallback (read-only). Only applies to exams.
	if subjectID.Valid && subjectID.String != "" {
		var fallback bool
		_ = a.db.QueryRowContext(ctx, `
			SELECT EXISTS(
			    SELECT 1 FROM teacher_subjects ts
			      JOIN teachers t ON t.id = ts.teacher_id
			     WHERE t.user_id = $1 AND ts.tenant_id = $2 AND ts.subject_id = $3
			       AND ts.status = 'active' AND t.status = 'active'
			)`,
			auth.UserID, tenantID, subjectID.String,
		).Scan(&fallback)
		if fallback {
			return roleAccess(RoleSubjectFallback), nil
		}
	}

	return roleAccess(RoleNone), nil
}

// resolveCourseAccess: same shape as exam, no subject fallback.
func (a *App) resolveCourseAccess(
	ctx context.Context, tenantID string, auth *AuthContext, courseID string,
) (ResourceAccess, error) {
	if tenantID == "" || courseID == "" || auth == nil || auth.UserID == "" {
		return ResourceAccess{}, nil
	}
	var ownerID string
	err := a.db.QueryRowContext(ctx,
		`SELECT owner_user_id::text FROM courses WHERE id = $1 AND tenant_id = $2`,
		courseID, tenantID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResourceAccess{}, nil
		}
		return ResourceAccess{}, err
	}
	if isTenantAdmin(auth) {
		return roleAccess(RoleAdmin), nil
	}
	if ownerID == auth.UserID {
		return roleAccess(RoleOwner), nil
	}
	var role string
	err = a.db.QueryRowContext(ctx,
		`SELECT role FROM course_collaborators WHERE course_id = $1 AND user_id = $2`,
		courseID, auth.UserID,
	).Scan(&role)
	if err == nil {
		return roleAccess(CollabRole(role)), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ResourceAccess{}, err
	}
	return roleAccess(RoleNone), nil
}

// resolveBlueprintAccess: blueprint_templates only. exam_blueprints
// inherit access from their parent exam — see resolveExamAccess.
func (a *App) resolveBlueprintAccess(
	ctx context.Context, tenantID string, auth *AuthContext, templateID string,
) (ResourceAccess, error) {
	if tenantID == "" || templateID == "" || auth == nil || auth.UserID == "" {
		return ResourceAccess{}, nil
	}
	var ownerID string
	err := a.db.QueryRowContext(ctx,
		`SELECT owner_user_id::text FROM blueprint_templates WHERE id = $1 AND tenant_id = $2`,
		templateID, tenantID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResourceAccess{}, nil
		}
		return ResourceAccess{}, err
	}
	if isTenantAdmin(auth) {
		return roleAccess(RoleAdmin), nil
	}
	if ownerID == auth.UserID {
		return roleAccess(RoleOwner), nil
	}
	var role string
	err = a.db.QueryRowContext(ctx,
		`SELECT role FROM blueprint_template_collaborators WHERE template_id = $1 AND user_id = $2`,
		templateID, auth.UserID,
	).Scan(&role)
	if err == nil {
		return roleAccess(CollabRole(role)), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ResourceAccess{}, err
	}
	return roleAccess(RoleNone), nil
}

// requireExamAccess gates a handler on the requested action against the
// caller's role on the exam. Writes a 404 (read failure) or 403 (write
// failure when reader-level access exists) on rejection — see ADR-0009
// for why the split.
//
// Returns true when the action is allowed.
func (a *App) requireExamAccess(
	w http.ResponseWriter, r *http.Request, examID string, action AccessAction,
) bool {
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return false
	}
	auth := AuthFromContext(r.Context())
	access, err := a.resolveExamAccess(r.Context(), tenantID, auth, examID)
	if err != nil {
		a.logger.Error("resolve exam access failed", "error", err, "examId", examID)
		writeErrorJSON(w, http.StatusInternalServerError, "access_check_failed",
			"Could not verify exam access", r)
		return false
	}
	return enforceAccess(w, r, access, action, "exam")
}

func (a *App) requireCourseAccess(
	w http.ResponseWriter, r *http.Request, courseID string, action AccessAction,
) bool {
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return false
	}
	auth := AuthFromContext(r.Context())
	access, err := a.resolveCourseAccess(r.Context(), tenantID, auth, courseID)
	if err != nil {
		a.logger.Error("resolve course access failed", "error", err, "courseId", courseID)
		writeErrorJSON(w, http.StatusInternalServerError, "access_check_failed",
			"Could not verify course access", r)
		return false
	}
	return enforceAccess(w, r, access, action, "course")
}

func (a *App) requireBlueprintAccess(
	w http.ResponseWriter, r *http.Request, templateID string, action AccessAction,
) bool {
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return false
	}
	auth := AuthFromContext(r.Context())
	access, err := a.resolveBlueprintAccess(r.Context(), tenantID, auth, templateID)
	if err != nil {
		a.logger.Error("resolve blueprint access failed", "error", err, "templateId", templateID)
		writeErrorJSON(w, http.StatusInternalServerError, "access_check_failed",
			"Could not verify blueprint access", r)
		return false
	}
	return enforceAccess(w, r, access, action, "blueprint")
}

// enforceAccess implements the 404-vs-403 split:
//
//   - No role at all → 404 (resource may exist in another tenant; do not leak).
//   - Has read but lacks the requested action → 403 (caller knows it
//     exists, surface the access shortfall so the AI bot can self-correct).
//   - Has the action → return true silently.
func enforceAccess(
	w http.ResponseWriter, r *http.Request,
	access ResourceAccess, action AccessAction, resourceLabel string,
) bool {
	if access.Allows(action) {
		return true
	}
	if access.CanRead {
		writeErrorJSON(w, http.StatusForbidden, "forbidden",
			fmt.Sprintf("You don't have %s access to this %s", action, resourceLabel), r)
		return false
	}
	writeErrorJSON(w, http.StatusNotFound, "not_found",
		fmt.Sprintf("%s not found", resourceLabel), r)
	return false
}

// canAccessExamReadBatch returns a map[examID]bool indicating which
// exams in `examIDs` the caller can read. Used by list endpoints to
// populate the `canAccess` field per ADR-0009 without doing N+1 round
// trips. Tenant admins always get all true.
func (a *App) canAccessExamReadBatch(
	ctx context.Context, tenantID string, auth *AuthContext, examIDs []string,
) map[string]bool {
	out := make(map[string]bool, len(examIDs))
	if len(examIDs) == 0 || auth == nil || auth.UserID == "" {
		return out
	}
	if isTenantAdmin(auth) {
		for _, id := range examIDs {
			out[id] = true
		}
		return out
	}
	// Owner OR collaborator OR subject fallback. Single query, OR'd.
	q := `
		WITH ids AS (SELECT unnest($3::uuid[]) AS exam_id)
		SELECT i.exam_id::text
		  FROM ids i
		  JOIN exams e ON e.id = i.exam_id AND e.tenant_id = $2
		 WHERE e.owner_user_id = $1
		    OR EXISTS (
		        SELECT 1 FROM exam_collaborators c
		         WHERE c.exam_id = e.id AND c.user_id = $1
		    )
		    OR (e.subject_id IS NOT NULL AND EXISTS (
		        SELECT 1 FROM teacher_subjects ts
		          JOIN teachers t ON t.id = ts.teacher_id
		         WHERE t.user_id = $1 AND ts.tenant_id = $2
		           AND ts.subject_id = e.subject_id
		           AND ts.status = 'active' AND t.status = 'active'
		    ))`
	rows, err := a.db.QueryContext(ctx, q, auth.UserID, tenantID, pgUUIDArray(examIDs))
	if err != nil {
		a.logger.Error("canAccessExamReadBatch query failed", "error", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			out[id] = true
		}
	}
	return out
}

// pgUUIDArray formats a Go []string as a Postgres uuid[] literal so it
// can be passed to `unnest($N::uuid[])`. We avoid pgx-specific array
// support to keep the database/sql interface minimal.
func pgUUIDArray(ids []string) string {
	if len(ids) == 0 {
		return "{}"
	}
	out := "{"
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	out += "}"
	return out
}
