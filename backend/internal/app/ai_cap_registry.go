package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RegisterAllCapabilities registers all API capabilities grouped by domain
func (a *App) RegisterAllCapabilities(reg *CapabilityRegistry) {
	// ─── General / Stats ───
	reg.Register(Capability{
		Name:        "get_stats",
		Description: "Statistik sekolah: jumlah siswa, guru, kelas, mapel aktif",
		Permission:  "users:read",
		Risk:        "read",
		Domain:      "general",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, a.capGetStats)

	reg.Register(Capability{
		Name:        "get_active_academic_year",
		Description: "Tahun ajaran yang sedang aktif",
		Permission:  "academic:read",
		Risk:        "read",
		Domain:      "general",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, a.capGetAcademicYear)

	// ─── Students ───
	reg.Register(Capability{
		Name:        "search_students",
		Description: "Cari siswa berdasarkan nama, email, kelas, atau grade level",
		Permission:  "users:read",
		Risk:        "read",
		Domain:      "students",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string"},"gradeLevel":{"type":"string"},"classSectionId":{"type":"string"},"limit":{"type":"integer","default":10}}}`),
	}, a.capSearchStudents)

	reg.Register(Capability{
		Name:        "create_student",
		Description: "Buat siswa baru. Butuh: nama, email, password. Opsional: NIS, grade, kelas",
		Permission:  "users:write",
		Risk:        "write",
		Domain:      "students",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"displayName":{"type":"string"},"email":{"type":"string"},"password":{"type":"string"},"studentIdNumber":{"type":"string"},"gradeLevel":{"type":"string"},"classSectionId":{"type":"string"}},"required":["displayName","email","password"]}`),
	}, a.capCreateStudent)

	reg.Register(Capability{
		Name:        "archive_student",
		Description: "Archive (nonaktifkan) siswa",
		Permission:  "users:write",
		Risk:        "destructive",
		Domain:      "students",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"studentId":{"type":"string"},"search":{"type":"string","description":"Cari by nama jika ID tidak diketahui"}}}`),
	}, a.capArchiveStudent)

	reg.Register(Capability{
		Name:        "batch_archive_students",
		Description: "Archive banyak siswa sekaligus. Gunakan jika user sudah konfirmasi untuk archive multiple students.",
		Permission:  "users:write",
		Risk:        "destructive",
		Domain:      "students",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"studentNames":{"type":"array","items":{"type":"string"},"description":"Array nama siswa yang akan diarsipkan"}},"required":["studentNames"]}`),
	}, a.capBatchArchiveStudents)

	// ─── Teachers ───
	reg.Register(Capability{
		Name:        "search_teachers",
		Description: "Cari guru berdasarkan nama atau spesialisasi",
		Permission:  "users:read",
		Risk:        "read",
		Domain:      "teachers",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string"},"limit":{"type":"integer","default":10}}}`),
	}, a.capSearchTeachers)

	reg.Register(Capability{
		Name:        "create_teacher",
		Description: "Buat guru baru. Butuh: nama, email, password. Opsional: NIP, spesialisasi, subjectIds",
		Permission:  "users:write",
		Risk:        "write",
		Domain:      "teachers",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"displayName":{"type":"string"},"email":{"type":"string"},"password":{"type":"string"},"employeeId":{"type":"string"},"specialization":{"type":"string"},"subjectIds":{"type":"array","items":{"type":"string"}}},"required":["displayName","email","password"]}`),
	}, a.capCreateTeacher)

	reg.Register(Capability{
		Name:        "assign_subject_to_teacher",
		Description: "Assign mata pelajaran ke guru",
		Permission:  "academic:write",
		Risk:        "write",
		Domain:      "teachers",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"teacherId":{"type":"string"},"teacherSearch":{"type":"string"},"subjectId":{"type":"string"},"subjectSearch":{"type":"string"}}}`),
	}, a.capAssignSubjectToTeacher)

	// ─── Classes ───
	reg.Register(Capability{
		Name:        "search_classes",
		Description: "Cari kelas berdasarkan nama, tampilkan wali kelas dan jumlah siswa",
		Permission:  "academic:read",
		Risk:        "read",
		Domain:      "classes",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string"},"limit":{"type":"integer","default":20}}}`),
	}, a.capSearchClasses)

	reg.Register(Capability{
		Name:        "create_class",
		Description: "Buat kelas baru. Butuh: nama, gradeLevel, academicYearId",
		Permission:  "academic:write",
		Risk:        "write",
		Domain:      "classes",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"gradeLevel":{"type":"string"},"academicYearId":{"type":"string"},"homeroomTeacherId":{"type":"string"},"capacity":{"type":"integer"}},"required":["name","gradeLevel","academicYearId"]}`),
	}, a.capCreateClass)

	// ─── Subjects ───
	reg.Register(Capability{
		Name:        "search_subjects",
		Description: "Cari mata pelajaran berdasarkan nama atau kode",
		Permission:  "academic:read",
		Risk:        "read",
		Domain:      "subjects",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string"}}}`),
	}, a.capSearchSubjects)

	reg.Register(Capability{
		Name:        "create_subject",
		Description: "Buat mata pelajaran baru. Butuh: code, name",
		Permission:  "academic:write",
		Risk:        "write",
		Domain:      "subjects",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"code":{"type":"string"},"name":{"type":"string"},"description":{"type":"string"}},"required":["code","name"]}`),
	}, a.capCreateSubject)

	// ─── Academic ───
	reg.Register(Capability{
		Name:        "list_academic_years",
		Description: "Daftar semua tahun ajaran",
		Permission:  "academic:read",
		Risk:        "read",
		Domain:      "academic",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}, a.capListAcademicYears)

	reg.Register(Capability{
		Name:        "create_academic_year",
		Description: "Buat tahun ajaran baru. Butuh: code, name",
		Permission:  "academic:write",
		Risk:        "write",
		Domain:      "academic",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"code":{"type":"string"},"name":{"type":"string"},"startsOn":{"type":"string"},"endsOn":{"type":"string"}},"required":["code","name"]}`),
	}, a.capCreateAcademicYear)

	// ─── Staff ───
	reg.Register(Capability{
		Name:        "search_staff",
		Description: "Cari staff berdasarkan nama, department, atau posisi",
		Permission:  "users:read",
		Risk:        "read",
		Domain:      "staff",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string"},"limit":{"type":"integer","default":10}}}`),
	}, a.capSearchStaff)

	reg.Register(Capability{
		Name:        "create_staff",
		Description: "Buat staff baru. Butuh: nama, email, password",
		Permission:  "users:write",
		Risk:        "write",
		Domain:      "staff",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"displayName":{"type":"string"},"email":{"type":"string"},"password":{"type":"string"},"employeeId":{"type":"string"},"department":{"type":"string"},"position":{"type":"string"}},"required":["displayName","email","password"]}`),
	}, a.capCreateStaff)

	// ─── Stats (alias domain) ───
	reg.Register(Capability{
		Name:        "count_by_entity",
		Description: "Hitung jumlah entity tertentu (siswa, guru, kelas, mapel, staff)",
		Permission:  "users:read",
		Risk:        "read",
		Domain:      "stats",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"entity":{"type":"string","enum":["students","teachers","classes","subjects","staff","academic_years"]}},"required":["entity"]}`),
	}, a.capCountEntity)

	// ─── Tenants ───
	reg.Register(Capability{
		Name:        "list_tenants",
		Description: "Daftar semua sekolah/tenant yang terdaftar",
		Permission:  "tenants:read",
		Risk:        "read",
		Domain:      "tenants",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"search":{"type":"string"}}}`),
	}, a.capListTenants)

	// ─── Exams ───
	a.registerExamCapabilities(reg)

	// ─── Exams (extra: update / delete / sections / groups / move / stimuli) ───
	a.RegisterExamExtraCapabilities(reg)
	a.RegisterExamV2Capabilities(reg)
	a.RegisterCompoundCapabilities(reg)

	// ─── Blueprints (Phase 9.5) ───
	a.registerBlueprintCapabilities(reg)
}

// ─── Capability Handlers ───

func (a *App) capGetStats(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var students, teachers, classes, subjects, staff int
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM students WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&students)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM teachers WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&teachers)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM class_sections WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&classes)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subjects WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&subjects)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_profiles WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&staff)
	b, _ := json.Marshal(map[string]int{"siswa": students, "guru": teachers, "kelas": classes, "mata_pelajaran": subjects, "staff": staff})
	return string(b), nil
}

func (a *App) capGetAcademicYear(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolGetAcademicYear(ctx, tenantID, userID, args)
}

func (a *App) capSearchStudents(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolListStudents(ctx, tenantID, userID, args)
}

func (a *App) capCreateStudent(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolCreateStudent(ctx, tenantID, userID, args)
}

func (a *App) capArchiveStudent(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolArchiveStudent(ctx, tenantID, userID, args)
}

func (a *App) capSearchTeachers(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolListTeachers(ctx, tenantID, userID, args)
}

func (a *App) capCreateTeacher(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolCreateTeacher(ctx, tenantID, userID, args)
}

func (a *App) capAssignSubjectToTeacher(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolAssignTeacherSubject(ctx, tenantID, userID, args)
}

func (a *App) capSearchClasses(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolListClasses(ctx, tenantID, userID, args)
}

func (a *App) capCreateClass(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolCreateClass(ctx, tenantID, userID, args)
}

func (a *App) capSearchSubjects(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolListSubjects(ctx, tenantID, userID, args)
}

func (a *App) capCreateSubject(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	return a.toolCreateSubject(ctx, tenantID, userID, args)
}

func (a *App) capListAcademicYears(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT id, code, name, status FROM academic_years WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 10`, tenantID)
	if err != nil {
		return "[]", nil
	}
	defer rows.Close()
	type Row struct {
		ID     string `json:"id"`
		Code   string `json:"code"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	var results []Row
	for rows.Next() {
		var r Row
		if rows.Scan(&r.ID, &r.Code, &r.Name, &r.Status) == nil {
			results = append(results, r)
		}
	}
	if results == nil {
		results = []Row{}
	}
	b, _ := json.Marshal(results)
	return string(b), nil
}

func (a *App) capCreateAcademicYear(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		StartsOn string `json:"startsOn"`
		EndsOn   string `json:"endsOn"`
	}
	_ = json.Unmarshal(args, &p)
	if p.Code == "" || p.Name == "" {
		return errValidationFailed("code", "code and name are required"), nil
	}
	var id string
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO academic_years (tenant_id, code, name, starts_on, ends_on, status) VALUES ($1, $2, $3, NULLIF($4,'')::date, NULLIF($5,'')::date, 'active') RETURNING id`,
		tenantID, p.Code, p.Name, p.StartsOn, p.EndsOn,
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return errDuplicateEntry("academic_year", "code", p.Code), nil
		}
		return "", err
	}
	return fmt.Sprintf(`{"success":true,"message":"Tahun ajaran %s berhasil dibuat","id":"%s"}`, p.Name, id), nil
}

func (a *App) capSearchStaff(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		Search string `json:"search"`
		Limit  int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &params)
	if params.Limit <= 0 || params.Limit > 20 {
		params.Limit = 10
	}
	query := `SELECT u.display_name, u.email, sp.department, sp.position, sp.status FROM staff_profiles sp JOIN users u ON u.id = sp.user_id WHERE sp.tenant_id = $1 AND sp.status = 'active'`
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
		return "[]", nil
	}
	defer rows.Close()
	type Row struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Department string `json:"department,omitempty"`
		Position   string `json:"position,omitempty"`
	}
	var results []Row
	for rows.Next() {
		var r Row
		var dept, pos *string
		if rows.Scan(&r.Name, &r.Email, &dept, &pos, new(string)) == nil {
			if dept != nil {
				r.Department = *dept
			}
			if pos != nil {
				r.Position = *pos
			}
			results = append(results, r)
		}
	}
	if results == nil {
		results = []Row{}
	}
	b, _ := json.Marshal(results)
	return string(b), nil
}

func (a *App) capCreateStaff(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var params struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		EmployeeID  string `json:"employeeId"`
		Department  string `json:"department"`
		Position    string `json:"position"`
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

	confirmText := fmt.Sprintf("Buat staff baru: %s (%s)", params.DisplayName, params.Email)
	if params.Department != "" {
		confirmText += fmt.Sprintf(", departemen: %s", params.Department)
	}

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_staff", args, confirmText)
}

func (a *App) capCountEntity(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Entity string `json:"entity"`
	}
	_ = json.Unmarshal(args, &p)

	var count int
	var table string
	switch p.Entity {
	case "students":
		table = "students"
	case "teachers":
		table = "teachers"
	case "classes":
		table = "class_sections"
	case "subjects":
		table = "subjects"
	case "staff":
		table = "staff_profiles"
	case "academic_years":
		table = "academic_years"
	default:
		return errValidationFailed("entity", "Unknown entity. Use: students, teachers, classes, subjects, staff, academic_years"), nil
	}

	_ = a.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE tenant_id = $1 AND status = 'active'`, table), tenantID).Scan(&count)
	return fmt.Sprintf(`{"entity":"%s","count":%d}`, p.Entity, count), nil
}

func (a *App) capListTenants(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Search string `json:"search"`
	}
	_ = json.Unmarshal(args, &p)

	query := `SELECT name, code, status FROM tenants WHERE 1=1`
	qArgs := []any{}
	idx := 1
	if p.Search != "" {
		query += ` AND (name ILIKE $` + itoa(idx) + ` OR code ILIKE $` + itoa(idx) + `)`
		qArgs = append(qArgs, "%"+p.Search+"%")
		idx++
	}
	query += ` ORDER BY name LIMIT 20`

	rows, err := a.db.QueryContext(ctx, query, qArgs...)
	if err != nil {
		return "[]", nil
	}
	defer rows.Close()
	type Row struct {
		Name   string `json:"name"`
		Code   string `json:"code"`
		Status string `json:"status"`
	}
	var results []Row
	for rows.Next() {
		var r Row
		if rows.Scan(&r.Name, &r.Code, &r.Status) == nil {
			results = append(results, r)
		}
	}
	if results == nil {
		results = []Row{}
	}
	b, _ := json.Marshal(results)
	return string(b), nil
}

func (a *App) capBatchArchiveStudents(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		StudentNames []string `json:"studentNames"`
	}
	_ = json.Unmarshal(args, &p)

	if len(p.StudentNames) == 0 {
		return errValidationFailed("studentNames", "studentNames array is required and must not be empty"), nil
	}

	type FailedItem struct {
		Name        string   `json:"name"`
		Code        string   `json:"code"`
		Reason      string   `json:"reason"`
		Suggestions []string `json:"suggestions,omitempty"`
	}

	var archived []string
	var failed []FailedItem

	for _, name := range p.StudentNames {
		var studentID string
		err := a.db.QueryRowContext(ctx,
			`SELECT s.id FROM students s JOIN users u ON u.id = s.user_id WHERE s.tenant_id = $1 AND u.display_name ILIKE $2 AND s.status = 'active' LIMIT 1`,
			tenantID, "%"+name+"%",
		).Scan(&studentID)
		if err != nil {
			suggestions := a.findSimilarNames(ctx, tenantID, "students", name)
			failed = append(failed, FailedItem{
				Name:        name,
				Code:        "ENTITY_NOT_FOUND",
				Reason:      "Student not found by name (active only)",
				Suggestions: suggestions,
			})
			continue
		}
		_, err = a.db.ExecContext(ctx, `UPDATE students SET status = 'archived', updated_at = now() WHERE id = $1`, studentID)
		if err != nil {
			failed = append(failed, FailedItem{
				Name:   name,
				Code:   "ARCHIVE_FAILED",
				Reason: "Database error during archive",
			})
			continue
		}
		// Cascade: free the email slot when this was the user's last active profile.
		if uid, _ := userIDForProfile(ctx, a.db, "students", studentID); uid != "" {
			if _, err := cascadeArchiveUserIfOrphan(ctx, a.db, uid); err != nil {
				a.logger.Error("batch cascade archive user failed", "error", err)
			}
		}
		archived = append(archived, name)
	}

	result := map[string]any{
		"success":  true,
		"archived": archived,
		"failed":   failed,
	}
	switch {
	case len(failed) == 0:
		result["message"] = fmt.Sprintf("%d siswa berhasil diarsipkan", len(archived))
	case len(archived) == 0:
		result["success"] = false
		result["message"] = fmt.Sprintf("Tidak ada yang berhasil diarsipkan; %d gagal", len(failed))
		result["recovery"] = map[string]any{
			"hint": "Untuk setiap entry di 'failed', periksa 'suggestions' atau panggil search_students dengan nama yang benar, lalu retry batch_archive_students dengan nama yang sudah dikoreksi.",
		}
	default:
		result["message"] = fmt.Sprintf("%d siswa diarsipkan, %d gagal", len(archived), len(failed))
		result["recovery"] = map[string]any{
			"hint": "Periksa 'failed' array — koreksi nama berdasarkan suggestions, lalu retry hanya untuk yang gagal.",
		}
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}
