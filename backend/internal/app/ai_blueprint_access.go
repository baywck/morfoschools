package app

import (
	"context"
)

// AI access helpers for blueprint/stimulus capabilities.
//
// AI capability handlers receive (ctx, tenantID, userID, args) — they
// don't have an *http.Request and therefore can't reuse the HTTP-bound
// requireExamAccess helpers that write a 4xx response. Instead we
// construct an AuthContext from the userID + tenant, run the same
// resolveExamAccess/resolveBlueprintAccess logic, and emit a structured
// ToolError JSON via the existing ai_errors.go envelope when access
// fails. This mirrors the Phase 9 fix in ai_exam_tools.go
// (checkExamWriteAccess) and extends the pattern to layered access.
//
// Returning "" means access granted; otherwise the returned string is a
// pre-serialized ToolError JSON the cap handler should pass straight
// back to the LLM so it can self-correct.

// loadAIAuth builds an AuthContext for an AI executor by loading the
// caller's roles + permissions for the active tenant. We use this to
// drive the same resolveExamAccess / resolveBlueprintAccess functions
// the HTTP handlers use, keeping the access matrix in one place.
func (a *App) loadAIAuth(ctx context.Context, tenantID, userID string) *AuthContext {
	if userID == "" {
		return nil
	}
	auth := &AuthContext{UserID: userID}
	if tenantID != "" {
		t := tenantID
		auth.EffectiveTenantID = &t
	}
	roles, perms, err := a.loadRolesAndPermissions(ctx, userID, auth.EffectiveTenantID)
	if err == nil {
		auth.Roles = roles
		auth.Permissions = perms
	}
	return auth
}

// checkAIExamAccess returns "" when the AI caller has the requested
// action on the given exam. On rejection it returns a serialized
// ToolError so the bot can self-correct.
//
// 404 (ENTITY_NOT_FOUND) when the resource is missing from the tenant
// or the caller has no read access. 403 (PERMISSION_DENIED) when the
// caller can read but lacks the requested action. The split mirrors
// enforceAccess() so the bot's recovery hint is meaningful.
func (a *App) checkAIExamAccess(
	ctx context.Context, tenantID, userID, examID string, action AccessAction,
) string {
	if examID == "" {
		return errValidationFailed("examId", "examId is required")
	}
	auth := a.loadAIAuth(ctx, tenantID, userID)
	if auth == nil {
		return errPermissionDenied(string(action) + " exam")
	}
	access, err := a.resolveExamAccess(ctx, tenantID, auth, examID)
	if err != nil {
		return errInternal("Could not verify exam access")
	}
	if access.Allows(action) {
		return ""
	}
	if access.CanRead {
		return errPermissionDenied(string(action) + " this exam")
	}
	return errEntityNotFound("exam", "examId", examID)
}

// checkAIBlueprintAccess is the template-side analog. Operates on
// blueprint_templates only — exam_blueprints inherit access from their
// parent exam (use checkAIExamAccess in that path).
func (a *App) checkAIBlueprintAccess(
	ctx context.Context, tenantID, userID, templateID string, action AccessAction,
) string {
	if templateID == "" {
		return errValidationFailed("templateId", "templateId is required")
	}
	auth := a.loadAIAuth(ctx, tenantID, userID)
	if auth == nil {
		return errPermissionDenied(string(action) + " blueprint template")
	}
	access, err := a.resolveBlueprintAccess(ctx, tenantID, auth, templateID)
	if err != nil {
		return errInternal("Could not verify blueprint template access")
	}
	if access.Allows(action) {
		return ""
	}
	if access.CanRead {
		return errPermissionDenied(string(action) + " this blueprint template")
	}
	return errEntityNotFound("blueprint_template", "templateId", templateID)
}

// resolveSlotParentExam returns the exam_id that owns a given
// exam_blueprint_slot in the active tenant. Returns "" when the slot
// is missing or cross-tenant. AI cap handlers use this before checking
// access on the parent exam.
func (a *App) resolveSlotParentExam(ctx context.Context, tenantID, slotID string) string {
	var examID string
	err := a.db.QueryRowContext(ctx, `
		SELECT b.exam_id::text
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE s.id = $1 AND b.tenant_id = $2`,
		slotID, tenantID,
	).Scan(&examID)
	if err != nil {
		return ""
	}
	return examID
}
