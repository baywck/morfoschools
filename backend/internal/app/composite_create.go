package app

import (
	"net/http"
	"strings"
)

// Composite create endpoints — create user + profile + assignments in one request

// POST /api/v1/teachers/create-full
func (a *App) handleCreateTeacherFull(w http.ResponseWriter, r *http.Request) {
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
		DisplayName    string   `json:"displayName"`
		Email          string   `json:"email"`
		Password       string   `json:"password"`
		EmployeeID     string   `json:"employeeId"`
		Specialization string   `json:"specialization"`
		SubjectIDs     []string `json:"subjectIds"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	if req.DisplayName == "" {
		fields["displayName"] = "Name is required"
	}
	if req.Email == "" {
		fields["email"] = "Email is required"
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

	// Check email uniqueness
	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
	if exists {
		writeValidationError(w, map[string]string{"email": "Email already in use"}, r)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create teacher", r)
		return
	}
	defer tx.Rollback()

	// Create user
	hash := hashPassword(req.Password)
	var userID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO users (email, display_name, status) VALUES ($1, $2, 'active') RETURNING id`,
		req.Email, req.DisplayName,
	).Scan(&userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}

	// Password
	_, err = tx.ExecContext(r.Context(), `INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`, userID, hash)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not set password", r)
		return
	}

	// Tenant membership
	_, err = tx.ExecContext(r.Context(), `INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary) VALUES ($1, $2, 'active', true)`, tenantID, userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create membership", r)
		return
	}

	// Assign teacher role
	_, _ = tx.ExecContext(r.Context(),
		`INSERT INTO user_roles (tenant_id, user_id, role_id) SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = 'teacher' ON CONFLICT DO NOTHING`,
		tenantID, userID,
	)

	// Create teacher profile
	var teacherID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO teachers (tenant_id, user_id, employee_id, specialization, status) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), 'active') RETURNING id`,
		tenantID, userID, strings.TrimSpace(req.EmployeeID), strings.TrimSpace(req.Specialization),
	).Scan(&teacherID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create teacher profile", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not complete creation", r)
		return
	}

	// Assign subjects (outside transaction, non-critical)
	for _, subjectID := range req.SubjectIDs {
		_, _ = a.db.ExecContext(r.Context(),
			`INSERT INTO teaching_assignments (tenant_id, teacher_id, subject_id, class_section_id, academic_year_id, status)
			 SELECT $1, $2, $3, cs.id, cs.academic_year_id, 'active'
			 FROM class_sections cs WHERE cs.tenant_id = $1 LIMIT 1
			 ON CONFLICT (tenant_id, teacher_id, subject_id, class_section_id, academic_year_id) DO NOTHING`,
			tenantID, teacherID, subjectID,
		)
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "teachers.create_full", "teacher", teacherID, r)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       teacherID,
		"userId":   userID,
		"email":    req.Email,
		"displayName": req.DisplayName,
		"status":   "active",
	})
}

// POST /api/v1/students/create-full
func (a *App) handleCreateStudentFull(w http.ResponseWriter, r *http.Request) {
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
		DisplayName     string  `json:"displayName"`
		Email           string  `json:"email"`
		Password        string  `json:"password"`
		StudentIDNumber string  `json:"studentIdNumber"`
		GradeLevel      string  `json:"gradeLevel"`
		ClassSectionID  *string `json:"classSectionId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	if req.DisplayName == "" {
		fields["displayName"] = "Name is required"
	}
	if req.Email == "" {
		fields["email"] = "Email is required"
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

	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
	if exists {
		writeValidationError(w, map[string]string{"email": "Email already in use"}, r)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create student", r)
		return
	}
	defer tx.Rollback()

	hash := hashPassword(req.Password)
	var userID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO users (email, display_name, status) VALUES ($1, $2, 'active') RETURNING id`,
		req.Email, req.DisplayName,
	).Scan(&userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}

	_, _ = tx.ExecContext(r.Context(), `INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`, userID, hash)
	_, _ = tx.ExecContext(r.Context(), `INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary) VALUES ($1, $2, 'active', true)`, tenantID, userID)
	_, _ = tx.ExecContext(r.Context(),
		`INSERT INTO user_roles (tenant_id, user_id, role_id) SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = 'student' ON CONFLICT DO NOTHING`,
		tenantID, userID,
	)

	var studentID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO students (tenant_id, user_id, student_id_number, grade_level, status) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), 'active') RETURNING id`,
		tenantID, userID, strings.TrimSpace(req.StudentIDNumber), strings.TrimSpace(req.GradeLevel),
	).Scan(&studentID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create student profile", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not complete creation", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "students.create_full", "student", studentID, r)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       studentID,
		"userId":   userID,
		"email":    req.Email,
		"displayName": req.DisplayName,
		"status":   "active",
	})
}

// POST /api/v1/staff/create-full
func (a *App) handleCreateStaffFull(w http.ResponseWriter, r *http.Request) {
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
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		EmployeeID  string `json:"employeeId"`
		Department  string `json:"department"`
		Position    string `json:"position"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	fields := map[string]string{}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	if req.DisplayName == "" {
		fields["displayName"] = "Name is required"
	}
	if req.Email == "" {
		fields["email"] = "Email is required"
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

	var exists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
	if exists {
		writeValidationError(w, map[string]string{"email": "Email already in use"}, r)
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create staff", r)
		return
	}
	defer tx.Rollback()

	hash := hashPassword(req.Password)
	var userID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO users (email, display_name, status) VALUES ($1, $2, 'active') RETURNING id`,
		req.Email, req.DisplayName,
	).Scan(&userID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create user", r)
		return
	}

	_, _ = tx.ExecContext(r.Context(), `INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`, userID, hash)
	_, _ = tx.ExecContext(r.Context(), `INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary) VALUES ($1, $2, 'active', true)`, tenantID, userID)
	_, _ = tx.ExecContext(r.Context(),
		`INSERT INTO user_roles (tenant_id, user_id, role_id) SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = 'staff' ON CONFLICT DO NOTHING`,
		tenantID, userID,
	)

	var staffID string
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO staff_profiles (tenant_id, user_id, employee_id, department, position, status) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), 'active') RETURNING id`,
		tenantID, userID, strings.TrimSpace(req.EmployeeID), strings.TrimSpace(req.Department), strings.TrimSpace(req.Position),
	).Scan(&staffID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create staff profile", r)
		return
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not complete creation", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "staff.create_full", "staff", staffID, r)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       staffID,
		"userId":   userID,
		"email":    req.Email,
		"displayName": req.DisplayName,
		"status":   "active",
	})
}
