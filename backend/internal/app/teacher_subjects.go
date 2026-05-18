package app

import (
	"net/http"
	"strings"
)

// Teacher-Subject assignments: assign/unassign subjects to teachers

func (a *App) registerTeacherSubjectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/teachers/{id}/subjects", a.handleListTeacherSubjects)
	mux.HandleFunc("POST /api/v1/teachers/{id}/subjects", a.handleAssignTeacherSubject)
	mux.HandleFunc("DELETE /api/v1/teachers/{id}/subjects/{subjectId}", a.handleUnassignTeacherSubject)
}

func (a *App) handleListTeacherSubjects(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}

	teacherID := r.PathValue("id")

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT s.id, s.code, s.name FROM teaching_assignments ta
		 JOIN subjects s ON s.id = ta.subject_id
		 WHERE ta.tenant_id = $1 AND ta.teacher_id = $2 AND ta.status = 'active'
		 ORDER BY s.name`,
		tenantID, teacherID,
	)
	if err != nil {
		a.logger.Error("list teacher subjects failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed", "Could not load subjects", r)
		return
	}
	defer rows.Close()

	type SubjectRef struct {
		ID   string `json:"id"`
		Code string `json:"code"`
		Name string `json:"name"`
	}

	var subjects []SubjectRef
	for rows.Next() {
		var s SubjectRef
		if err := rows.Scan(&s.ID, &s.Code, &s.Name); err != nil {
			continue
		}
		subjects = append(subjects, s)
	}
	if subjects == nil {
		subjects = []SubjectRef{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": subjects})
}

func (a *App) handleAssignTeacherSubject(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	teacherID := r.PathValue("id")

	// Verify teacher belongs to tenant
	var teacherExists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM teachers WHERE id = $1 AND tenant_id = $2)`, teacherID, tenantID).Scan(&teacherExists)
	if !teacherExists {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Teacher not found", r)
		return
	}

	var req struct {
		SubjectID string `json:"subjectId"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if strings.TrimSpace(req.SubjectID) == "" {
		writeValidationError(w, map[string]string{"subjectId": "Subject ID is required"}, r)
		return
	}

	// Verify subject belongs to tenant
	var subjectExists bool
	_ = a.db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM subjects WHERE id = $1 AND tenant_id = $2)`, req.SubjectID, tenantID).Scan(&subjectExists)
	if !subjectExists {
		writeValidationError(w, map[string]string{"subjectId": "Subject not found"}, r)
		return
	}

	// We need a dummy academic_year_id and class_section_id for the teaching_assignments table
	// For now, use a simplified assignment (subject-only, no class/year constraint)
	// Insert with ON CONFLICT to make it idempotent
	var assignmentID string
	err := a.db.QueryRowContext(r.Context(),
		`INSERT INTO teaching_assignments (tenant_id, teacher_id, subject_id, class_section_id, academic_year_id, status)
		 SELECT $1, $2, $3, cs.id, cs.academic_year_id, 'active'
		 FROM class_sections cs WHERE cs.tenant_id = $1 LIMIT 1
		 ON CONFLICT (tenant_id, teacher_id, subject_id, class_section_id, academic_year_id) DO NOTHING
		 RETURNING id`,
		tenantID, teacherID, req.SubjectID,
	).Scan(&assignmentID)

	// If no class sections exist yet, create a simpler record
	if err != nil {
		// Try direct insert without class_section constraint
		// For MVP: we'll track teacher-subject as a simple relation
		_, err = a.db.ExecContext(r.Context(),
			`INSERT INTO teaching_assignments (tenant_id, teacher_id, subject_id, class_section_id, academic_year_id, status)
			 VALUES ($1, $2, $3, (SELECT id FROM class_sections WHERE tenant_id = $1 LIMIT 1), (SELECT id FROM academic_years WHERE tenant_id = $1 LIMIT 1), 'active')
			 ON CONFLICT (tenant_id, teacher_id, subject_id, class_section_id, academic_year_id) DO NOTHING`,
			tenantID, teacherID, req.SubjectID,
		)
		if err != nil {
			a.logger.Error("assign teacher subject failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "assign_failed", "Could not assign subject. Ensure at least one class section and academic year exist.", r)
			return
		}
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "teachers.assign_subject", "teacher", teacherID, r)

	writeJSON(w, http.StatusCreated, map[string]any{"teacherId": teacherID, "subjectId": req.SubjectID})
}

func (a *App) handleUnassignTeacherSubject(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "academic:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}

	teacherID := r.PathValue("id")
	subjectID := r.PathValue("subjectId")

	_, err := a.db.ExecContext(r.Context(),
		`UPDATE teaching_assignments SET status = 'archived' WHERE tenant_id = $1 AND teacher_id = $2 AND subject_id = $3`,
		tenantID, teacherID, subjectID,
	)
	if err != nil {
		a.logger.Error("unassign teacher subject failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "unassign_failed", "Could not unassign subject", r)
		return
	}

	auth := AuthFromContext(r.Context())
	a.audit(r.Context(), &tenantID, auth.UserID, "teachers.unassign_subject", "teacher", teacherID, r)

	writeJSON(w, http.StatusOK, map[string]any{"status": "unassigned"})
}
