package app

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type examCurriculumContextResponse struct {
	Status      string          `json:"status"`
	Source      string          `json:"source"`
	SubjectID   *string         `json:"subjectId,omitempty"`
	SubjectName string          `json:"subjectName,omitempty"`
	SubjectCode string          `json:"subjectCode,omitempty"`
	GradeLevel  string          `json:"gradeLevel,omitempty"`
	LevelCode   string          `json:"levelCode,omitempty"`
	Phase       string          `json:"phase,omitempty"`
	Reference   *cpReferenceRow `json:"reference,omitempty"`
	Elements    []cpElementRow  `json:"elements"`
	Warnings    []string        `json:"warnings"`
}

func (a *App) registerExamCurriculumContextRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/exams/{id}/curriculum-context", a.handleGetExamCurriculumContext)
}

func (a *App) handleGetExamCurriculumContext(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionRead) {
		return
	}
	resp, err := a.ensureExamCurriculumContext(r.Context(), tenantID, examID)
	if err != nil {
		a.logger.Error("ensure exam curriculum context failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "curriculum_context_failed", "Could not load curriculum context", r)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) ensureExamCurriculumContext(ctx context.Context, tenantID, examID string) (examCurriculumContextResponse, error) {
	var subjectID sql.NullString
	var subjectName, subjectCode, gradeLevel string
	var usesKisiKisi bool
	err := a.db.QueryRowContext(ctx, `
		SELECT e.subject_id::text, COALESCE(s.name,''), COALESCE(s.code,''), COALESCE(e.grade_level,''), e.uses_kisi_kisi
		FROM exams e
		LEFT JOIN subjects s ON s.id = e.subject_id
		WHERE e.id = $1 AND e.tenant_id = $2`, examID, tenantID,
	).Scan(&subjectID, &subjectName, &subjectCode, &gradeLevel, &usesKisiKisi)
	if err != nil {
		return examCurriculumContextResponse{}, err
	}
	resp := examCurriculumContextResponse{
		Status:      "not_applicable",
		Source:      "none",
		SubjectName: subjectName,
		SubjectCode: subjectCode,
		GradeLevel:  gradeLevel,
		Elements:    []cpElementRow{},
	}
	if subjectID.Valid {
		v := subjectID.String
		resp.SubjectID = &v
	}
	if !usesKisiKisi {
		return resp, nil
	}
	levelCode := cpLevelCodeForGrade(gradeLevel)
	phase := gradeLevelToPhase(gradeLevel)
	resp.LevelCode = levelCode
	resp.Phase = phase
	if subjectCode == "" || gradeLevel == "" || levelCode == "" || phase == "" {
		resp.Status = "missing"
		resp.Warnings = append(resp.Warnings, "CP resmi belum bisa dimuat karena mata pelajaran atau kelas exam belum lengkap.")
		return resp, nil
	}
	ref, found, err := a.findCPReferenceBySubject(ctx, levelCode, subjectCode, phase)
	if err != nil {
		return resp, err
	}
	if found {
		resp.Status = "ready"
		resp.Source = "local_db"
		resp.Reference = &ref
		resp.Elements = ref.Elements
		return resp, nil
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 28*time.Second)
	defer cancel()
	seeded, fields, err := a.seedCPReferenceFromKemendikdasmen(fetchCtx, levelCode, subjectCode, phase)
	if err == nil && len(fields) == 0 {
		resp.Status = "ready"
		resp.Source = "remote_fetch"
		resp.Reference = &seeded
		resp.Elements = seeded.Elements
		return resp, nil
	}
	resp.Status = "missing"
	resp.Source = "remote_failed"
	resp.Warnings = append(resp.Warnings, "CP resmi belum tersedia di master data dan fetch Kemendikdasmen gagal. AI masih bisa membantu draft, tetapi CP/TP perlu diverifikasi manual.")
	return resp, nil
}

func (a *App) findCPReferenceBySubject(ctx context.Context, levelCode, subjectCode, phase string) (cpReferenceRow, bool, error) {
	var id string
	err := a.db.QueryRowContext(ctx, `
		SELECT id::text
		FROM curriculum_cp_references
		WHERE curriculum_code = 'merdeka'
		  AND level_code = $1
		  AND subject_code = $2
		  AND phase = $3
		  AND status = 'active'
		LIMIT 1`, levelCode, strings.ToLower(strings.TrimSpace(subjectCode)), strings.ToLower(strings.TrimSpace(phase)),
	).Scan(&id)
	if err == sql.ErrNoRows {
		return cpReferenceRow{}, false, nil
	}
	if err != nil {
		return cpReferenceRow{}, false, err
	}
	ref, _, err := a.loadCPReferenceNoHTTP(ctx, id)
	return ref, err == nil, err
}

func cpLevelCodeForGrade(gradeLevel string) string {
	gradeLevel = strings.ToUpper(strings.TrimSpace(gradeLevel))
	if gradeLevel == "" {
		return ""
	}
	if strings.Contains(gradeLevel, "SMK") {
		return "smk"
	}
	if gradeLevelToPhase(gradeLevel) != "" {
		return "sd-sma"
	}
	return ""
}
