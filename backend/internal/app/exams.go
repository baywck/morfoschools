package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

// Exams: list, create, edit, archive, restore, publish.
//
// RBAC:
//   exams:read   for list/get
//   exams:write  for create/update/archive/publish
// Teachers may only author exams whose subject is in their teacher_subjects.
// Admins are unrestricted within their tenant. Enforcement lives in
// requireExamSubjectAccess.

func (a *App) registerExamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/exams", a.handleListExams)
	mux.HandleFunc("POST /api/v1/exams", a.handleCreateExam)
	mux.HandleFunc("GET /api/v1/exams/{id}", a.handleGetExam)
	mux.HandleFunc("PATCH /api/v1/exams/{id}", a.handleUpdateExam)
	mux.HandleFunc("PATCH /api/v1/exams/{id}/publish", a.handlePublishExam)
	mux.HandleFunc("PATCH /api/v1/exams/{id}/archive", a.handleArchiveExam)
	mux.HandleFunc("PATCH /api/v1/exams/{id}/restore", a.handleRestoreExam)
	mux.HandleFunc("DELETE /api/v1/exams/{id}", a.handleHardDeleteExam)
}

type examRow struct {
	ID                    string  `json:"id"`
	Title                 string  `json:"title"`
	Description           *string `json:"description"`
	SubjectID             *string `json:"subjectId"`
	SubjectName           *string `json:"subjectName,omitempty"`
	GradeLevel            *string `json:"gradeLevel"`
	ExamType              string  `json:"examType"`
	DurationMinutes       *int    `json:"durationMinutes"`
	MaxScore              float64 `json:"maxScore"`
	PassingScore          float64 `json:"passingScore"`
	Status                string  `json:"status"`
	UsesKisiKisi          bool    `json:"usesKisiKisi"`
	ShuffleQuestions      bool    `json:"shuffleQuestions"`
	ShuffleOptions        bool    `json:"shuffleOptions"`
	ShowResultImmediately bool    `json:"showResultImmediately"`
	PublishedAt           *string `json:"publishedAt"`
	CreatedAt             string  `json:"createdAt"`
	QuestionCount         int     `json:"questionCount"`
	TotalPoints           float64 `json:"totalPoints"`
	CanAccess             bool    `json:"canAccess"`
	CanWrite              bool    `json:"canWrite"`
	CanDelete             bool    `json:"canDelete"`
}

// isAdminOverrideRole returns true if the auth context can override the
// draft-only kisi-kisi-toggle transition gate. Platform / master admins
// are the escape hatch; tenant admins are not enough since flipping the
// kisi-kisi enforcement on a published exam silently changes the
// publish-coverage contract for students who already started.
func isAdminOverrideRole(auth *AuthContext) bool {
	if auth == nil {
		return false
	}
	if auth.IsPlatformAdmin {
		return true
	}
	for _, r := range auth.Roles {
		if r == "master_admin" || r == "platform_admin" {
			return true
		}
	}
	return false
}

func (a *App) handleListExams(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	p := httpx.ParsePagination(r)
	search := httpx.QueryString(r, "search", "")
	status := httpx.QueryString(r, "status", "")
	subjectID := httpx.QueryString(r, "subjectId", "")
	auth := AuthFromContext(r.Context())

	appendExamListAccessFilter := func(query *string, args *[]any, argIdx *int) {
		if isTenantAdmin(auth) {
			return
		}
		if auth == nil || auth.UserID == "" {
			*query += ` AND false`
			return
		}
		ownerArg := *argIdx
		*args = append(*args, auth.UserID)
		*argIdx++
		collabArg := *argIdx
		*args = append(*args, auth.UserID)
		*argIdx++
		subjectUserArg := *argIdx
		*args = append(*args, auth.UserID)
		*argIdx++
		*query += ` AND (
			e.owner_user_id = $` + strconv.Itoa(ownerArg) + `
			OR EXISTS (SELECT 1 FROM exam_collaborators ec WHERE ec.exam_id = e.id AND ec.user_id = $` + strconv.Itoa(collabArg) + `)
			OR EXISTS (
				SELECT 1 FROM teacher_subjects ts
				  JOIN teachers t ON t.id = ts.teacher_id
				 WHERE t.user_id = $` + strconv.Itoa(subjectUserArg) + `
				   AND ts.tenant_id = e.tenant_id
				   AND ts.subject_id = e.subject_id
				   AND ts.status = 'active'
				   AND t.status = 'active'
			)
		)`
	}

	countQuery := `SELECT COUNT(*) FROM exams e WHERE e.tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2
	appendExamListAccessFilter(&countQuery, &args, &argIdx)
	if search != "" {
		countQuery += ` AND e.title ILIKE $` + strconv.Itoa(argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		countQuery += ` AND e.status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	if subjectID != "" {
		countQuery += ` AND e.subject_id = $` + strconv.Itoa(argIdx)
		args = append(args, subjectID)
		argIdx++
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
		a.logger.Error("count exams failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "exams_lookup_failed", "Could not load exams", r)
		return
	}

	// Aggregate question count + total points in the same query — admins
	// see this on the list view, no need for an N+1 follow-up.
	query := `
		SELECT e.id, e.title, e.description, e.subject_id, s.name AS subject_name, e.grade_level,
		       e.exam_type, e.duration_minutes, e.max_score, e.passing_score,
		       e.status, e.uses_kisi_kisi,
		       e.shuffle_questions, e.shuffle_options, e.show_result_immediately,
		       e.published_at, e.created_at,
		       COALESCE((SELECT COUNT(*) FROM exam_questions q WHERE q.exam_id = e.id AND q.tenant_id = e.tenant_id), 0) AS question_count,
		       COALESCE((SELECT SUM(points) FROM exam_questions q WHERE q.exam_id = e.id AND q.tenant_id = e.tenant_id), 0) AS total_points
		  FROM exams e
		  LEFT JOIN subjects s ON s.id = e.subject_id
		 WHERE e.tenant_id = $1`

	args = []any{tenantID}
	argIdx = 2
	appendExamListAccessFilter(&query, &args, &argIdx)
	if search != "" {
		query += ` AND e.title ILIKE $` + strconv.Itoa(argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND e.status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	if subjectID != "" {
		query += ` AND e.subject_id = $` + strconv.Itoa(argIdx)
		args = append(args, subjectID)
		argIdx++
	}
	query += ` ORDER BY e.created_at DESC LIMIT $` + strconv.Itoa(argIdx) +
		` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list exams failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "exams_lookup_failed", "Could not load exams", r)
		return
	}
	defer rows.Close()

	out := make([]examRow, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var e examRow
		if err := rows.Scan(
			&e.ID, &e.Title, &e.Description, &e.SubjectID, &e.SubjectName, &e.GradeLevel,
			&e.ExamType, &e.DurationMinutes, &e.MaxScore, &e.PassingScore,
			&e.Status, &e.UsesKisiKisi,
			&e.ShuffleQuestions, &e.ShuffleOptions, &e.ShowResultImmediately,
			&e.PublishedAt, &e.CreatedAt, &e.QuestionCount, &e.TotalPoints,
		); err != nil {
			continue
		}
		out = append(out, e)
		ids = append(ids, e.ID)
	}

	access := a.examAccessBatch(r.Context(), tenantID, auth, ids)
	for i := range out {
		acc := access[out[i].ID]
		out[i].CanAccess = acc.CanRead
		out[i].CanWrite = acc.CanWrite
		out[i].CanDelete = acc.CanDelete
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(out, p, total))
}

func (a *App) handleGetExam(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	examID := r.PathValue("id")
	if examID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Exam ID is required", r)
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionRead) {
		return
	}

	var e examRow
	err := a.db.QueryRowContext(r.Context(), `
		SELECT e.id, e.title, e.description, e.subject_id, s.name AS subject_name, e.grade_level,
		       e.exam_type, e.duration_minutes, e.max_score, e.passing_score,
		       e.status, e.uses_kisi_kisi,
		       e.shuffle_questions, e.shuffle_options, e.show_result_immediately,
		       e.published_at, e.created_at,
		       COALESCE((SELECT COUNT(*) FROM exam_questions q WHERE q.exam_id = e.id AND q.tenant_id = e.tenant_id), 0),
		       COALESCE((SELECT SUM(points) FROM exam_questions q WHERE q.exam_id = e.id AND q.tenant_id = e.tenant_id), 0)
		  FROM exams e
		  LEFT JOIN subjects s ON s.id = e.subject_id
		 WHERE e.id = $1 AND e.tenant_id = $2`,
		examID, tenantID,
	).Scan(
		&e.ID, &e.Title, &e.Description, &e.SubjectID, &e.SubjectName, &e.GradeLevel,
		&e.ExamType, &e.DurationMinutes, &e.MaxScore, &e.PassingScore,
		&e.Status, &e.UsesKisiKisi,
		&e.ShuffleQuestions, &e.ShuffleOptions, &e.ShowResultImmediately,
		&e.PublishedAt, &e.CreatedAt, &e.QuestionCount, &e.TotalPoints,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found", r)
		return
	}
	e.CanAccess = true // verified by requireExamAccess above
	writeJSON(w, http.StatusOK, e)
}

func (a *App) handleCreateExam(w http.ResponseWriter, r *http.Request) {
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
		Title                 string   `json:"title"`
		Description           string   `json:"description"`
		SubjectID             *string  `json:"subjectId"`
		GradeLevel            *string  `json:"gradeLevel"`
		ExamType              string   `json:"examType"`
		DurationMinutes       *int     `json:"durationMinutes"`
		MaxScore              *float64 `json:"maxScore"`
		PassingScore          *float64 `json:"passingScore"`
		ShuffleQuestions      bool     `json:"shuffleQuestions"`
		ShuffleOptions        bool     `json:"shuffleOptions"`
		ShowResultImmediately bool     `json:"showResultImmediately"`
		UsesKisiKisi          *bool    `json:"usesKisiKisi"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	if req.Title == "" {
		fields["title"] = "Title is required"
	}
	if req.ExamType == "" {
		req.ExamType = "quiz"
	}
	// usesKisiKisi defaults to false (boolean column has DB default).
	// Per ADR-0012 fresh exams start with the toggle off; applying a
	// blueprint flips it on as a side effect of clone_blueprint_to_exam.
	usesKisiKisi := false
	if req.UsesKisiKisi != nil {
		usesKisiKisi = *req.UsesKisiKisi
	}
	if req.GradeLevel != nil {
		for key, message := range a.validateTenantGradeLevel(r.Context(), tenantID, *req.GradeLevel) {
			fields[key] = message
		}
	}
	if len(fields) > 0 {
		writeValidationError(w, fields, r)
		return
	}

	auth := AuthFromContext(r.Context())
	// Phase 9.5 retrofit: ownership + collaborator model governs writes,
	// not subject membership. Anyone with `exams:write` in the active
	// tenant can create an exam; they automatically become owner. The
	// optional subjectId is now read-fallback metadata only — we still
	// validate it belongs to the tenant so the FK insert won't fail.
	if req.SubjectID != nil && *req.SubjectID != "" {
		var subjectExists bool
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM subjects WHERE id = $1 AND tenant_id = $2)`,
			*req.SubjectID, tenantID,
		).Scan(&subjectExists)
		if !subjectExists {
			writeValidationError(w, map[string]string{
				"subjectId": "Subject not found in this tenant",
			}, r)
			return
		}
	}

	maxScore := 100.0
	if req.MaxScore != nil {
		maxScore = *req.MaxScore
	}
	passingScore := 70.0
	if req.PassingScore != nil {
		passingScore = *req.PassingScore
	}

	// Per Phase 9.8 UX rewrite: every exam must have at least one section.
	// Auto-create "Section 1" inside the same transaction so the canvas
	// always has a container ready when the user lands on the detail page.
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create exam", r)
		return
	}
	defer tx.Rollback()

	var id string
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO exams (
		    tenant_id, title, description, subject_id, grade_level, exam_type,
		    duration_minutes, max_score, passing_score,
		    shuffle_questions, shuffle_options, show_result_immediately,
		    created_by, owner_user_id, status, uses_kisi_kisi
		) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,'')::uuid, NULLIF($5,''), $6,
		          $7, $8, $9, $10, $11, $12, $13, $13, 'draft', $14)
		RETURNING id`,
		tenantID, req.Title, req.Description, ptrToString(req.SubjectID), ptrToString(req.GradeLevel), req.ExamType,
		req.DurationMinutes, maxScore, passingScore,
		req.ShuffleQuestions, req.ShuffleOptions, req.ShowResultImmediately,
		auth.UserID, usesKisiKisi,
	).Scan(&id)
	if err != nil {
		a.logger.Error("create exam failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create exam", r)
		return
	}

	var defaultSectionID string
	if err := tx.QueryRowContext(r.Context(), `
		INSERT INTO exam_sections (tenant_id, exam_id, title, sort_order)
		VALUES ($1, $2, 'Section 1', 0)
		RETURNING id`,
		tenantID, id,
	).Scan(&defaultSectionID); err != nil {
		a.logger.Error("create default section failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create default section", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not finalize exam create", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exams.create", "exam", id, r)
	a.audit(r.Context(), &tenantID, auth.UserID, "exam_sections.create", "exam_section", defaultSectionID, r)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":               id,
		"status":           "draft",
		"usesKisiKisi":     usesKisiKisi,
		"defaultSectionId": defaultSectionID,
	})
}

func (a *App) handleUpdateExam(w http.ResponseWriter, r *http.Request) {
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
	examID := r.PathValue("id")
	if examID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Exam ID is required", r)
		return
	}
	// Layered access: owner / collaborator(editor) / admin / subject fallback.
	// Replaces the prior subject-only gate per ADR-0009.
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}

	var current struct {
		Status       string
		SubjectID    *string
		UsesKisiKisi bool
	}
	err := a.db.QueryRowContext(r.Context(),
		`SELECT status, subject_id::text, uses_kisi_kisi FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&current.Status, &current.SubjectID, &current.UsesKisiKisi)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found", r)
		return
	}

	auth := AuthFromContext(r.Context())

	var req struct {
		Title                 *string  `json:"title"`
		Description           *string  `json:"description"`
		SubjectID             *string  `json:"subjectId"`
		GradeLevel            *string  `json:"gradeLevel"`
		ExamType              *string  `json:"examType"`
		DurationMinutes       *int     `json:"durationMinutes"`
		MaxScore              *float64 `json:"maxScore"`
		PassingScore          *float64 `json:"passingScore"`
		ShuffleQuestions      *bool    `json:"shuffleQuestions"`
		ShuffleOptions        *bool    `json:"shuffleOptions"`
		ShowResultImmediately *bool    `json:"showResultImmediately"`
		UsesKisiKisi          *bool    `json:"usesKisiKisi"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.GradeLevel != nil {
		if fields := a.validateTenantGradeLevel(r.Context(), tenantID, *req.GradeLevel); len(fields) > 0 {
			writeValidationError(w, fields, r)
			return
		}
	}

	// Kisi-kisi toggle gate (ADR-0012). Same admin override contract as
	// ADR-0012 toggle flips: platform/master admin can change toggle on a
	// non-draft exam; everyone else needs the exam in draft. Slots and
	// slot bindings are PRESERVED on disable — just bypass enforcement.
	var toggleWarning string
	var toggleHint string
	if req.UsesKisiKisi != nil && *req.UsesKisiKisi != current.UsesKisiKisi {
		if current.Status != "draft" && !isAdminOverrideRole(auth) {
			writeErrorJSON(w, http.StatusConflict, "invalid_state",
				"Kisi-kisi toggle can only change while exam is draft", r)
			return
		}
		isDisable := current.UsesKisiKisi && !*req.UsesKisiKisi
		if isDisable {
			var boundCount int
			_ = a.db.QueryRowContext(r.Context(), `
				SELECT COUNT(*) FROM exam_questions q
				 WHERE q.exam_id = $1 AND q.tenant_id = $2 AND q.blueprint_slot_id IS NOT NULL`,
				examID, tenantID,
			).Scan(&boundCount)
			if boundCount > 0 {
				toggleWarning = fmt.Sprintf("%d question(s) remain bound to kisi-kisi slots; coverage gate is now disabled but bindings are preserved.", boundCount)
			}
		}
		isEnable := !current.UsesKisiKisi && *req.UsesKisiKisi
		if isEnable {
			// Hint to the frontend: when the exam already has questions
			// but no blueprint, the user will land on an empty canvas.
			// Surface a CTA to apply a template or generate from existing.
			var questionCount, blueprintCount int
			_ = a.db.QueryRowContext(r.Context(),
				`SELECT COUNT(*) FROM exam_questions WHERE exam_id = $1 AND tenant_id = $2`,
				examID, tenantID,
			).Scan(&questionCount)
			_ = a.db.QueryRowContext(r.Context(),
				`SELECT COUNT(*) FROM exam_blueprints WHERE exam_id = $1 AND tenant_id = $2`,
				examID, tenantID,
			).Scan(&blueprintCount)
			if questionCount > 0 && blueprintCount == 0 {
				toggleHint = "Apply a blueprint template or generate kisi-kisi from existing questions to anchor your slots."
			}
		}
	}

	// If the caller is reassigning to a different subject, re-check teacher
	// subject access against the NEW subject. A teacher with access to the
	// current subject must not be able to move the exam onto a subject they
	// are not assigned to.
	if req.SubjectID != nil && *req.SubjectID != "" {
		curSubject := ""
		if current.SubjectID != nil {
			curSubject = *current.SubjectID
		}
		if *req.SubjectID != curSubject {
			if !a.requireExamSubjectAccess(w, r, tenantID, auth, *req.SubjectID) {
				return
			}
		}
	}

	// Build a partial UPDATE. We assemble in code rather than Squirrel-style
	// to keep the dependency-free style of the rest of the project.
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
	if req.Description != nil {
		add("description", *req.Description)
	}
	if req.SubjectID != nil {
		if *req.SubjectID == "" {
			parts = append(parts, "subject_id = NULL")
		} else {
			add("subject_id", *req.SubjectID)
		}
	}
	if req.GradeLevel != nil {
		if strings.TrimSpace(*req.GradeLevel) == "" {
			parts = append(parts, "grade_level = NULL")
		} else {
			add("grade_level", strings.TrimSpace(*req.GradeLevel))
		}
	}
	if req.ExamType != nil {
		add("exam_type", *req.ExamType)
	}
	if req.DurationMinutes != nil {
		add("duration_minutes", *req.DurationMinutes)
	}
	if req.MaxScore != nil {
		add("max_score", *req.MaxScore)
	}
	if req.PassingScore != nil {
		add("passing_score", *req.PassingScore)
	}
	if req.ShuffleQuestions != nil {
		add("shuffle_questions", *req.ShuffleQuestions)
	}
	if req.ShuffleOptions != nil {
		add("shuffle_options", *req.ShuffleOptions)
	}
	if req.ShowResultImmediately != nil {
		add("show_result_immediately", *req.ShowResultImmediately)
	}
	if req.UsesKisiKisi != nil && *req.UsesKisiKisi != current.UsesKisiKisi {
		add("uses_kisi_kisi", *req.UsesKisiKisi)
	}

	if len(args) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"id": examID, "status": "no_change"})
		return
	}

	q := "UPDATE exams SET " + joinComma(parts) +
		" WHERE id = $" + strconv.Itoa(idx) +
		" AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, examID, tenantID)

	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		a.logger.Error("update exam failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update exam", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exams.update", "exam", examID, r)
	resp := map[string]any{"id": examID, "status": "updated"}
	if toggleWarning != "" {
		resp["warning"] = toggleWarning
	}
	if toggleHint != "" {
		resp["hint"] = toggleHint
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handlePublishExam(w http.ResponseWriter, r *http.Request) {
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
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}

	auth := AuthFromContext(r.Context())
	if _, ok := a.requireExamWriteAccess(w, r, tenantID, auth, examID); !ok {
		return
	}

	// Publish gate: at least one question must exist.
	var questionCount int
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM exam_questions WHERE exam_id = $1 AND tenant_id = $2`, examID, tenantID,
	).Scan(&questionCount)
	if questionCount == 0 {
		writeValidationError(w, map[string]string{
			"questions": "Cannot publish an exam with zero questions",
		}, r)
		return
	}

	// Strict-coverage gate (ADR-0012). When the exam has
	// uses_kisi_kisi=true AND a draft blueprint AND strict_coverage=true,
	// every slot must be filled. AKM detection is implicit: AKM blueprints
	// are seeded with strict_coverage=true at clone time, so the test is
	// the same regardless of curriculum. When uses_kisi_kisi=false, the
	// gate is bypassed even if a blueprint is present (downgrade path).
	var usesKisiKisi bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT uses_kisi_kisi FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&usesKisiKisi)
	var (
		bpID                        sql.NullString
		bpStrict                    bool
		bpTotalSlots, bpFilledSlots int
	)
	_ = a.db.QueryRowContext(r.Context(), `
		SELECT b.id::text, b.strict_coverage, b.total_slots,
		       (SELECT COUNT(*) FROM exam_blueprint_slots s
		          JOIN exam_questions q ON q.blueprint_slot_id = s.id
		         WHERE s.exam_blueprint_id = b.id) AS filled
		  FROM exam_blueprints b
		 WHERE b.exam_id = $1 AND b.tenant_id = $2 AND b.status = 'draft'`,
		examID, tenantID,
	).Scan(&bpID, &bpStrict, &bpTotalSlots, &bpFilledSlots)
	enforceCoverage := usesKisiKisi && bpID.Valid && bpStrict
	if enforceCoverage && bpFilledSlots < bpTotalSlots {
		writeValidationError(w, map[string]string{
			"coverage": fmt.Sprintf("%d filled of %d slots; strict-coverage blueprint requires 100%%", bpFilledSlots, bpTotalSlots),
		}, r)
		return
	}

	// Atomic publish: flip exam.status and lock the blueprint in one tx.
	// Locking the blueprint mirrors ADR-0010's lifecycle: a published
	// exam's kisi-kisi is read-only and matches what students see.
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "publish_failed", "Could not publish exam", r)
		return
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(r.Context(),
		`UPDATE exams SET status = 'published', published_at = now(), updated_at = now()
		  WHERE id = $1 AND tenant_id = $2 AND status = 'draft'`,
		examID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "publish_failed", "Could not publish exam", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusConflict, "invalid_state", "Exam is not in draft status", r)
		return
	}

	if _, err := tx.ExecContext(r.Context(),
		`UPDATE exam_blueprints SET status = 'locked', updated_at = now()
		  WHERE exam_id = $1 AND tenant_id = $2 AND status = 'draft'`,
		examID, tenantID,
	); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "publish_failed", "Could not lock blueprint", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "publish_failed", "Could not finalize publish", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exams.publish", "exam", examID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": examID, "status": "published"})
}

func (a *App) handleArchiveExam(w http.ResponseWriter, r *http.Request) {
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
	examID := r.PathValue("id")
	// Archive is a destructive lifecycle change — owner / admin only.
	if !a.requireExamAccess(w, r, examID, ActionDelete) {
		return
	}

	auth := AuthFromContext(r.Context())
	if _, ok := a.requireExamWriteAccess(w, r, tenantID, auth, examID); !ok {
		return
	}

	res, err := a.db.ExecContext(r.Context(),
		`UPDATE exams SET status = 'archived', archived_at = now(), updated_at = now()
		  WHERE id = $1 AND tenant_id = $2 AND status != 'archived'`,
		examID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive exam", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found or already archived", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exams.archive", "exam", examID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": examID, "status": "archived"})
}

func (a *App) handleHardDeleteExam(w http.ResponseWriter, r *http.Request) {
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
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionDelete) {
		return
	}

	auth := AuthFromContext(r.Context())
	if _, ok := a.requireExamWriteAccess(w, r, tenantID, auth, examID); !ok {
		return
	}

	res, err := a.db.ExecContext(r.Context(),
		`DELETE FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	)
	if err != nil {
		a.logger.Error("hard delete exam failed", "error", err, "examId", examID)
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not permanently delete exam", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exams.hard_delete", "exam", examID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": examID, "status": "deleted"})
}

func (a *App) handleRestoreExam(w http.ResponseWriter, r *http.Request) {
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
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}

	auth := AuthFromContext(r.Context())
	if _, ok := a.requireExamWriteAccess(w, r, tenantID, auth, examID); !ok {
		return
	}

	res, err := a.db.ExecContext(r.Context(),
		`UPDATE exams SET status = 'draft', archived_at = NULL, updated_at = now()
		  WHERE id = $1 AND tenant_id = $2 AND status = 'archived'`,
		examID, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore exam", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found or not archived", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "exams.restore", "exam", examID, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": examID, "status": "draft"})
}

// checkTeacherSubjectAccess is the non-HTTP predicate at the heart of
// exam authoring RBAC. Tenant admins / staff with exams:write but no
// active teacher profile are allowed for any subject. Teachers must have
// an active (tenant, subject) row in teacher_subjects.
//
// Subject "" or NULL is treated as no-subject and always allowed (the
// exam is unscoped). Use this from AI tool executors and any internal
// path; HTTP handlers should prefer requireExamSubjectAccess which writes
// the 403 response.
func (a *App) checkTeacherSubjectAccess(ctx context.Context, tenantID, userID, subjectID string) bool {
	if subjectID == "" {
		return true
	}
	var isTeacher bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM teachers WHERE user_id = $1 AND tenant_id = $2 AND status = 'active')`,
		userID, tenantID,
	).Scan(&isTeacher)
	if !isTeacher {
		return true
	}
	var allowed bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(
		    SELECT 1 FROM teacher_subjects ts
		      JOIN teachers t ON t.id = ts.teacher_id
		     WHERE t.user_id = $1 AND ts.tenant_id = $2 AND ts.subject_id = $3 AND ts.status = 'active'
		)`,
		userID, tenantID, subjectID,
	).Scan(&allowed)
	return allowed
}

// requireExamSubjectAccess enforces that the caller has permission to author
// for the given subject. Returns true when access is granted. When it
// returns false the response has already been written.
func (a *App) requireExamSubjectAccess(
	w http.ResponseWriter, r *http.Request,
	tenantID string, auth *AuthContext, subjectID string,
) bool {
	if a.checkTeacherSubjectAccess(r.Context(), tenantID, auth.UserID, subjectID) {
		return true
	}
	writeErrorJSON(w, http.StatusForbidden, "forbidden",
		"You can only author exams for subjects assigned to you", r)
	return false
}

// requireExamWriteAccess loads an exam by ID, verifies it belongs to the
// caller's tenant, then runs subject-based RBAC against its subject. This
// is the standard write-side gate for any handler that mutates exam
// state by examID-from-URL (publish, archive, restore, sections, gates).
//
// Returns the loaded subject (may be empty) when access granted; empty
// string + false when denied (response already written).
func (a *App) requireExamWriteAccess(
	w http.ResponseWriter, r *http.Request,
	tenantID string, auth *AuthContext, examID string,
) (subjectID string, ok bool) {
	var sid sql.NullString
	err := a.db.QueryRowContext(r.Context(),
		`SELECT subject_id::text FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&sid)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found", r)
		return "", false
	}
	if sid.Valid && sid.String != "" {
		if !a.requireExamSubjectAccess(w, r, tenantID, auth, sid.String) {
			return "", false
		}
		return sid.String, true
	}
	return "", true
}

// requireQuestionWriteAccess loads the question, verifies it belongs to the
// caller's tenant, then runs requireExamAccess against the parent exam
// (per ADR-0009 layered access model). Returns the loaded examID for
// downstream use, or empty string when access was denied (response
// already written).
//
// Use this anywhere we mutate a question or its options by ID-from-URL.
func (a *App) requireQuestionWriteAccess(
	w http.ResponseWriter, r *http.Request,
	tenantID string, auth *AuthContext, questionID string,
) (examID string, ok bool) {
	_ = auth // signature retained for callers; access check uses request context
	var examIDOut string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT q.exam_id
		  FROM exam_questions q
		 WHERE q.id = $1 AND q.tenant_id = $2`,
		questionID, tenantID,
	).Scan(&examIDOut)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Question not found", r)
		return "", false
	}
	if !a.requireExamAccess(w, r, examIDOut, ActionWrite) {
		return "", false
	}
	return examIDOut, true
}

// requireOptionWriteAccess loads the option, verifies it belongs to the
// caller's tenant, then defers to requireQuestionWriteAccess for the parent
// question's subject RBAC.
func (a *App) requireOptionWriteAccess(
	w http.ResponseWriter, r *http.Request,
	tenantID string, auth *AuthContext, optionID string,
) (questionID string, ok bool) {
	var qid string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT question_id FROM exam_question_options WHERE id = $1 AND tenant_id = $2`,
		optionID, tenantID,
	).Scan(&qid); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Option not found", r)
		return "", false
	}
	if _, ok2 := a.requireQuestionWriteAccess(w, r, tenantID, auth, qid); !ok2 {
		return "", false
	}
	return qid, true
}

// joinComma joins a slice of strings with ", ". Local helper to avoid
// pulling in strings.Join just for clarity at call sites.
func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// Ensure json import is used. The package-level var keeps the import
// honest if any handler in this file needs raw json work later.
var _ = json.Marshal
