package app

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

// Blueprint Templates — reusable kisi-kisi structures per ADR-0010.
//
// Templates live in a tenant-wide library and are cloned (snapshot)
// into individual exams via clone_blueprint_to_exam. Templates support
// the full ownership + collaborator model from ADR-0009.
//
// Lifecycle:
//   draft     — under construction
//   published — frozen for use; can still be cloned but not edited
//                without bumping `version` (advisory)
//   archived  — hidden from new selections but retained for audit and
//                so already-cloned exam blueprints keep their lineage

func (a *App) registerBlueprintTemplateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/blueprint-templates", a.handleListBlueprintTemplates)
	mux.HandleFunc("POST /api/v1/blueprint-templates", a.handleCreateBlueprintTemplate)
	mux.HandleFunc("GET /api/v1/blueprint-templates/{id}", a.handleGetBlueprintTemplate)
	mux.HandleFunc("PATCH /api/v1/blueprint-templates/{id}", a.handleUpdateBlueprintTemplate)
	mux.HandleFunc("PATCH /api/v1/blueprint-templates/{id}/publish", a.handlePublishBlueprintTemplate)
	mux.HandleFunc("PATCH /api/v1/blueprint-templates/{id}/unpublish", a.handleUnpublishBlueprintTemplate)
	mux.HandleFunc("PATCH /api/v1/blueprint-templates/{id}/archive", a.handleArchiveBlueprintTemplate)
	mux.HandleFunc("PATCH /api/v1/blueprint-templates/{id}/restore", a.handleRestoreBlueprintTemplate)
}

type blueprintTemplateRow struct {
	ID              string  `json:"id"`
	OwnerUserID     string  `json:"ownerUserId"`
	OwnerName       string  `json:"ownerName"`
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	CurriculumID    string  `json:"curriculumId"`
	CurriculumCode  string  `json:"curriculumCode"`
	CurriculumLabel string  `json:"competencyLabel"`
	SubjectCode     *string `json:"subjectCode,omitempty"`
	GradeOrPhase    *string `json:"gradeOrPhase,omitempty"`
	BlueprintType   string  `json:"blueprintType"`
	TotalSlots      int     `json:"totalSlots"`
	TotalPoints     float64 `json:"totalPoints"`
	StrictCoverage  bool    `json:"strictCoverage"`
	Status          string  `json:"status"`
	Version         int     `json:"version"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	CanAccess       bool    `json:"canAccess"`
}

func (a *App) handleListBlueprintTemplates(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	status := httpx.QueryString(r, "status", "")
	curriculum := httpx.QueryString(r, "curriculum", "")
	bptype := httpx.QueryString(r, "type", "")

	var (
		whereParts = []string{"t.tenant_id = $1"}
		args       = []any{tenantID}
		idx        = 1
	)
	add := func(clause string, val any) {
		idx++
		whereParts = append(whereParts, strings.Replace(clause, "$$", "$"+strconv.Itoa(idx), 1))
		args = append(args, val)
	}
	if status != "" {
		add("t.status = $$", status)
	}
	if curriculum != "" {
		add("c.code = $$", curriculum)
	}
	if bptype != "" {
		add("t.blueprint_type = $$", bptype)
	}
	if search != "" {
		add("t.title ILIKE $$", "%"+search+"%")
	}
	whereClause := " WHERE " + strings.Join(whereParts, " AND ")

	var total int
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM blueprint_templates t
		   JOIN curricula c ON c.id = t.curriculum_id`+whereClause, args...,
	).Scan(&total); err != nil {
		a.logger.Error("count blueprint templates failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load blueprint templates", r)
		return
	}

	idx++
	limitArg := "$" + strconv.Itoa(idx)
	idx++
	offsetArg := "$" + strconv.Itoa(idx)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), `
		SELECT t.id::text, t.owner_user_id::text, COALESCE(u.display_name, ''),
		       t.title, t.description,
		       t.curriculum_id::text, c.code, c.competency_label,
		       t.subject_code, t.grade_or_phase,
		       t.blueprint_type, t.total_slots, t.total_points, t.strict_coverage,
		       t.status, t.version,
		       t.created_at::text, t.updated_at::text
		  FROM blueprint_templates t
		  JOIN curricula c ON c.id = t.curriculum_id
		  LEFT JOIN users u ON u.id = t.owner_user_id`+whereClause+
			` ORDER BY t.updated_at DESC LIMIT `+limitArg+` OFFSET `+offsetArg, args...,
	)
	if err != nil {
		a.logger.Error("list blueprint templates failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load blueprint templates", r)
		return
	}
	defer rows.Close()

	out := make([]blueprintTemplateRow, 0)
	var ids []string
	for rows.Next() {
		var t blueprintTemplateRow
		if err := rows.Scan(&t.ID, &t.OwnerUserID, &t.OwnerName,
			&t.Title, &t.Description,
			&t.CurriculumID, &t.CurriculumCode, &t.CurriculumLabel,
			&t.SubjectCode, &t.GradeOrPhase,
			&t.BlueprintType, &t.TotalSlots, &t.TotalPoints, &t.StrictCoverage,
			&t.Status, &t.Version,
			&t.CreatedAt, &t.UpdatedAt); err == nil {
			out = append(out, t)
			ids = append(ids, t.ID)
		}
	}

	// Compute canAccess in batch (per ADR-0009 list-response contract)
	auth := AuthFromContext(r.Context())
	access := a.canAccessBlueprintReadBatch(r.Context(), tenantID, auth, ids)
	for i := range out {
		out[i].CanAccess = access[out[i].ID]
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(out, p, total))
}

// canAccessBlueprintReadBatch is the equivalent of the exam/course
// helpers, scoped to blueprint_templates. No subject fallback.
func (a *App) canAccessBlueprintReadBatch(
	ctx context.Context, tenantID string, auth *AuthContext, templateIDs []string,
) map[string]bool {
	out := make(map[string]bool, len(templateIDs))
	if len(templateIDs) == 0 || auth == nil || auth.UserID == "" {
		return out
	}
	if isTenantAdmin(auth) {
		for _, id := range templateIDs {
			out[id] = true
		}
		return out
	}
	rows, err := a.db.QueryContext(ctx, `
		WITH ids AS (SELECT unnest($3::uuid[]) AS template_id)
		SELECT i.template_id::text
		  FROM ids i
		  JOIN blueprint_templates t ON t.id = i.template_id AND t.tenant_id = $2
		 WHERE t.owner_user_id = $1
		    OR EXISTS (
		        SELECT 1 FROM blueprint_template_collaborators c
		         WHERE c.template_id = t.id AND c.user_id = $1
		    )`, auth.UserID, tenantID, pgUUIDArray(templateIDs))
	if err != nil {
		a.logger.Error("canAccessBlueprintReadBatch query failed", "error", err)
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

func (a *App) handleGetBlueprintTemplate(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:read") {
		return
	}
	id := r.PathValue("id")
	if !a.requireBlueprintAccess(w, r, id, ActionRead) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	var t blueprintTemplateRow
	err := a.db.QueryRowContext(r.Context(), `
		SELECT t.id::text, t.owner_user_id::text, COALESCE(u.display_name, ''),
		       t.title, t.description,
		       t.curriculum_id::text, c.code, c.competency_label,
		       t.subject_code, t.grade_or_phase,
		       t.blueprint_type, t.total_slots, t.total_points, t.strict_coverage,
		       t.status, t.version,
		       t.created_at::text, t.updated_at::text
		  FROM blueprint_templates t
		  JOIN curricula c ON c.id = t.curriculum_id
		  LEFT JOIN users u ON u.id = t.owner_user_id
		 WHERE t.id = $1 AND t.tenant_id = $2`,
		id, tenantID,
	).Scan(&t.ID, &t.OwnerUserID, &t.OwnerName,
		&t.Title, &t.Description,
		&t.CurriculumID, &t.CurriculumCode, &t.CurriculumLabel,
		&t.SubjectCode, &t.GradeOrPhase,
		&t.BlueprintType, &t.TotalSlots, &t.TotalPoints, &t.StrictCoverage,
		&t.Status, &t.Version,
		&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Blueprint template not found", r)
		return
	}
	t.CanAccess = true // already verified by requireBlueprintAccess
	writeJSON(w, http.StatusOK, t)
}

func (a *App) handleCreateBlueprintTemplate(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
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
		Title          string  `json:"title"`
		Description    string  `json:"description"`
		CurriculumCode string  `json:"curriculumCode"` // 'k13' | 'merdeka' | 'akm_*'
		SubjectCode    string  `json:"subjectCode"`
		GradeOrPhase   string  `json:"gradeOrPhase"`
		BlueprintType  string  `json:"blueprintType"`  // 'reguler' | 'akm_literasi' | 'akm_numerasi'
		StrictCoverage *bool   `json:"strictCoverage"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.CurriculumCode = strings.TrimSpace(req.CurriculumCode)
	if req.BlueprintType == "" {
		req.BlueprintType = "reguler"
	}

	fields := map[string]string{}
	if req.Title == "" {
		fields["title"] = "Title is required"
	}
	if req.CurriculumCode == "" {
		fields["curriculumCode"] = "curriculumCode is required (k13/merdeka/akm_numerasi/akm_literasi)"
	}
	if req.BlueprintType != "reguler" &&
		req.BlueprintType != "akm_literasi" &&
		req.BlueprintType != "akm_numerasi" {
		fields["blueprintType"] = "blueprintType must be reguler/akm_literasi/akm_numerasi"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// Resolve curriculum FK
	var curriculumID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT id::text FROM curricula WHERE code = $1`, req.CurriculumCode,
	).Scan(&curriculumID); err != nil {
		writeValidationError(w, map[string]string{
			"curriculumCode": "Unknown curriculum code",
		}, r)
		return
	}

	// AKM blueprints default to strict_coverage = true unless caller
	// explicitly says otherwise. Reguler defaults to false.
	strict := false
	if strings.HasPrefix(req.BlueprintType, "akm_") {
		strict = true
	}
	if req.StrictCoverage != nil {
		strict = *req.StrictCoverage
	}

	auth := AuthFromContext(r.Context())
	var id string
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO blueprint_templates (
		    tenant_id, owner_user_id, title, description,
		    curriculum_id, subject_code, grade_or_phase, blueprint_type,
		    strict_coverage, status, version
		) VALUES ($1, $2, $3, NULLIF($4,''),
		          $5, NULLIF($6,''), NULLIF($7,''), $8,
		          $9, 'draft', 1)
		RETURNING id::text`,
		tenantID, auth.UserID, req.Title, req.Description,
		curriculumID, strings.TrimSpace(req.SubjectCode), strings.TrimSpace(req.GradeOrPhase),
		req.BlueprintType, strict,
	).Scan(&id)
	if err != nil {
		a.logger.Error("create blueprint template failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
			"Could not create blueprint template", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates.create", "blueprint_template", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "draft"})
}

func (a *App) handleUpdateBlueprintTemplate(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	id := r.PathValue("id")
	if !a.requireBlueprintAccess(w, r, id, ActionWrite) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	var req struct {
		Title          *string `json:"title"`
		Description    *string `json:"description"`
		SubjectCode    *string `json:"subjectCode"`
		GradeOrPhase   *string `json:"gradeOrPhase"`
		StrictCoverage *bool   `json:"strictCoverage"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	parts := []string{"updated_at = now()", "version = version + 1"}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}
	if req.Title != nil {
		add("title", *req.Title)
	}
	if req.Description != nil {
		if *req.Description == "" {
			parts = append(parts, "description = NULL")
		} else {
			add("description", *req.Description)
		}
	}
	if req.SubjectCode != nil {
		if *req.SubjectCode == "" {
			parts = append(parts, "subject_code = NULL")
		} else {
			add("subject_code", *req.SubjectCode)
		}
	}
	if req.GradeOrPhase != nil {
		if *req.GradeOrPhase == "" {
			parts = append(parts, "grade_or_phase = NULL")
		} else {
			add("grade_or_phase", *req.GradeOrPhase)
		}
	}
	if req.StrictCoverage != nil {
		add("strict_coverage", *req.StrictCoverage)
	}
	if len(args) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "no_change"})
		return
	}
	q := "UPDATE blueprint_templates SET " + strings.Join(parts, ", ") +
		" WHERE id = $" + strconv.Itoa(idx) + " AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, id, tenantID)
	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed",
			"Could not update blueprint template", r)
		return
	}
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates.update", "blueprint_template", id, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "updated"})
}

func (a *App) handlePublishBlueprintTemplate(w http.ResponseWriter, r *http.Request) {
	a.transitionBlueprintTemplate(w, r, "published",
		"draft", "Template is not in draft status", "publish")
}

// handleUnpublishBlueprintTemplate returns a published template back
// to draft so its slots / metadata can be edited again. Per user
// feedback (Phase 9.10): once published, downstream changes are
// disallowed by the standards UI; teachers still need to fix typos
// or refine indikator. Reverting to draft is allowed only when the
// template is currently published — archived templates must be
// restored first. Existing exam_blueprints already cloned from this
// template are unaffected (they are independent snapshots).
func (a *App) handleUnpublishBlueprintTemplate(w http.ResponseWriter, r *http.Request) {
	a.transitionBlueprintTemplate(w, r, "draft",
		"published", "Template is not in published status", "unpublish")
}

func (a *App) handleArchiveBlueprintTemplate(w http.ResponseWriter, r *http.Request) {
	a.transitionBlueprintTemplate(w, r, "archived",
		"", "", "archive")
}

func (a *App) handleRestoreBlueprintTemplate(w http.ResponseWriter, r *http.Request) {
	a.transitionBlueprintTemplate(w, r, "draft",
		"archived", "Template is not archived", "restore")
}

// transitionBlueprintTemplate is a small DRY helper for status-change
// endpoints. When `requiredFrom` is empty, any current status is accepted.
func (a *App) transitionBlueprintTemplate(
	w http.ResponseWriter, r *http.Request,
	to, requiredFrom, conflictMsg, action string,
) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	id := r.PathValue("id")
	// archive/publish are delete-class; restore is write-class. Use
	// ActionDelete for archive/publish so only owner+admin can do it,
	// matching the exam pattern.
	wantedAction := ActionDelete
	if action == "restore" {
		wantedAction = ActionWrite
	}
	if !a.requireBlueprintAccess(w, r, id, wantedAction) {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)

	q := `UPDATE blueprint_templates SET status = $1, updated_at = now()
		   WHERE id = $2 AND tenant_id = $3`
	args := []any{to, id, tenantID}
	if requiredFrom != "" {
		q += ` AND status = $4`
		args = append(args, requiredFrom)
	}
	res, err := a.db.ExecContext(r.Context(), q, args...)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, action+"_failed",
			"Could not "+action+" template", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if conflictMsg != "" {
			writeErrorJSON(w, http.StatusConflict, "invalid_state", conflictMsg, r)
		} else {
			writeErrorJSON(w, http.StatusNotFound, "not_found", "Template not found", r)
		}
		return
	}
	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID,
		"blueprint_templates."+action, "blueprint_template", id, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": to})
}
