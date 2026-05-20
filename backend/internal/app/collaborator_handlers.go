package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// HTTP handlers for collaborator management. One generic implementation
// per verb (invite, list, change role, remove, transfer ownership)
// reused across exams, courses, and blueprint_templates via small
// adapter functions that supply the resource-specific table names and
// access resolver.
//
// Routes registered by registerCollaboratorRoutes() below; that
// function is called from app.go alongside the existing module
// registrations.

func (a *App) registerCollaboratorRoutes(mux *http.ServeMux) {
	// Exams
	mux.HandleFunc("GET /api/v1/exams/{id}/collaborators",
		a.handleListExamCollaborators)
	mux.HandleFunc("POST /api/v1/exams/{id}/collaborators",
		a.handleInviteExamCollaborator)
	mux.HandleFunc("PATCH /api/v1/exam-collaborators/{collabId}",
		a.handleUpdateExamCollaborator)
	mux.HandleFunc("DELETE /api/v1/exam-collaborators/{collabId}",
		a.handleRemoveExamCollaborator)
	mux.HandleFunc("PATCH /api/v1/exams/{id}/transfer-ownership",
		a.handleTransferExamOwnership)

	// Courses
	mux.HandleFunc("GET /api/v1/courses/{id}/collaborators",
		a.handleListCourseCollaborators)
	mux.HandleFunc("POST /api/v1/courses/{id}/collaborators",
		a.handleInviteCourseCollaborator)
	mux.HandleFunc("PATCH /api/v1/course-collaborators/{collabId}",
		a.handleUpdateCourseCollaborator)
	mux.HandleFunc("DELETE /api/v1/course-collaborators/{collabId}",
		a.handleRemoveCourseCollaborator)
	mux.HandleFunc("PATCH /api/v1/courses/{id}/transfer-ownership",
		a.handleTransferCourseOwnership)

	// Blueprint templates
	mux.HandleFunc("GET /api/v1/blueprint-templates/{id}/collaborators",
		a.handleListBlueprintCollaborators)
	mux.HandleFunc("POST /api/v1/blueprint-templates/{id}/collaborators",
		a.handleInviteBlueprintCollaborator)
	mux.HandleFunc("PATCH /api/v1/blueprint-template-collaborators/{collabId}",
		a.handleUpdateBlueprintCollaborator)
	mux.HandleFunc("DELETE /api/v1/blueprint-template-collaborators/{collabId}",
		a.handleRemoveBlueprintCollaborator)
	mux.HandleFunc("PATCH /api/v1/blueprint-templates/{id}/transfer-ownership",
		a.handleTransferBlueprintOwnership)
}

// resourceSpec captures the bits that differ between exam / course /
// blueprint collaboration. Everything else is shared.
type resourceSpec struct {
	label             string // user-facing noun
	parentTable       string // exams, courses, blueprint_templates
	collabTable       string // exam_collaborators, ...
	parentFKColumn    string // exam_id, course_id, template_id
	auditPrefix       string // "exams", "courses", "blueprint_templates"
	requireAccess     func(w http.ResponseWriter, r *http.Request, parentID string, action AccessAction) bool
	resolveAccessByCollab func(ctx context.Context, collabID string) (parentID string, tenantID string, err error)
}

func (a *App) examSpec() resourceSpec {
	return resourceSpec{
		label:          "exam",
		parentTable:    "exams",
		collabTable:    "exam_collaborators",
		parentFKColumn: "exam_id",
		auditPrefix:    "exams",
		requireAccess:  a.requireExamAccess,
		resolveAccessByCollab: func(ctx context.Context, collabID string) (string, string, error) {
			var pid, tid string
			err := a.db.QueryRowContext(ctx,
				`SELECT exam_id::text, tenant_id::text FROM exam_collaborators WHERE id = $1`,
				collabID,
			).Scan(&pid, &tid)
			return pid, tid, err
		},
	}
}

func (a *App) courseSpec() resourceSpec {
	return resourceSpec{
		label:          "course",
		parentTable:    "courses",
		collabTable:    "course_collaborators",
		parentFKColumn: "course_id",
		auditPrefix:    "courses",
		requireAccess:  a.requireCourseAccess,
		resolveAccessByCollab: func(ctx context.Context, collabID string) (string, string, error) {
			var pid, tid string
			err := a.db.QueryRowContext(ctx,
				`SELECT course_id::text, tenant_id::text FROM course_collaborators WHERE id = $1`,
				collabID,
			).Scan(&pid, &tid)
			return pid, tid, err
		},
	}
}

func (a *App) blueprintSpec() resourceSpec {
	return resourceSpec{
		label:          "blueprint template",
		parentTable:    "blueprint_templates",
		collabTable:    "blueprint_template_collaborators",
		parentFKColumn: "template_id",
		auditPrefix:    "blueprint_templates",
		requireAccess:  a.requireBlueprintAccess,
		resolveAccessByCollab: func(ctx context.Context, collabID string) (string, string, error) {
			var pid, tid string
			err := a.db.QueryRowContext(ctx,
				`SELECT template_id::text, tenant_id::text FROM blueprint_template_collaborators WHERE id = $1`,
				collabID,
			).Scan(&pid, &tid)
			return pid, tid, err
		},
	}
}

// --- Generic implementations ---

type collabRow struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	InvitedBy   string `json:"invitedBy,omitempty"`
	InvitedAt   string `json:"invitedAt"`
}

func (a *App) listCollaborators(w http.ResponseWriter, r *http.Request, spec resourceSpec) {
	parentID := r.PathValue("id")
	if parentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Resource ID is required", r)
		return
	}
	if !spec.requireAccess(w, r, parentID, ActionRead) {
		return
	}

	// Fetch owner snapshot first so the UI can render the dedicated
	// "Owner" row distinct from collaborators. parentTable always has
	// owner_user_id (Phase 9.5 ownership model). Best-effort — a NULL
	// owner_user_id is rare but should not fail the list.
	type ownerRow struct {
		ID, UserID, DisplayName, Email string
	}
	var (
		owner          *ownerRow
		ownerNullID    sql.NullString
		ownerName      sql.NullString
		ownerEmail     sql.NullString
	)
	ownerQuery := fmt.Sprintf(`
		SELECT u.id::text, u.display_name, u.email
		  FROM %s p
		  JOIN users u ON u.id = p.owner_user_id
		 WHERE p.id = $1`, spec.parentTable)
	if err := a.db.QueryRowContext(r.Context(), ownerQuery, parentID).
		Scan(&ownerNullID, &ownerName, &ownerEmail); err == nil {
		owner = &ownerRow{
			ID:          ownerNullID.String,
			UserID:      ownerNullID.String,
			DisplayName: ownerName.String,
			Email:       ownerEmail.String,
		}
	}

	q := fmt.Sprintf(`
		SELECT c.id::text, c.user_id::text, u.display_name, u.email, c.role,
		       COALESCE(c.invited_by::text, ''), c.invited_at::text
		  FROM %s c
		  JOIN users u ON u.id = c.user_id
		 WHERE c.%s = $1
		 ORDER BY c.invited_at ASC`,
		spec.collabTable, spec.parentFKColumn,
	)
	rows, err := a.db.QueryContext(r.Context(), q, parentID)
	if err != nil {
		a.logger.Error("list collaborators failed", "error", err, "label", spec.label)
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load collaborators", r)
		return
	}
	defer rows.Close()
	collabs := make([]collabRow, 0)
	for rows.Next() {
		var c collabRow
		if err := rows.Scan(&c.ID, &c.UserID, &c.DisplayName, &c.Email, &c.Role, &c.InvitedBy, &c.InvitedAt); err == nil {
			collabs = append(collabs, c)
		}
	}

	// Frontend ShareDialog reads { owner, collaborators } directly. We
	// also keep `data` (legacy) so older callers keep working until
	// they migrate.
	var ownerJSON any
	if owner != nil {
		ownerJSON = map[string]any{
			"id":          owner.ID,
			"userId":      owner.UserID,
			"displayName": owner.DisplayName,
			"email":       owner.Email,
			"role":        "owner",
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"owner":         ownerJSON,
		"collaborators": collabs,
		"data":          collabs,
	})
}

func (a *App) inviteCollaborator(w http.ResponseWriter, r *http.Request, spec resourceSpec) {
	parentID := r.PathValue("id")
	if parentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Resource ID is required", r)
		return
	}
	if !spec.requireAccess(w, r, parentID, ActionManage) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.Role = strings.TrimSpace(req.Role)
	fields := map[string]string{}
	if req.UserID == "" {
		fields["userId"] = "userId is required"
	}
	if req.Role != "editor" && req.Role != "viewer" {
		fields["role"] = "role must be 'editor' or 'viewer'"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// Verify the invitee belongs to the same tenant
	var inTenant bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2 AND status = 'active')`,
		tenantID, req.UserID,
	).Scan(&inTenant)
	if !inTenant {
		writeValidationError(w, map[string]string{
			"userId": "User is not a member of this tenant",
		}, r)
		return
	}

	// Cannot invite the owner — they already have full access
	var ownerID string
	_ = a.db.QueryRowContext(r.Context(),
		fmt.Sprintf(`SELECT owner_user_id::text FROM %s WHERE id = $1`, spec.parentTable),
		parentID,
	).Scan(&ownerID)
	if ownerID == req.UserID {
		writeValidationError(w, map[string]string{
			"userId": "User is already the owner of this resource",
		}, r)
		return
	}

	auth := AuthFromContext(r.Context())
	q := fmt.Sprintf(`
		INSERT INTO %s (tenant_id, %s, user_id, role, invited_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (%s, user_id) DO UPDATE SET role = EXCLUDED.role
		RETURNING id::text`,
		spec.collabTable, spec.parentFKColumn, spec.parentFKColumn,
	)
	var id string
	err := a.db.QueryRowContext(r.Context(), q, tenantID, parentID, req.UserID, req.Role, auth.UserID).Scan(&id)
	if err != nil {
		a.logger.Error("invite collaborator failed", "error", err, "label", spec.label)
		writeErrorJSON(w, http.StatusInternalServerError, "invite_failed",
			"Could not invite collaborator", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID,
		spec.auditPrefix+".collaborator_invited", spec.label, parentID, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "role": req.Role})
}

func (a *App) updateCollaborator(w http.ResponseWriter, r *http.Request, spec resourceSpec) {
	collabID := r.PathValue("collabId")
	if collabID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Collaborator ID is required", r)
		return
	}
	parentID, _, err := spec.resolveAccessByCollab(r.Context(), collabID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Collaborator not found", r)
		return
	}
	// Updating a role is a manage action
	if !spec.requireAccess(w, r, parentID, ActionManage) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	if req.Role != "editor" && req.Role != "viewer" {
		writeValidationError(w, map[string]string{"role": "role must be 'editor' or 'viewer'"}, r)
		return
	}
	q := fmt.Sprintf(`UPDATE %s SET role = $1 WHERE id = $2`, spec.collabTable)
	if _, err := a.db.ExecContext(r.Context(), q, req.Role, collabID); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed",
			"Could not update collaborator", r)
		return
	}

	auth := AuthFromContext(r.Context())
	tenantID := a.RequireEffectiveTenant(w, r)
	a.audit(r.Context(), &tenantID, auth.UserID,
		spec.auditPrefix+".collaborator_role_changed", spec.label, parentID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": collabID, "role": req.Role})
}

// removeCollaborator handles two distinct cases:
//
//  1. Manage actor removes someone (anyone but the owner)
//  2. The collaborator removes themselves ("leave"). This is allowed
//     even without manage permission — leaving is always self-permitted.
//
// Owner cannot be removed via this path; they must transfer ownership
// first. There's no row in the collaborators table for the owner, so
// this case never arises naturally.
func (a *App) removeCollaborator(w http.ResponseWriter, r *http.Request, spec resourceSpec) {
	collabID := r.PathValue("collabId")
	if collabID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Collaborator ID is required", r)
		return
	}
	parentID, tenantID, err := spec.resolveAccessByCollab(r.Context(), collabID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Collaborator not found", r)
		return
	}

	auth := AuthFromContext(r.Context())
	// Determine if this is a self-leave or an admin removal
	var collabUserID string
	_ = a.db.QueryRowContext(r.Context(),
		fmt.Sprintf(`SELECT user_id::text FROM %s WHERE id = $1`, spec.collabTable),
		collabID,
	).Scan(&collabUserID)

	isSelfLeave := collabUserID == auth.UserID
	if !isSelfLeave {
		if !spec.requireAccess(w, r, parentID, ActionManage) {
			return
		}
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	q := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, spec.collabTable)
	res, err := a.db.ExecContext(r.Context(), q, collabID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "remove_failed",
			"Could not remove collaborator", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Collaborator not found", r)
		return
	}

	auditAction := spec.auditPrefix + ".collaborator_removed"
	if isSelfLeave {
		auditAction = spec.auditPrefix + ".collaborator_left"
	}
	a.audit(r.Context(), &tenantID, auth.UserID, auditAction, spec.label, parentID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": collabID, "status": "removed"})
}

// transferOwnership atomically reassigns ownership to a new user. The
// old owner is automatically demoted to `editor` so they don't lose
// access entirely (per ADR-0009 anti-self-lockout). Tenant admin can
// also force-transfer (e.g. teacher resignation).
//
// In one transaction:
//  1. Verify caller is current owner OR tenant admin
//  2. Verify newOwner is in the same tenant
//  3. UPDATE parent_table SET owner_user_id = newOwner
//  4. DELETE FROM collab_table WHERE user_id = newOwner  (was collaborator → no longer needed)
//  5. INSERT INTO collab_table for old owner as editor (skip if old owner == new owner)
//  6. Audit
func (a *App) transferOwnership(w http.ResponseWriter, r *http.Request, spec resourceSpec) {
	parentID := r.PathValue("id")
	if parentID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Resource ID is required", r)
		return
	}
	// We use ActionManage since only the owner (or admin) can transfer.
	// The require* helper already enforces this.
	if !spec.requireAccess(w, r, parentID, ActionManage) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	var req struct {
		NewOwnerID string `json:"newOwnerId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	req.NewOwnerID = strings.TrimSpace(req.NewOwnerID)
	if req.NewOwnerID == "" {
		writeValidationError(w, map[string]string{"newOwnerId": "newOwnerId is required"}, r)
		return
	}

	// New owner must be in the same tenant
	var inTenant bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2 AND status = 'active')`,
		tenantID, req.NewOwnerID,
	).Scan(&inTenant)
	if !inTenant {
		writeValidationError(w, map[string]string{
			"newOwnerId": "New owner is not a member of this tenant",
		}, r)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "transfer_failed",
			"Could not start transaction", r)
		return
	}
	defer tx.Rollback()

	var oldOwnerID string
	if err := tx.QueryRowContext(r.Context(),
		fmt.Sprintf(`SELECT owner_user_id::text FROM %s WHERE id = $1 FOR UPDATE`, spec.parentTable),
		parentID,
	).Scan(&oldOwnerID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("%s not found", spec.label), r)
		return
	}

	if oldOwnerID == req.NewOwnerID {
		// No-op transfer. Don't error — return idempotent success.
		writeJSON(w, http.StatusOK, map[string]any{
			"id": parentID, "ownerId": oldOwnerID, "status": "no_change",
		})
		return
	}

	// Set new owner
	if _, err := tx.ExecContext(r.Context(),
		fmt.Sprintf(`UPDATE %s SET owner_user_id = $1, updated_at = now() WHERE id = $2`, spec.parentTable),
		req.NewOwnerID, parentID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "transfer_failed",
			"Could not update owner", r)
		return
	}

	// Remove new owner's collaborator row if present (they'd have a
	// strictly weaker role row that's now obsolete)
	if _, err := tx.ExecContext(r.Context(),
		fmt.Sprintf(`DELETE FROM %s WHERE %s = $1 AND user_id = $2`, spec.collabTable, spec.parentFKColumn),
		parentID, req.NewOwnerID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "transfer_failed",
			"Could not clean new-owner collaborator row", r)
		return
	}

	// Demote old owner to editor (anti-self-lockout)
	auth := AuthFromContext(r.Context())
	if _, err := tx.ExecContext(r.Context(),
		fmt.Sprintf(`
			INSERT INTO %s (tenant_id, %s, user_id, role, invited_by)
			VALUES ($1, $2, $3, 'editor', $4)
			ON CONFLICT (%s, user_id) DO UPDATE SET role = 'editor'`,
			spec.collabTable, spec.parentFKColumn, spec.parentFKColumn),
		tenantID, parentID, oldOwnerID, auth.UserID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "transfer_failed",
			"Could not demote old owner", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "transfer_failed",
			"Could not finalize transfer", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID,
		spec.auditPrefix+".ownership_transferred", spec.label, parentID, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      parentID,
		"ownerId": req.NewOwnerID,
		"status":  "transferred",
	})
}

// --- Adapter handlers per resource type (thin wrappers) ---

func (a *App) handleListExamCollaborators(w http.ResponseWriter, r *http.Request) {
	a.listCollaborators(w, r, a.examSpec())
}
func (a *App) handleInviteExamCollaborator(w http.ResponseWriter, r *http.Request) {
	a.inviteCollaborator(w, r, a.examSpec())
}
func (a *App) handleUpdateExamCollaborator(w http.ResponseWriter, r *http.Request) {
	a.updateCollaborator(w, r, a.examSpec())
}
func (a *App) handleRemoveExamCollaborator(w http.ResponseWriter, r *http.Request) {
	a.removeCollaborator(w, r, a.examSpec())
}
func (a *App) handleTransferExamOwnership(w http.ResponseWriter, r *http.Request) {
	a.transferOwnership(w, r, a.examSpec())
}

func (a *App) handleListCourseCollaborators(w http.ResponseWriter, r *http.Request) {
	a.listCollaborators(w, r, a.courseSpec())
}
func (a *App) handleInviteCourseCollaborator(w http.ResponseWriter, r *http.Request) {
	a.inviteCollaborator(w, r, a.courseSpec())
}
func (a *App) handleUpdateCourseCollaborator(w http.ResponseWriter, r *http.Request) {
	a.updateCollaborator(w, r, a.courseSpec())
}
func (a *App) handleRemoveCourseCollaborator(w http.ResponseWriter, r *http.Request) {
	a.removeCollaborator(w, r, a.courseSpec())
}
func (a *App) handleTransferCourseOwnership(w http.ResponseWriter, r *http.Request) {
	a.transferOwnership(w, r, a.courseSpec())
}

func (a *App) handleListBlueprintCollaborators(w http.ResponseWriter, r *http.Request) {
	a.listCollaborators(w, r, a.blueprintSpec())
}
func (a *App) handleInviteBlueprintCollaborator(w http.ResponseWriter, r *http.Request) {
	a.inviteCollaborator(w, r, a.blueprintSpec())
}
func (a *App) handleUpdateBlueprintCollaborator(w http.ResponseWriter, r *http.Request) {
	a.updateCollaborator(w, r, a.blueprintSpec())
}
func (a *App) handleRemoveBlueprintCollaborator(w http.ResponseWriter, r *http.Request) {
	a.removeCollaborator(w, r, a.blueprintSpec())
}
func (a *App) handleTransferBlueprintOwnership(w http.ResponseWriter, r *http.Request) {
	a.transferOwnership(w, r, a.blueprintSpec())
}

// silence unused import when the file is evaluated under different build
// tags; keeps lints quiet without forcing a blank import elsewhere.
var _ = errors.Is
var _ = sql.ErrNoRows
