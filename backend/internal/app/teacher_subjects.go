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
		`SELECT s.id, s.code, s.name FROM teacher_subjects ts
		 JOIN subjects s ON s.id = ts.subject_id
		 WHERE ts.tenant_id = $1 AND ts.teacher_id = $2 AND ts.status = 'active'
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

	// Insert into teacher_subjects (simple direct mapping)
	_, err := a.db.ExecContext(r.Context(),
		`INSERT INTO teacher_subjects (tenant_id, teacher_id, subject_id, status)
		 VALUES ($1, $2, $3, 'active')
		 ON CONFLICT (tenant_id, teacher_id, subject_id) DO UPDATE SET status = 'active'`,
		tenantID, teacherID, req.SubjectID,
	)
	if err != nil {
		a.logger.Error("assign teacher subject failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "assign_failed", "Could not assign subject", r)
		return
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
		`UPDATE teacher_subjects SET status = 'archived' WHERE tenant_id = $1 AND teacher_id = $2 AND subject_id = $3`,
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
