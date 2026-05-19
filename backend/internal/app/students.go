package app

import (
	"net/http"
	"strconv"
	"strings"

	"morfoschools/backend/internal/platform/httpx"
)

func (a *App) registerStudentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/students", a.handleListStudents)
	mux.HandleFunc("POST /api/v1/students", a.handleCreateStudent)
	mux.HandleFunc("PATCH /api/v1/students/{id}", a.handleUpdateStudent)
	mux.HandleFunc("PATCH /api/v1/students/{id}/archive", a.handleArchiveStudent)
	mux.HandleFunc("PATCH /api/v1/students/{id}/restore", a.handleRestoreStudent)
}

func (a *App) handleListStudents(w http.ResponseWriter, r *http.Request) {
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
	classFilter := httpx.QueryString(r, "classSectionId", "")

	countQuery := `SELECT COUNT(*) FROM students s JOIN users u ON u.id = s.user_id WHERE s.tenant_id = $1 AND s.status != 'archived'`
	countArgs := []any{tenantID}
	argIdx := 2

	if classFilter != "" {
		countQuery += ` AND s.class_section_id = $` + strconv.Itoa(argIdx)
		countArgs = append(countArgs, classFilter)
		argIdx++
	}
	if search != "" {
		countQuery += ` AND (u.display_name ILIKE $` + strconv.Itoa(argIdx) + ` OR u.email ILIKE $` + strconv.Itoa(argIdx) + `)`
		countArgs = append(countArgs, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		countQuery += ` AND s.status = $` + strconv.Itoa(argIdx)
		countArgs = append(countArgs, status)
		argIdx++
	}

	var total int
	if err := a.db.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&total); err != nil {
		a.logger.Error("count students failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "students_lookup_failed", "Could not load students", r)
		return
	}

	query := `SELECT s.id, s.user_id, u.email, u.display_name, s.student_id_number, s.grade_level, s.status, s.created_at
		FROM students s JOIN users u ON u.id = s.user_id
		WHERE s.tenant_id = $1 AND s.status != 'archived'`
	args := []any{tenantID}
	argIdx = 2

	if classFilter != "" {
		query += ` AND s.class_section_id = $` + strconv.Itoa(argIdx)
		args = append(args, classFilter)
		argIdx++
	}
	if search != "" {
		query += ` AND (u.display_name ILIKE $` + strconv.Itoa(argIdx) + ` OR u.email ILIKE $` + strconv.Itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += ` AND s.status = $` + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	query += ` ORDER BY s.created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, p.PageSize, p.Offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		a.logger.Error("list students failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "students_lookup_failed", "Could not load students", r)
		return
	}
	defer rows.Close()

	type StudentRow struct {
		ID              string  `json:"id"`
		UserID          string  `json:"userId"`
		Email           string  `json:"email"`
		DisplayName     string  `json:"displayName"`
		StudentIDNumber *string `json:"studentIdNumber"`
		GradeLevel      *string `json:"gradeLevel"`
		Status          string  `json:"status"`
		CreatedAt       string  `json:"createdAt"`
	}

	var students []StudentRow
	for rows.Next() {
		var s StudentRow
		if err := rows.Scan(&s.ID, &s.UserID, &s.Email, &s.DisplayName, &s.StudentIDNumber, &s.GradeLevel, &s.Status, &s.CreatedAt); err != nil {
			a.logger.Error("scan student failed", "error", err)
			continue
		}
		students = append(students, s)
	}

	writeJSON(w, http.StatusOK, httpx.NewPaginatedResponse(students, p, total))
}

func (a *App) handleCreateStudent(w http.ResponseWriter, r *http.Request) {
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
		UserID          string `json:"userId"`
		StudentIDNumber string `json:"studentIdNumber"`
		GradeLevel      string `json:"gradeLevel"`
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

	var userExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, req.UserID,
	).Scan(&userExists)
	if !userExists {
		writeValidationError(w, map[string]string{"userId": "User not found in this tenant"}, r)
		return
	}

	var alreadyExists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM students WHERE tenant_id = $1 AND user_id = $2)`,
		tenantID, req.UserID,
	).Scan(&alreadyExists)
	if alreadyExists {
		writeValidationError(w, map[string]string{"userId": "User is already registered as a student"}, r)
		return
	}

	var studentID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO students (tenant_id, user_id, student_id_number, grade_level, status)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), 'active') RETURNING id`,
		tenantID, req.UserID, strings.TrimSpace(req.StudentIDNumber), strings.TrimSpace(req.GradeLevel),
	).Scan(&studentID)
	if err != nil {
		a.logger.Error("insert student failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create student", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "students.create", "student", studentID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"id": studentID, "userId": req.UserID, "status": "active"})
}

func (a *App) handleUpdateStudent(w http.ResponseWriter, r *http.Request) {
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

	studentID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM students WHERE id = $1 AND tenant_id = $2)`,
		studentID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Student not found", r)
		return
	}

	var req struct {
		StudentIDNumber *string `json:"studentIdNumber"`
		GradeLevel      *string `json:"gradeLevel"`
		Status          *string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if req.StudentIDNumber != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE students SET student_id_number = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.StudentIDNumber), studentID)
	}
	if req.GradeLevel != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE students SET grade_level = NULLIF($1,''), updated_at = now() WHERE id = $2`, strings.TrimSpace(*req.GradeLevel), studentID)
	}
	if req.Status != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE students SET status = $1, updated_at = now() WHERE id = $2`, *req.Status, studentID)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "students.update", "student", studentID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": studentID, "status": "updated"})
}

func (a *App) handleArchiveStudent(w http.ResponseWriter, r *http.Request) {
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

	studentID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM students WHERE id = $1 AND tenant_id = $2)`,
		studentID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Student not found", r)
		return
	}

	userID, _ := userIDForProfile(r.Context(), a.db, "students", studentID)

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		a.logger.Error("begin tx failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive student", r)
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(),
		`UPDATE students SET status = 'archived', updated_at = now() WHERE id = $1`, studentID,
	); err != nil {
		a.logger.Error("archive student failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive student", r)
		return
	}

	// Cascade: archive the parent user iff this was their last active profile,
	// freeing the email slot for re-registration.
	cascaded, err := cascadeArchiveUserIfOrphan(r.Context(), tx, userID)
	if err != nil {
		a.logger.Error("cascade archive user failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive student", r)
		return
	}
	if err := tx.Commit(); err != nil {
		a.logger.Error("commit archive failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "archive_failed", "Could not archive student", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "students.archive", "student", studentID, r)
	if cascaded {
		a.audit(r.Context(), &tenantID, auth.UserID, "users.archive_cascade", "user", userID, r)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "archived",
		"userArchived": cascaded,
	})
}

func (a *App) handleRestoreStudent(w http.ResponseWriter, r *http.Request) {
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

	studentID := r.PathValue("id")
	var exists bool
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM students WHERE id = $1 AND tenant_id = $2)`,
		studentID, tenantID,
	).Scan(&exists)
	if !exists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Student not found", r)
		return
	}

	userID, _ := userIDForProfile(r.Context(), a.db, "students", studentID)

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore student", r)
		return
	}
	defer tx.Rollback()

	if err := restoreProfile(r.Context(), tx, "students", studentID, "active"); err != nil {
		a.logger.Error("restore student failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore student", r)
		return
	}

	// Restore the parent user too. If the original email is already taken,
	// surface a 409 so the admin can supply a new one.
	resolved, taken, err := restoreUser(r.Context(), tx, userID, "")
	if err != nil {
		a.logger.Error("restore user failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore student", r)
		return
	}
	if taken {
		writeValidationError(w, map[string]string{
			"email": "Email " + resolved + " is already in use. Restore the user account manually with a new email first.",
		}, r)
		return
	}
	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "restore_failed", "Could not restore student", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "students.restore", "student", studentID, r)

	writeJSON(w, http.StatusOK, map[string]any{"id": studentID, "status": "active"})
}
