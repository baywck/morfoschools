package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AI Write Tools — medium/high risk operations that require confirmation

// ActionProposal represents a proposed write action awaiting confirmation
type ActionProposal struct {
	ID               string `json:"proposalId"`
	ToolName         string `json:"toolName"`
	ConfirmationText string `json:"confirmationText"`
	ExpiresAt        string `json:"expiresAt"`
}

// RegisterWriteTools adds write tools to the registry
func (a *App) RegisterWriteTools(registry *ToolRegistry) {
	registry.Register(AITool{
		Name:        "create_student",
		Description: "Create a new student. Requires: displayName, email, password. Optional: studentIdNumber, gradeLevel, classSectionId.",
		Permission:  "users:write",
		Risk:        "medium",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"displayName":{"type":"string","description":"Student full name"},"email":{"type":"string","description":"Student email"},"password":{"type":"string","description":"Login password (min 6 chars)"},"studentIdNumber":{"type":"string","description":"Student ID number"},"gradeLevel":{"type":"string","description":"Grade level (e.g. 10, 11, 12)"},"classSectionId":{"type":"string","description":"Class section UUID to enroll in"}},"required":["displayName","email","password"]}`),
	}, a.toolCreateStudent)

	registry.Register(AITool{
		Name:        "create_teacher",
		Description: "Create a new teacher. Requires: displayName, email, password. Optional: employeeId, specialization, subjectIds.",
		Permission:  "users:write",
		Risk:        "medium",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"displayName":{"type":"string","description":"Teacher full name"},"email":{"type":"string","description":"Teacher email"},"password":{"type":"string","description":"Login password (min 6 chars)"},"employeeId":{"type":"string","description":"Employee ID"},"specialization":{"type":"string","description":"Teaching specialization"},"subjectIds":{"type":"array","items":{"type":"string"},"description":"Subject UUIDs to assign"}},"required":["displayName","email","password"]}`),
	}, a.toolCreateTeacher)

	registry.Register(AITool{
		Name:        "create_class",
		Description: "Create a new class section. Requires: name, gradeLevel, academicYearId. Optional: homeroomTeacherId, capacity.",
		Permission:  "academic:write",
		Risk:        "medium",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Class name (e.g. X-IPA-1)"},"gradeLevel":{"type":"string","description":"Grade level"},"academicYearId":{"type":"string","description":"Academic year UUID"},"homeroomTeacherId":{"type":"string","description":"Homeroom teacher UUID"},"capacity":{"type":"integer","description":"Max students"}},"required":["name","gradeLevel","academicYearId"]}`),
	}, a.toolCreateClass)

	registry.Register(AITool{
		Name:        "create_subject",
		Description: "Create a new subject. Requires: code, name. Optional: description.",
		Permission:  "academic:write",
		Risk:        "medium",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"code":{"type":"string","description":"Subject code (e.g. MATH-101)"},"name":{"type":"string","description":"Subject name"},"description":{"type":"string","description":"Subject description"}},"required":["code","name"]}`),
	}, a.toolCreateSubject)

	registry.Register(AITool{
		Name:        "archive_student",
		Description: "Archive (soft-delete) a student by name or ID. This is a destructive action.",
		Permission:  "users:write",
		Risk:        "high",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"studentName":{"type":"string","description":"Student name to search and archive"},"studentId":{"type":"string","description":"Student UUID if known"}}}`),
	}, a.toolArchiveStudent)

	registry.Register(AITool{
		Name:        "assign_teacher_subject",
		Description: "Assign a subject to a teacher.",
		Permission:  "academic:write",
		Risk:        "medium",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"teacherName":{"type":"string","description":"Teacher name to search"},"teacherId":{"type":"string","description":"Teacher UUID if known"},"subjectName":{"type":"string","description":"Subject name to search"},"subjectId":{"type":"string","description":"Subject UUID if known"}}}`),
	}, a.toolAssignTeacherSubject)
}

// --- Write Tool Handlers (return proposals, not direct execution) ---

func (a *App) createProposal(ctx context.Context, sessionID, tenantID, userID, toolName string, args json.RawMessage, confirmText string) (string, error) {
	var proposalID string
	expiresAt := time.Now().Add(5 * time.Minute)
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO ai_pending_actions (session_id, tenant_id, user_id, tool_name, tool_args, confirmation_text, expires_at)
		 VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7) RETURNING id`,
		sessionID, tenantID, userID, toolName, args, confirmText, expiresAt,
	).Scan(&proposalID)
	if err != nil {
		return "", err
	}

	result := ActionProposal{
		ID:               proposalID,
		ToolName:         toolName,
		ConfirmationText: confirmText,
		ExpiresAt:        expiresAt.Format(time.RFC3339),
	}
	b, _ := json.Marshal(map[string]any{"type": "confirmation_required", "proposal": result})
	return string(b), nil
}

// sessionIDFromContext extracts session ID — stored in context by the handler
type ctxKeySessionID struct{}

func (a *App) toolCreateStudent(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		DisplayName     string `json:"displayName"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		StudentIDNumber string `json:"studentIdNumber"`
		GradeLevel      string `json:"gradeLevel"`
		ClassSectionID  string `json:"classSectionId"`
	}
	_ = json.Unmarshal(args, &params)

	if params.DisplayName == "" {
		return errValidationFailed("displayName", "displayName is required"), nil
	}
	if params.Email == "" {
		return errValidationFailed("email", "email is required"), nil
	}
	if params.Password == "" {
		return errValidationFailed("password", "password is required (min 6 chars)"), nil
	}

	// Validate classSectionId if provided
	if params.ClassSectionID != "" && !isUUID(params.ClassSectionID) {
		return errInvalidUUID("classSectionId", params.ClassSectionID, "class"), nil
	}

	// Pre-propose duplicate guard: catch collisions before the user is asked
	// to confirm. Without this, the bot proposes plausible names/emails in a
	// batch and the user only finds out at execute time that some already
	// exist. Cheaper to fail here and let the bot pick a fresh value.
	if dup := a.checkStudentDuplicate(ctx, tenantID, params.Email, params.StudentIDNumber, params.DisplayName); dup != "" {
		return dup, nil
	}

	confirmText := fmt.Sprintf("Buat siswa baru: %s (%s)", params.DisplayName, params.Email)
	if params.GradeLevel != "" {
		confirmText += fmt.Sprintf(", kelas %s", params.GradeLevel)
	}

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_student", args, confirmText)
}

func (a *App) toolCreateTeacher(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		DisplayName    string   `json:"displayName"`
		Email          string   `json:"email"`
		Password       string   `json:"password"`
		EmployeeID     string   `json:"employeeId"`
		Specialization string   `json:"specialization"`
		SubjectIDs     []string `json:"subjectIds"`
	}
	_ = json.Unmarshal(args, &params)

	if params.DisplayName == "" {
		return errValidationFailed("displayName", "displayName is required"), nil
	}
	if params.Email == "" {
		return errValidationFailed("email", "email is required"), nil
	}
	if params.Password == "" {
		return errValidationFailed("password", "password is required (min 6 chars)"), nil
	}

	// Validate subjectIds if provided
	for _, sid := range params.SubjectIDs {
		if !isUUID(sid) {
			return errInvalidUUID("subjectIds", sid, "subject"), nil
		}
	}

	// Pre-propose duplicate guard.
	if dup := a.checkTeacherDuplicate(ctx, tenantID, params.Email, params.EmployeeID, params.DisplayName); dup != "" {
		return dup, nil
	}

	confirmText := fmt.Sprintf("Buat guru baru: %s (%s)", params.DisplayName, params.Email)
	if params.Specialization != "" {
		confirmText += fmt.Sprintf(", spesialisasi: %s", params.Specialization)
	}

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_teacher", args, confirmText)
}

func (a *App) toolCreateClass(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		Name              string `json:"name"`
		GradeLevel        string `json:"gradeLevel"`
		AcademicYearID    string `json:"academicYearId"`
		HomeroomTeacherID string `json:"homeroomTeacherId"`
		Capacity          int    `json:"capacity"`
	}
	_ = json.Unmarshal(args, &params)

	if params.Name == "" {
		return errValidationFailed("name", "class name is required"), nil
	}
	if params.GradeLevel == "" {
		return errValidationFailed("gradeLevel", "gradeLevel is required (e.g. SMA-10, SMP-7)"), nil
	}
	if params.AcademicYearID == "" {
		return errValidationFailed("academicYearId", "academicYearId is required. Use get_active_academic_year or list_academic_years to find it"), nil
	}
	if params.AcademicYearID != "" && !isUUID(params.AcademicYearID) {
		return errInvalidUUID("academicYearId", params.AcademicYearID, "academic_year"), nil
	}
	if params.HomeroomTeacherID != "" && !isUUID(params.HomeroomTeacherID) {
		return errInvalidUUID("homeroomTeacherId", params.HomeroomTeacherID, "teacher"), nil
	}

	// Pre-propose duplicate guard: a class with the same (academicYearId, name)
	// will fail the unique constraint at execute time, so catch it here.
	if dup := a.checkClassDuplicate(ctx, tenantID, params.AcademicYearID, params.Name); dup != "" {
		return dup, nil
	}

	confirmText := fmt.Sprintf("Buat kelas baru: %s (grade %s)", params.Name, params.GradeLevel)

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_class", args, confirmText)
}

func (a *App) toolCreateSubject(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(args, &params)

	if params.Code == "" {
		return errValidationFailed("code", "subject code is required (e.g. MATH-101)"), nil
	}
	if params.Name == "" {
		return errValidationFailed("name", "subject name is required"), nil
	}

	// Pre-propose duplicate guard.
	if dup := a.checkSubjectDuplicate(ctx, tenantID, params.Code, params.Name); dup != "" {
		return dup, nil
	}

	confirmText := fmt.Sprintf("Buat mata pelajaran baru: %s (%s)", params.Name, params.Code)

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_subject", args, confirmText)
}

func (a *App) toolArchiveStudent(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		StudentName string `json:"studentName"`
		StudentID   string `json:"studentId"`
		Search      string `json:"search"`
	}
	_ = json.Unmarshal(args, &params)

	// Resolve student
	var studentID, displayName string
	if params.StudentID != "" {
		if !isUUID(params.StudentID) {
			return errInvalidUUID("studentId", params.StudentID, "student"), nil
		}
		studentID = params.StudentID
		err := a.db.QueryRowContext(ctx,
			`SELECT u.display_name FROM students s JOIN users u ON u.id = s.user_id WHERE s.id = $1 AND s.tenant_id = $2 AND s.status = 'active'`,
			studentID, tenantID,
		).Scan(&displayName)
		if err != nil {
			return errEntityNotFound("student", "studentId", params.StudentID), nil
		}
	} else {
		searchTerm := params.StudentName
		if searchTerm == "" {
			searchTerm = params.Search
		}
		if searchTerm == "" {
			return errValidationFailed("studentName", "studentName or studentId is required"), nil
		}
		err := a.db.QueryRowContext(ctx,
			`SELECT s.id, u.display_name FROM students s JOIN users u ON u.id = s.user_id WHERE s.tenant_id = $1 AND u.display_name ILIKE $2 AND s.status = 'active' LIMIT 1`,
			tenantID, "%"+searchTerm+"%",
		).Scan(&studentID, &displayName)
		if err != nil {
			// Try to find similar names for suggestions
			suggestions := a.findSimilarNames(ctx, tenantID, "students", searchTerm)
			if len(suggestions) > 0 {
				return errWithSuggestions("student", "studentName", searchTerm, suggestions), nil
			}
			return errEntityNotFound("student", "studentName", searchTerm), nil
		}
	}

	if studentID == "" {
		return errEntityNotFound("student", "studentName", params.StudentName), nil
	}

	confirmText := fmt.Sprintf("⚠️ Archive siswa: %s. Akses akan dinonaktifkan.", displayName)
	argsWithID, _ := json.Marshal(map[string]string{"studentId": studentID, "displayName": displayName})

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "archive_student", argsWithID, confirmText)
}

func (a *App) toolAssignTeacherSubject(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		TeacherName   string `json:"teacherName"`
		TeacherID     string `json:"teacherId"`
		TeacherSearch string `json:"teacherSearch"`
		SubjectName   string `json:"subjectName"`
		SubjectID     string `json:"subjectId"`
		SubjectSearch string `json:"subjectSearch"`
	}
	_ = json.Unmarshal(args, &params)

	// Resolve teacher
	var teacherID, teacherName string
	if params.TeacherID != "" {
		if !isUUID(params.TeacherID) {
			return errInvalidUUID("teacherId", params.TeacherID, "teacher"), nil
		}
		teacherID = params.TeacherID
		err := a.db.QueryRowContext(ctx,
			`SELECT u.display_name FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.id = $1 AND t.tenant_id = $2`,
			teacherID, tenantID,
		).Scan(&teacherName)
		if err != nil {
			return errEntityNotFound("teacher", "teacherId", params.TeacherID), nil
		}
	} else {
		searchTerm := params.TeacherName
		if searchTerm == "" {
			searchTerm = params.TeacherSearch
		}
		if searchTerm == "" {
			return errValidationFailed("teacherName", "teacherName or teacherId is required"), nil
		}
		err := a.db.QueryRowContext(ctx,
			`SELECT t.id, u.display_name FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.tenant_id = $1 AND u.display_name ILIKE $2 AND t.status = 'active' LIMIT 1`,
			tenantID, "%"+searchTerm+"%",
		).Scan(&teacherID, &teacherName)
		if err != nil {
			suggestions := a.findSimilarNames(ctx, tenantID, "teachers", searchTerm)
			if len(suggestions) > 0 {
				return errWithSuggestions("teacher", "teacherName", searchTerm, suggestions), nil
			}
			return errEntityNotFound("teacher", "teacherName", searchTerm), nil
		}
	}

	// Resolve subject
	var subjectID, subjectName string
	if params.SubjectID != "" {
		if !isUUID(params.SubjectID) {
			return errInvalidUUID("subjectId", params.SubjectID, "subject"), nil
		}
		subjectID = params.SubjectID
		err := a.db.QueryRowContext(ctx, `SELECT name FROM subjects WHERE id = $1 AND tenant_id = $2`, subjectID, tenantID).Scan(&subjectName)
		if err != nil {
			return errEntityNotFound("subject", "subjectId", params.SubjectID), nil
		}
	} else {
		searchTerm := params.SubjectName
		if searchTerm == "" {
			searchTerm = params.SubjectSearch
		}
		if searchTerm == "" {
			return errValidationFailed("subjectName", "subjectName or subjectId is required"), nil
		}
		err := a.db.QueryRowContext(ctx,
			`SELECT id, name FROM subjects WHERE tenant_id = $1 AND (name ILIKE $2 OR code ILIKE $2) AND status = 'active' LIMIT 1`,
			tenantID, "%"+searchTerm+"%",
		).Scan(&subjectID, &subjectName)
		if err != nil {
			suggestions := a.findSimilarSubjects(ctx, tenantID, searchTerm)
			if len(suggestions) > 0 {
				return errWithSuggestions("subject", "subjectName", searchTerm, suggestions), nil
			}
			return errEntityNotFound("subject", "subjectName", searchTerm), nil
		}
	}

	confirmText := fmt.Sprintf("Assign %s sebagai pengajar %s", teacherName, subjectName)
	resolvedArgs, _ := json.Marshal(map[string]string{"teacherId": teacherID, "subjectId": subjectID})

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "assign_teacher_subject", resolvedArgs, confirmText)
}

// --- Action Executor (called after user confirms) ---

func (a *App) executeConfirmedAction(ctx context.Context, tenantID, userID, toolName string, args json.RawMessage) (string, error) {
	switch toolName {
	case "create_student":
		return a.execCreateStudent(ctx, tenantID, args)
	case "create_teacher":
		return a.execCreateTeacher(ctx, tenantID, args)
	case "create_staff":
		return a.execCreateStaff(ctx, tenantID, args)
	case "create_class":
		return a.execCreateClass(ctx, tenantID, args)
	case "create_subject":
		return a.execCreateSubject(ctx, tenantID, args)
	case "archive_student":
		return a.execArchiveStudent(ctx, tenantID, args)
	case "assign_teacher_subject":
		return a.execAssignTeacherSubject(ctx, tenantID, args)
	case "create_exam":
		return a.execCreateExam(ctx, tenantID, userID, args)
	case "create_question":
		return a.execCreateQuestion(ctx, tenantID, userID, args)
	case "batch_create_questions":
		return a.execBatchCreateQuestions(ctx, tenantID, userID, args)
	case "create_blueprint_template":
		return a.execCreateBlueprintTemplate(ctx, tenantID, userID, args)
	case "add_blueprint_slot":
		return a.execAddBlueprintSlot(ctx, tenantID, userID, args)
	case "bulk_add_blueprint_slots":
		return a.execBulkAddBlueprintSlots(ctx, tenantID, userID, args)
	case "clone_blueprint_to_exam":
		return a.execCloneBlueprintToExam(ctx, tenantID, userID, args)
	case "generate_question_for_slot":
		return a.execGenerateQuestionForSlot(ctx, tenantID, userID, args)
	case "apply_blueprint_analysis":
		return a.execApplyBlueprintAnalysis(ctx, tenantID, userID, args)
	case "set_uses_kisi_kisi":
		return a.execSetUsesKisiKisi(ctx, tenantID, userID, args)
	case "convert_questions_to_kisi_kisi":
		return a.execConvertQuestionsToKisiKisi(ctx, tenantID, userID, args)
	case "update_question":
		return a.execUpdateQuestion(ctx, tenantID, userID, args)
	case "delete_question":
		return a.execDeleteQuestion(ctx, tenantID, userID, args)
	case "create_exam_section":
		return a.execCreateExamSection(ctx, tenantID, userID, args)
	case "create_question_group":
		return a.execCreateQuestionGroup(ctx, tenantID, userID, args)
	case "create_stimulus":
		return a.execCreateStimulus(ctx, tenantID, userID, args)
	case "move_question":
		return a.execMoveQuestion(ctx, tenantID, userID, args)
	case "update_exam":
		return a.execUpdateExam(ctx, tenantID, userID, args)
	case "publish_exam":
		return a.execPublishExam(ctx, tenantID, userID, args)
	case "update_exam_section":
		return a.execUpdateExamSection(ctx, tenantID, userID, args)
	case "delete_exam_section":
		return a.execDeleteExamSection(ctx, tenantID, userID, args)
	case "update_question_group":
		return a.execUpdateQuestionGroup(ctx, tenantID, userID, args)
	case "delete_question_group":
		return a.execDeleteQuestionGroup(ctx, tenantID, userID, args)
	case "update_stimulus":
		return a.execUpdateStimulus(ctx, tenantID, userID, args)
	case "archive_stimulus":
		return a.execArchiveStimulus(ctx, tenantID, userID, args)
	case "promote_stimulus":
		return a.execPromoteStimulus(ctx, tenantID, userID, args)
	case "create_exam_gate":
		return a.execCreateExamGate(ctx, tenantID, userID, args)
	case "update_exam_gate":
		return a.execUpdateExamGate(ctx, tenantID, userID, args)
	case "delete_exam_gate":
		return a.execDeleteExamGate(ctx, tenantID, userID, args)
	case "assign_question_to_slot":
		return a.execAssignQuestionToSlot(ctx, tenantID, userID, args)
	case "export_exam_to_template":
		return a.execExportExamToTemplate(ctx, tenantID, userID, args)
	default:
		return "", fmt.Errorf("unknown action: %s", toolName)
	}
}

func (a *App) execCreateStudent(ctx context.Context, tenantID string, args json.RawMessage) (string, error) {
	var p struct {
		DisplayName     string `json:"displayName"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		StudentIDNumber string `json:"studentIdNumber"`
		GradeLevel      string `json:"gradeLevel"`
		ClassSectionID  string `json:"classSectionId"`
	}
	_ = json.Unmarshal(args, &p)

	// Resolve classSectionId if it's a name instead of UUID
	if p.ClassSectionID != "" && !isUUID(p.ClassSectionID) {
		var resolved string
		_ = a.db.QueryRowContext(ctx,
			`SELECT id FROM class_sections WHERE tenant_id = $1 AND (name ILIKE $2 OR name ILIKE $3) AND status = 'active' LIMIT 1`,
			tenantID, p.ClassSectionID, "%"+p.ClassSectionID+"%",
		).Scan(&resolved)
		if resolved != "" {
			p.ClassSectionID = resolved
		} else {
			p.ClassSectionID = ""
		}
	}

	hash := hashPassword(p.Password)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var userID string
	err = tx.QueryRowContext(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1, $2, 'active') RETURNING id`, p.Email, p.DisplayName).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return errDuplicateEntry("student", "email", p.Email), nil
		}
		return "", err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`, userID, hash)
	_, _ = tx.ExecContext(ctx, `INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary) VALUES ($1, $2, 'active', true)`, tenantID, userID)
	_, _ = tx.ExecContext(ctx, `INSERT INTO user_roles (tenant_id, user_id, role_id) SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = 'student' ON CONFLICT DO NOTHING`, tenantID, userID)

	var studentID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO students (tenant_id, user_id, student_id_number, grade_level, class_section_id, status) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,'')::uuid, 'active') RETURNING id`,
		tenantID, userID, p.StudentIDNumber, p.GradeLevel, p.ClassSectionID,
	).Scan(&studentID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"success":true,"message":"Siswa %s berhasil dibuat","studentId":"%s"}`, p.DisplayName, studentID), nil
}

func (a *App) execCreateTeacher(ctx context.Context, tenantID string, args json.RawMessage) (string, error) {
	var p struct {
		DisplayName    string   `json:"displayName"`
		Email          string   `json:"email"`
		Password       string   `json:"password"`
		EmployeeID     string   `json:"employeeId"`
		Specialization string   `json:"specialization"`
		SubjectIDs     []string `json:"subjectIds"`
	}
	_ = json.Unmarshal(args, &p)

	hash := hashPassword(p.Password)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var userID string
	err = tx.QueryRowContext(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1, $2, 'active') RETURNING id`, p.Email, p.DisplayName).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return errDuplicateEntry("teacher", "email", p.Email), nil
		}
		return "", err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`, userID, hash)
	_, _ = tx.ExecContext(ctx, `INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary) VALUES ($1, $2, 'active', true)`, tenantID, userID)
	_, _ = tx.ExecContext(ctx, `INSERT INTO user_roles (tenant_id, user_id, role_id) SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = 'teacher' ON CONFLICT DO NOTHING`, tenantID, userID)

	var teacherID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO teachers (tenant_id, user_id, employee_id, specialization, status) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), 'active') RETURNING id`,
		tenantID, userID, p.EmployeeID, p.Specialization,
	).Scan(&teacherID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	// Assign subjects (non-critical)
	for _, subjectID := range p.SubjectIDs {
		_, _ = a.db.ExecContext(ctx,
			`INSERT INTO teacher_subjects (tenant_id, teacher_id, subject_id, status) VALUES ($1, $2, $3, 'active') ON CONFLICT (tenant_id, teacher_id, subject_id) DO UPDATE SET status = 'active'`,
			tenantID, teacherID, subjectID,
		)
	}

	return fmt.Sprintf(`{"success":true,"message":"Guru %s berhasil dibuat","teacherId":"%s"}`, p.DisplayName, teacherID), nil
}

func (a *App) execCreateClass(ctx context.Context, tenantID string, args json.RawMessage) (string, error) {
	var p struct {
		Name              string `json:"name"`
		GradeLevel        string `json:"gradeLevel"`
		AcademicYearID    string `json:"academicYearId"`
		HomeroomTeacherID string `json:"homeroomTeacherId"`
		Capacity          int    `json:"capacity"`
	}
	_ = json.Unmarshal(args, &p)

	var classID string
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO class_sections (tenant_id, name, grade_level, academic_year_id, homeroom_teacher_id, capacity, status)
		 VALUES ($1, $2, $3, $4, NULLIF($5,'')::uuid, NULLIF($6, 0), 'active') RETURNING id`,
		tenantID, p.Name, p.GradeLevel, p.AcademicYearID, p.HomeroomTeacherID, p.Capacity,
	).Scan(&classID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"success":true,"message":"Kelas %s berhasil dibuat","classId":"%s"}`, p.Name, classID), nil
}

func (a *App) execCreateSubject(ctx context.Context, tenantID string, args json.RawMessage) (string, error) {
	var p struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(args, &p)

	var subjectID string
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO subjects (tenant_id, code, name, description, status) VALUES ($1, $2, $3, NULLIF($4,''), 'active') RETURNING id`,
		tenantID, p.Code, p.Name, p.Description,
	).Scan(&subjectID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return errDuplicateEntry("subject", "code", p.Code), nil
		}
		return "", err
	}
	return fmt.Sprintf(`{"success":true,"message":"Mata pelajaran %s berhasil dibuat","subjectId":"%s"}`, p.Name, subjectID), nil
}

func (a *App) execCreateStaff(ctx context.Context, tenantID string, args json.RawMessage) (string, error) {
	var p struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		EmployeeID  string `json:"employeeId"`
		Department  string `json:"department"`
		Position    string `json:"position"`
	}
	_ = json.Unmarshal(args, &p)

	hash := hashPassword(p.Password)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var userID string
	err = tx.QueryRowContext(ctx, `INSERT INTO users (email, display_name, status) VALUES ($1, $2, 'active') RETURNING id`, p.Email, p.DisplayName).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return errDuplicateEntry("staff", "email", p.Email), nil
		}
		return "", err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`, userID, hash)
	_, _ = tx.ExecContext(ctx, `INSERT INTO tenant_memberships (tenant_id, user_id, status, is_primary) VALUES ($1, $2, 'active', true)`, tenantID, userID)
	_, _ = tx.ExecContext(ctx, `INSERT INTO user_roles (tenant_id, user_id, role_id) SELECT $1, $2, id FROM roles WHERE tenant_id = $1 AND slug = 'staff' ON CONFLICT DO NOTHING`, tenantID, userID)

	var staffID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO staff_profiles (tenant_id, user_id, employee_id, department, position, status) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), 'active') RETURNING id`,
		tenantID, userID, p.EmployeeID, p.Department, p.Position,
	).Scan(&staffID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"success":true,"message":"Staff %s berhasil dibuat","staffId":"%s"}`, p.DisplayName, staffID), nil
}

func (a *App) execArchiveStudent(ctx context.Context, tenantID string, args json.RawMessage) (string, error) {
	var p struct {
		StudentID   string `json:"studentId"`
		DisplayName string `json:"displayName"`
	}
	_ = json.Unmarshal(args, &p)

	_, err := a.db.ExecContext(ctx, `UPDATE students SET status = 'archived', updated_at = now() WHERE id = $1 AND tenant_id = $2`, p.StudentID, tenantID)
	if err != nil {
		return "", err
	}
	// Cascade: free the email slot when this was the user's last active profile.
	if uid, _ := userIDForProfile(ctx, a.db, "students", p.StudentID); uid != "" {
		if _, err := cascadeArchiveUserIfOrphan(ctx, a.db, uid); err != nil {
			a.logger.Error("ai cascade archive user failed", "error", err)
		}
	}
	return fmt.Sprintf(`{"success":true,"message":"Siswa %s berhasil diarchive"}`, p.DisplayName), nil
}

func (a *App) execAssignTeacherSubject(ctx context.Context, tenantID string, args json.RawMessage) (string, error) {
	var p struct {
		TeacherID string `json:"teacherId"`
		SubjectID string `json:"subjectId"`
	}
	_ = json.Unmarshal(args, &p)

	_, err := a.db.ExecContext(ctx,
		`INSERT INTO teacher_subjects (tenant_id, teacher_id, subject_id, status) VALUES ($1, $2, $3, 'active') ON CONFLICT (tenant_id, teacher_id, subject_id) DO UPDATE SET status = 'active'`,
		tenantID, p.TeacherID, p.SubjectID,
	)
	if err != nil {
		return "", err
	}
	return `{"success":true,"message":"Subject berhasil di-assign ke guru"}`, nil
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
