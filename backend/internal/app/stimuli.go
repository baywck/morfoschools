package app

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

// Stimuli — text-only library entries that can be attached to exam
// question groups (typically AKM literasi/numerasi). Per ADR-0010.
//
// Lifecycle:
//   exam_scoped — created inline in question form, only visible inside
//                 its parent exam. Default for inline-created stimuli
//                 to avoid library pollution.
//   shared      — promoted to library, visible across the tenant.
//   archived    — hidden from new selections but kept for audit and
//                 to preserve the snapshots in exams that already use it.
//
// Permission model:
//   - Anyone with exams:write can read SHARED stimuli to attach them.
//   - exam_scoped stimuli are visible only when fetched in the context
//     of their parent exam (UI must filter by parent_exam_id).
//   - Only the owner OR a tenant admin can edit/archive a stimulus.
//
// We treat stimuli as creator-owned for delete purposes (no
// collaborator junction table for stimuli — too granular, and the
// snapshot semantics protect exams from rogue edits anyway).

func (a *App) registerStimuliRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/stimuli", a.handleListStimuli)
	mux.HandleFunc("POST /api/v1/stimuli", a.handleCreateStimulus)
	mux.HandleFunc("GET /api/v1/stimuli/{id}", a.handleGetStimulus)
	mux.HandleFunc("PATCH /api/v1/stimuli/{id}", a.handleUpdateStimulus)
	mux.HandleFunc("PATCH /api/v1/stimuli/{id}/archive", a.handleArchiveStimulus)
	mux.HandleFunc("PATCH /api/v1/stimuli/{id}/promote", a.handlePromoteStimulus)
	mux.HandleFunc("POST /api/v1/stimuli/{id}/sync-snapshot", a.handleSyncStimulusSnapshot)
}

type stimulusRow struct {
	ID           string  `json:"id"`
	OwnerUserID  string  `json:"ownerUserId"`
	OwnerName    string  `json:"ownerName"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Content      string  `json:"content"`
	Source       *string `json:"source,omitempty"`
	Lifecycle    string  `json:"lifecycle"`
	ParentExamID *string `json:"parentExamId,omitempty"`
	UsageCount   int     `json:"usageCount"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

func (a *App) handleListStimuli(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	auth := AuthFromContext(r.Context())

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	lifecycle := httpx.QueryString(r, "lifecycle", "shared")
	// Default to "shared" so the library page does not return private
	// exam_scoped rows. Privileged values:
	//   lifecycle=all          — tenant admins only
	//   lifecycle=exam_scoped  — must include parentExamId AND have
	//                            read access to that parent exam
	//   lifecycle=archived     — owner-or-admin readable; non-owners get
	//                            an empty list rather than other tenants'
	//                            archived stimuli
	parentExamID := httpx.QueryString(r, "parentExamId", "")

	switch lifecycle {
	case "all":
		if !isTenantAdmin(auth) {
			writeErrorJSON(w, http.StatusForbidden, "forbidden",
				"lifecycle=all is restricted to tenant admins", r)
			return
		}
	case "exam_scoped":
		if parentExamID == "" {
			writeErrorJSON(w, http.StatusBadRequest, "invalid_request",
				"lifecycle=exam_scoped requires parentExamId", r)
			return
		}
		if !a.requireExamAccess(w, r, parentExamID, ActionRead) {
			return
		}
	case "archived":
		// Allowed only to tenant admins; otherwise users could enumerate
		// other teachers' archived stimuli. Owners can still see their
		// own archives via the dedicated owner filter (future).
		if !isTenantAdmin(auth) {
			writeErrorJSON(w, http.StatusForbidden, "forbidden",
				"lifecycle=archived is restricted to tenant admins", r)
			return
		}
	case "shared", "":
		lifecycle = "shared"
	default:
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request",
			"lifecycle must be shared/exam_scoped/archived/all", r)
		return
	}

	var (
		whereParts []string
		args       []any
		idx        int
	)
	add := func(clause string, val any) {
		idx++
		whereParts = append(whereParts, strings.Replace(clause, "$$", "$"+strconv.Itoa(idx), 1))
		args = append(args, val)
	}
	add("tenant_id = $$", tenantID)
	switch lifecycle {
	case "all":
		// no filter beyond tenant_id
	default:
		add("lifecycle = $$", lifecycle)
	}
	if parentExamID != "" {
		add("parent_exam_id = $$", parentExamID)
	}
	if search != "" {
		add("title ILIKE $$", "%"+search+"%")
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = " WHERE " + strings.Join(whereParts, " AND ")
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM stimuli"+whereClause, args...,
	).Scan(&total); err != nil {
		a.logger.Error("count stimuli failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load stimuli", r)
		return
	}

	idx++
	limitArg := "$" + strconv.Itoa(idx)
	idx++
	offsetArg := "$" + strconv.Itoa(idx)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT s.id::text, s.owner_user_id::text, COALESCE(u.display_name, ''),
		        s.type, s.title, s.content, s.source, s.lifecycle,
		        COALESCE(s.parent_exam_id::text, ''), s.usage_count,
		        s.created_at::text, s.updated_at::text
		   FROM stimuli s
		   LEFT JOIN users u ON u.id = s.owner_user_id`+whereClause+
			` ORDER BY s.updated_at DESC LIMIT `+limitArg+` OFFSET `+offsetArg, args...,
	)
	if err != nil {
		a.logger.Error("list stimuli failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed",
			"Could not load stimuli", r)
		return
	}
	defer rows.Close()

	out := make([]stimulusRow, 0)
	for rows.Next() {
		var s stimulusRow
		var parentRaw string
		if err := rows.Scan(&s.ID, &s.OwnerUserID, &s.OwnerName,
			&s.Type, &s.Title, &s.Content, &s.Source, &s.Lifecycle,
			&parentRaw, &s.UsageCount, &s.CreatedAt, &s.UpdatedAt); err == nil {
			if parentRaw != "" {
				p := parentRaw
				s.ParentExamID = &p
			}
			out = append(out, s)
		}
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(out, p, total))
}

func (a *App) handleGetStimulus(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	id := r.PathValue("id")
	auth := AuthFromContext(r.Context())

	var s stimulusRow
	var parentRaw string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT s.id::text, s.owner_user_id::text, COALESCE(u.display_name, ''),
		       s.type, s.title, s.content, s.source, s.lifecycle,
		       COALESCE(s.parent_exam_id::text, ''), s.usage_count,
		       s.created_at::text, s.updated_at::text
		  FROM stimuli s
		  LEFT JOIN users u ON u.id = s.owner_user_id
		 WHERE s.id = $1 AND s.tenant_id = $2`,
		id, tenantID,
	).Scan(&s.ID, &s.OwnerUserID, &s.OwnerName,
		&s.Type, &s.Title, &s.Content, &s.Source, &s.Lifecycle,
		&parentRaw, &s.UsageCount, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Stimulus not found", r)
		return
	}

	// Lifecycle-aware privacy gate. Shared library entries are visible
	// to anyone with exams:read in the tenant. exam_scoped requires read
	// access on the parent exam (otherwise the per-exam private content
	// would leak via direct ID lookups). Archived is owner-or-admin only.
	switch s.Lifecycle {
	case "shared":
		// allowed
	case "exam_scoped":
		if parentRaw == "" {
			// Defensive: exam_scoped without parent should never happen,
			// but if data drift produced it, deny rather than leak.
			writeErrorJSON(w, http.StatusNotFound, "not_found", "Stimulus not found", r)
			return
		}
		if !a.requireExamAccess(w, r, parentRaw, ActionRead) {
			return
		}
	case "archived":
		if !isTenantAdmin(auth) && (auth == nil || auth.UserID != s.OwnerUserID) {
			writeErrorJSON(w, http.StatusNotFound, "not_found", "Stimulus not found", r)
			return
		}
	}
	if parentRaw != "" {
		s.ParentExamID = &parentRaw
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *App) handleCreateStimulus(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
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
		Title        string  `json:"title"`
		Content      string  `json:"content"`
		Source       string  `json:"source"`
		Lifecycle    string  `json:"lifecycle"`    // "exam_scoped" | "shared"
		ParentExamID *string `json:"parentExamId"` // required when lifecycle=exam_scoped
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	req.Lifecycle = strings.TrimSpace(req.Lifecycle)

	fields := map[string]string{}
	if req.Title == "" {
		fields["title"] = "Title is required"
	}
	if req.Content == "" {
		fields["content"] = "Content is required"
	}
	if req.Lifecycle == "" {
		req.Lifecycle = "exam_scoped"
	}
	if req.Lifecycle != "exam_scoped" && req.Lifecycle != "shared" {
		fields["lifecycle"] = "lifecycle must be 'exam_scoped' or 'shared'"
	}
	if req.Lifecycle == "exam_scoped" && (req.ParentExamID == nil || *req.ParentExamID == "") {
		fields["parentExamId"] = "parentExamId is required for exam_scoped stimuli"
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	// If exam_scoped, verify the caller can write to the parent exam.
	if req.Lifecycle == "exam_scoped" && req.ParentExamID != nil {
		if !a.requireExamAccess(w, r, *req.ParentExamID, ActionWrite) {
			return
		}
	}

	auth := AuthFromContext(r.Context())
	parent := ""
	if req.ParentExamID != nil {
		parent = *req.ParentExamID
	}

	var id string
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO stimuli (
		    tenant_id, owner_user_id, type, title, content, source,
		    lifecycle, parent_exam_id
		) VALUES ($1, $2, 'text', $3, $4, NULLIF($5,''),
		          $6, NULLIF($7,'')::uuid)
		RETURNING id::text`,
		tenantID, auth.UserID, req.Title, req.Content, strings.TrimSpace(req.Source),
		req.Lifecycle, parent,
	).Scan(&id)
	if err != nil {
		a.logger.Error("create stimulus failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed",
			"Could not create stimulus", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "stimuli.create", "stimulus", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "lifecycle": req.Lifecycle})
}

func (a *App) handleUpdateStimulus(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	id := r.PathValue("id")
	auth := AuthFromContext(r.Context())

	if !a.requireStimulusEditAccess(w, r, tenantID, auth, id) {
		return
	}

	var req struct {
		Title   *string `json:"title"`
		Content *string `json:"content"`
		Source  *string `json:"source"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}

	// Lifecycle lock: title/content edits are blocked when any linked
	// exam_question_group sits inside a published exam or an exam that
	// already has at least one attempt. Per Phase 9.5 lifecycle rules,
	// frozen content stays frozen — edits would silently desync the
	// library from what students saw / will see. Source-only edits
	// (citation) are always allowed because the snapshot does not store
	// it.
	if req.Title != nil || req.Content != nil {
		if locked, msg := a.stimulusContentLocked(r.Context(), id, tenantID); locked {
			writeErrorJSON(w, http.StatusConflict, "invalid_state", msg, r)
			return
		}
	}

	parts := []string{"updated_at = now()"}
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
	if req.Content != nil {
		add("content", *req.Content)
	}
	if req.Source != nil {
		if *req.Source == "" {
			parts = append(parts, "source = NULL")
		} else {
			add("source", *req.Source)
		}
	}
	if len(args) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "no_change"})
		return
	}

	q := "UPDATE stimuli SET " + strings.Join(parts, ", ") +
		" WHERE id = $" + strconv.Itoa(idx) + " AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, id, tenantID)
	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed",
			"Could not update stimulus", r)
		return
	}
	a.audit(r.Context(), &tenantID, auth.UserID, "stimuli.update", "stimulus", id, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "updated"})
}

// stimulusContentLocked checks whether any exam_question_group references
// this stimulus from a published exam OR an exam that already has at
// least one attempt. Returns (true, message) when the library content
// must not change — even owner/admin should not alter the source while
// frozen snapshots reference it. Source-only metadata edits are
// permitted because snapshots do not include source.
func (a *App) stimulusContentLocked(ctx context.Context, stimulusID, tenantID string) (bool, string) {
	var publishedCount, attemptedCount int
	_ = a.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN e.status = 'published' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN EXISTS(
		           SELECT 1 FROM exam_attempts a WHERE a.exam_id = e.id
		       ) THEN 1 ELSE 0 END), 0)
		  FROM exam_question_groups g
		  JOIN exams e ON e.id = g.exam_id
		 WHERE g.stimulus_id = $1 AND g.tenant_id = $2`,
		stimulusID, tenantID,
	).Scan(&publishedCount, &attemptedCount)
	if publishedCount > 0 {
		return true, "Stimulus content is locked: it is referenced by a published exam. Library edits would silently desync from the frozen snapshot. Use sync-snapshot only on draft exams."
	}
	if attemptedCount > 0 {
		return true, "Stimulus content is locked: it is referenced by an exam that has student attempts. Frozen snapshots are absolutely locked once any submission exists."
	}
	return false, ""
}

// handleSyncStimulusSnapshot copies the current library title + content
// into the stimulus_title_snapshot / stimulus_body_snapshot columns of
// every exam_question_group referencing this stimulus. Allowed only
// when ALL linked exams are status='draft' AND have zero attempts —
// once any exam is published or attempted, the snapshot is frozen.
func (a *App) handleSyncStimulusSnapshot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	id := r.PathValue("id")
	auth := AuthFromContext(r.Context())
	if !a.requireStimulusEditAccess(w, r, tenantID, auth, id) {
		return
	}

	// Verify all linked groups belong to draft exams with zero attempts.
	var nonDraft, attempted int
	_ = a.db.QueryRowContext(r.Context(), `
		SELECT COALESCE(SUM(CASE WHEN e.status != 'draft' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN EXISTS(
		           SELECT 1 FROM exam_attempts a WHERE a.exam_id = e.id
		       ) THEN 1 ELSE 0 END), 0)
		  FROM exam_question_groups g
		  JOIN exams e ON e.id = g.exam_id
		 WHERE g.stimulus_id = $1 AND g.tenant_id = $2`,
		id, tenantID,
	).Scan(&nonDraft, &attempted)
	if nonDraft > 0 || attempted > 0 {
		writeErrorJSON(w, http.StatusConflict, "invalid_state",
			"Sync requires every linked exam to be draft with zero attempts. Cannot refresh frozen snapshots.", r)
		return
	}

	res, err := a.db.ExecContext(r.Context(), `
		UPDATE exam_question_groups g SET
		    stimulus_title_snapshot = s.title,
		    stimulus_body_snapshot  = s.content,
		    updated_at = now()
		  FROM stimuli s
		 WHERE g.stimulus_id = s.id
		   AND s.id = $1 AND s.tenant_id = $2 AND g.tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "sync_failed",
			"Could not sync stimulus snapshots", r)
		return
	}
	count, _ := res.RowsAffected()
	a.audit(r.Context(), &tenantID, auth.UserID, "stimuli.sync_snapshot", "stimulus", id, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":            id,
		"status":        "synced",
		"groupsUpdated": count,
	})
}

func (a *App) handleArchiveStimulus(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	id := r.PathValue("id")
	auth := AuthFromContext(r.Context())
	if !a.requireStimulusEditAccess(w, r, tenantID, auth, id) {
		return
	}

	if _, err := a.db.ExecContext(r.Context(),
		`UPDATE stimuli SET lifecycle = 'archived', updated_at = now()
		  WHERE id = $1 AND tenant_id = $2 AND lifecycle != 'archived'`,
		id, tenantID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed",
			"Could not archive stimulus", r)
		return
	}
	a.audit(r.Context(), &tenantID, auth.UserID, "stimuli.archive", "stimulus", id, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "archived"})
}

// handlePromoteStimulus moves an exam_scoped stimulus into the
// tenant-wide library (lifecycle = shared, parent_exam_id cleared).
// Useful when a stimulus created inline turns out to be reusable.
func (a *App) handlePromoteStimulus(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	id := r.PathValue("id")
	auth := AuthFromContext(r.Context())
	if !a.requireStimulusEditAccess(w, r, tenantID, auth, id) {
		return
	}

	res, err := a.db.ExecContext(r.Context(), `
		UPDATE stimuli SET lifecycle = 'shared', parent_exam_id = NULL, updated_at = now()
		 WHERE id = $1 AND tenant_id = $2 AND lifecycle = 'exam_scoped'`,
		id, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "promote_failed",
			"Could not promote stimulus", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusConflict, "invalid_state",
			"Stimulus is not exam_scoped", r)
		return
	}
	a.audit(r.Context(), &tenantID, auth.UserID, "stimuli.promote", "stimulus", id, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "shared"})
}

// requireStimulusEditAccess checks owner-or-admin for stimulus mutations.
// Stimuli have no collaborator table (per ADR), so the rule is simpler
// than the exam/course/blueprint helpers.
func (a *App) requireStimulusEditAccess(
	w http.ResponseWriter, r *http.Request,
	tenantID string, auth *AuthContext, stimulusID string,
) bool {
	if isTenantAdmin(auth) {
		// Confirm existence in tenant
		var exists bool
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM stimuli WHERE id = $1 AND tenant_id = $2)`,
			stimulusID, tenantID,
		).Scan(&exists)
		if !exists {
			writeErrorJSON(w, http.StatusNotFound, "not_found", "Stimulus not found", r)
			return false
		}
		return true
	}

	var ownerID string
	err := a.db.QueryRowContext(r.Context(),
		`SELECT owner_user_id::text FROM stimuli WHERE id = $1 AND tenant_id = $2`,
		stimulusID, tenantID,
	).Scan(&ownerID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Stimulus not found", r)
		return false
	}
	if ownerID != auth.UserID {
		writeErrorJSON(w, http.StatusForbidden, "forbidden",
			"Only the owner or a tenant admin can edit this stimulus", r)
		return false
	}
	return true
}
