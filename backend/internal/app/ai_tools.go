package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AI Tool definitions and executor

type AITool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Permission  string          `json:"-"`
	Risk        string          `json:"-"` // low, medium, high
	Parameters  json.RawMessage `json:"parameters"`
}

type AIToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Args     string `json:"arguments"`
}

type AIToolResult struct {
	CallID  string `json:"tool_call_id"`
	Content string `json:"content"`
}

// ToolRegistry holds all available tools
type ToolRegistry struct {
	tools    []AITool
	handlers map[string]ToolHandler
}

type ToolHandler func(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error)

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: make(map[string]ToolHandler),
	}
}

func (r *ToolRegistry) Register(tool AITool, handler ToolHandler) {
	r.tools = append(r.tools, tool)
	r.handlers[tool.Name] = handler
}

// GetToolsForRole returns tool definitions filtered by user permissions
func (r *ToolRegistry) GetToolsForRole(permissions []string) []map[string]any {
	permSet := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		permSet[p] = true
	}

	var tools []map[string]any
	for _, t := range r.tools {
		if t.Permission == "" || permSet[t.Permission] {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  json.RawMessage(t.Parameters),
				},
			})
		}
	}
	return tools
}

// Execute runs a tool call and returns the result
func (r *ToolRegistry) Execute(ctx context.Context, tenantID, userID, toolName, args string) (AIToolResult, error) {
	handler, ok := r.handlers[toolName]
	if !ok {
		return AIToolResult{}, fmt.Errorf("unknown tool: %s", toolName)
	}
	result, err := handler(ctx, tenantID, userID, json.RawMessage(args))
	if err != nil {
		return AIToolResult{Content: fmt.Sprintf("Error: %s", err.Error())}, nil
	}
	return AIToolResult{Content: result}, nil
}

// RegisterSchoolTools registers all school-related tools
func (a *App) RegisterSchoolTools(registry *ToolRegistry) {
	// --- Read Tools ---

	registry.Register(AITool{
		Name:        "get_school_stats",
		Description: "Get overview statistics: total students, teachers, classes, subjects for the current school",
		Permission:  "users:read",
		Risk:        "low",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, a.toolGetSchoolStats)

	registry.Register(AITool{
		Name:        "list_students",
		Description: "Search and list students. Can filter by name, grade level, or class.",
		Permission:  "users:read",
		Risk:        "low",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string","description":"Search by name or email"},"gradeLevel":{"type":"string","description":"Filter by grade level"},"classSectionId":{"type":"string","description":"Filter by class section ID"},"limit":{"type":"integer","description":"Max results (default 10)"}}}`),
	}, a.toolListStudents)

	registry.Register(AITool{
		Name:        "list_teachers",
		Description: "Search and list teachers. Can filter by name or specialization.",
		Permission:  "users:read",
		Risk:        "low",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string","description":"Search by name or email"},"limit":{"type":"integer","description":"Max results (default 10)"}}}`),
	}, a.toolListTeachers)

	registry.Register(AITool{
		Name:        "list_classes",
		Description: "List class sections with their homeroom teacher and student count.",
		Permission:  "academic:read",
		Risk:        "low",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string","description":"Search by class name"},"limit":{"type":"integer","description":"Max results (default 20)"}}}`),
	}, a.toolListClasses)

	registry.Register(AITool{
		Name:        "list_subjects",
		Description: "List subjects available in the school.",
		Permission:  "academic:read",
		Risk:        "low",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string","description":"Search by subject name or code"}}}`),
	}, a.toolListSubjects)

	registry.Register(AITool{
		Name:        "get_academic_year",
		Description: "Get the current active academic year information.",
		Permission:  "academic:read",
		Risk:        "low",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, a.toolGetAcademicYear)
}

// --- Tool Handlers ---

func (a *App) toolGetSchoolStats(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var students, teachers, classes, subjects int
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM students WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&students)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM teachers WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&teachers)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM class_sections WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&classes)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subjects WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&subjects)

	result := map[string]int{"students": students, "teachers": teachers, "classes": classes, "subjects": subjects}
	b, _ := json.Marshal(result)
	return string(b), nil
}

func (a *App) toolListStudents(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		Search         string `json:"search"`
		GradeLevel     string `json:"gradeLevel"`
		ClassSectionID string `json:"classSectionId"`
		Limit          int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &params)
	if params.Limit <= 0 || params.Limit > 20 {
		params.Limit = 10
	}

	query := `SELECT s.id, u.display_name, u.email, s.student_id_number, s.grade_level, s.status FROM students s JOIN users u ON u.id = s.user_id WHERE s.tenant_id = $1 AND s.status = 'active'`
	qArgs := []any{tenantID}
	idx := 2

	if params.Search != "" {
		query += ` AND (u.display_name ILIKE $` + itoa(idx) + ` OR u.email ILIKE $` + itoa(idx) + `)`
		qArgs = append(qArgs, "%"+params.Search+"%")
		idx++
	}
	if params.GradeLevel != "" {
		query += ` AND s.grade_level = $` + itoa(idx)
		qArgs = append(qArgs, params.GradeLevel)
		idx++
	}
	if params.ClassSectionID != "" {
		query += ` AND s.class_section_id = $` + itoa(idx)
		qArgs = append(qArgs, params.ClassSectionID)
		idx++
	}
	query += ` ORDER BY u.display_name LIMIT $` + itoa(idx)
	qArgs = append(qArgs, params.Limit)

	rows, err := a.db.QueryContext(ctx, query, qArgs...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type StudentRow struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		StudentID string `json:"studentId,omitempty"`
		Grade     string `json:"grade,omitempty"`
		Status    string `json:"status"`
	}
	var results []StudentRow
	for rows.Next() {
		var r StudentRow
		var sid, grade *string
		if err := rows.Scan(&r.ID, &r.Name, &r.Email, &sid, &grade, &r.Status); err != nil {
			continue
		}
		if sid != nil { r.StudentID = *sid }
		if grade != nil { r.Grade = *grade }
		results = append(results, r)
	}
	if results == nil { results = []StudentRow{} }
	b, _ := json.Marshal(results)
	return string(b), nil
}

func (a *App) toolListTeachers(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		Search string `json:"search"`
		Limit  int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &params)
	if params.Limit <= 0 || params.Limit > 20 {
		params.Limit = 10
	}

	query := `SELECT t.id, u.display_name, u.email, t.specialization, t.status FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.tenant_id = $1 AND t.status = 'active'`
	qArgs := []any{tenantID}
	idx := 2

	if params.Search != "" {
		query += ` AND (u.display_name ILIKE $` + itoa(idx) + ` OR u.email ILIKE $` + itoa(idx) + `)`
		qArgs = append(qArgs, "%"+params.Search+"%")
		idx++
	}
	query += ` ORDER BY u.display_name LIMIT $` + itoa(idx)
	qArgs = append(qArgs, params.Limit)

	rows, err := a.db.QueryContext(ctx, query, qArgs...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type TeacherRow struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Email          string `json:"email"`
		Specialization string `json:"specialization,omitempty"`
		Status         string `json:"status"`
	}
	var results []TeacherRow
	for rows.Next() {
		var r TeacherRow
		var spec *string
		if err := rows.Scan(&r.ID, &r.Name, &r.Email, &spec, &r.Status); err != nil {
			continue
		}
		if spec != nil { r.Specialization = *spec }
		results = append(results, r)
	}
	if results == nil { results = []TeacherRow{} }
	b, _ := json.Marshal(results)
	return string(b), nil
}

func (a *App) toolListClasses(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		Search string `json:"search"`
		Limit  int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &params)
	if params.Limit <= 0 || params.Limit > 30 {
		params.Limit = 20
	}

	query := `SELECT cs.id, cs.name, cs.grade_level, COALESCE(u.display_name, ''), cs.capacity, cs.status,
		(SELECT COUNT(*) FROM students st WHERE st.class_section_id = cs.id AND st.status = 'active')
		FROM class_sections cs
		LEFT JOIN teachers t ON t.id = cs.homeroom_teacher_id
		LEFT JOIN users u ON u.id = t.user_id
		WHERE cs.tenant_id = $1 AND cs.status = 'active'`
	qArgs := []any{tenantID}
	idx := 2

	if params.Search != "" {
		query += ` AND cs.name ILIKE $` + itoa(idx)
		qArgs = append(qArgs, "%"+params.Search+"%")
		idx++
	}
	query += ` ORDER BY cs.name LIMIT $` + itoa(idx)
	qArgs = append(qArgs, params.Limit)

	rows, err := a.db.QueryContext(ctx, query, qArgs...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type ClassRow struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		GradeLevel      string `json:"gradeLevel"`
		HomeroomTeacher string `json:"homeroomTeacher,omitempty"`
		Capacity        int    `json:"capacity"`
		StudentCount    int    `json:"studentCount"`
		Status          string `json:"status"`
	}
	var results []ClassRow
	for rows.Next() {
		var r ClassRow
		var cap *int
		if err := rows.Scan(&r.ID, &r.Name, &r.GradeLevel, &r.HomeroomTeacher, &cap, &r.Status, &r.StudentCount); err != nil {
			continue
		}
		if cap != nil { r.Capacity = *cap }
		results = append(results, r)
	}
	if results == nil { results = []ClassRow{} }
	b, _ := json.Marshal(results)
	return string(b), nil
}

func (a *App) toolListSubjects(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		Search string `json:"search"`
	}
	_ = json.Unmarshal(args, &params)

	query := `SELECT id, code, name, COALESCE(description, '') FROM subjects WHERE tenant_id = $1 AND status = 'active'`
	qArgs := []any{tenantID}
	idx := 2

	if params.Search != "" {
		query += ` AND (name ILIKE $` + itoa(idx) + ` OR code ILIKE $` + itoa(idx) + `)`
		qArgs = append(qArgs, "%"+params.Search+"%")
		idx++
	}
	query += ` ORDER BY name`

	rows, err := a.db.QueryContext(ctx, query, qArgs...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type SubjectRow struct {
		ID          string `json:"id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	var results []SubjectRow
	for rows.Next() {
		var r SubjectRow
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Description); err != nil {
			continue
		}
		results = append(results, r)
	}
	if results == nil { results = []SubjectRow{} }
	b, _ := json.Marshal(results)
	return string(b), nil
}

func (a *App) toolGetAcademicYear(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var id, name, code, status string
	var startsOn, endsOn *string
	err := a.db.QueryRowContext(ctx,
		`SELECT id, name, code, status, starts_on::text, ends_on::text FROM academic_years WHERE tenant_id = $1 AND status = 'active' ORDER BY created_at DESC LIMIT 1`,
		tenantID,
	).Scan(&id, &name, &code, &status, &startsOn, &endsOn)
	if err != nil {
		return `{"message":"No active academic year found. Create one first using create_academic_year."}`, nil
	}

	result := map[string]any{"id": id, "name": name, "code": code, "status": status}
	if startsOn != nil { result["startsOn"] = strings.Split(*startsOn, " ")[0] }
	if endsOn != nil { result["endsOn"] = strings.Split(*endsOn, " ")[0] }
	b, _ := json.Marshal(result)
	return string(b), nil
}
