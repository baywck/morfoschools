package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type agentCreateExamArgs struct {
	Title                 string   `json:"title"`
	Description           string   `json:"description,omitempty"`
	SubjectID             *string  `json:"subjectId,omitempty"`
	SubjectName           string   `json:"subjectName,omitempty"`
	GradeLevel            *string  `json:"gradeLevel,omitempty"`
	ExamType              string   `json:"examType,omitempty"`
	DurationMinutes       *int     `json:"durationMinutes,omitempty"`
	MaxScore              *float64 `json:"maxScore,omitempty"`
	PassingScore          *float64 `json:"passingScore,omitempty"`
	ShuffleQuestions      bool     `json:"shuffleQuestions,omitempty"`
	ShuffleOptions        bool     `json:"shuffleOptions,omitempty"`
	ShowResultImmediately bool     `json:"showResultImmediately,omitempty"`
	UsesKisiKisi          bool     `json:"usesKisiKisi,omitempty"`
}

func (a *App) validateAgentCreateExamArgs(ctx context.Context, tenantID string, args agentCreateExamArgs) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(args.Title) == "" {
		fields["title"] = "Title is required"
	}
	if args.ExamType == "" {
		args.ExamType = "quiz"
	}
	if !validAgentExamType(args.ExamType) {
		fields["examType"] = "Exam type must be one of quiz, midterm, final, tryout, daily"
	}
	if args.GradeLevel != nil {
		for key, message := range a.validateTenantGradeLevel(ctx, tenantID, *args.GradeLevel) {
			fields[key] = message
		}
	}
	if args.SubjectID != nil && strings.TrimSpace(*args.SubjectID) != "" {
		var exists bool
		_ = a.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM subjects WHERE id=$1 AND tenant_id=$2)`, *args.SubjectID, tenantID).Scan(&exists)
		if !exists {
			fields["subjectId"] = "Subject not found in this tenant"
		}
	} else if strings.TrimSpace(args.SubjectName) != "" {
		var count int
		_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM subjects WHERE tenant_id=$1 AND lower(name)=lower($2)`, tenantID, strings.TrimSpace(args.SubjectName)).Scan(&count)
		if count == 0 {
			fields["subjectName"] = "Subject not found in this tenant"
		}
		if count > 1 {
			fields["subjectName"] = "Multiple subjects matched; choose subject explicitly"
		}
	}
	return fields
}

func validAgentExamType(v string) bool {
	switch v {
	case "quiz", "midterm", "final", "tryout", "daily":
		return true
	default:
		return false
	}
}

func (a *App) buildAgentCreateExamPreview(args agentCreateExamArgs) string {
	examType := args.ExamType
	if examType == "" {
		examType = "quiz"
	}
	lines := []string{
		"Saya akan membuat exam baru:",
		"",
		fmt.Sprintf("- Judul: %s", strings.TrimSpace(args.Title)),
		fmt.Sprintf("- Tipe: %s", examType),
	}
	if args.GradeLevel != nil && strings.TrimSpace(*args.GradeLevel) != "" {
		lines = append(lines, fmt.Sprintf("- Kelas: %s", strings.TrimSpace(*args.GradeLevel)))
	}
	if args.DurationMinutes != nil {
		lines = append(lines, fmt.Sprintf("- Durasi: %d menit", *args.DurationMinutes))
	}
	if args.UsesKisiKisi {
		lines = append(lines, "- Kisi-kisi: aktif; akan dibuat container kisi-kisi kosong/default")
	} else {
		lines = append(lines, "- Kisi-kisi: tidak aktif")
	}
	lines = append(lines, "", "Konfirmasi dulu sebelum saya menyimpan data.")
	return strings.Join(lines, "\n")
}

func (a *App) executeAgentCreateExam(ctx context.Context, tenantID, userID string, raw json.RawMessage) (agentWorkflowResult, error) {
	var args agentCreateExamArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return agentWorkflowResult{}, err
	}
	if args.ExamType == "" {
		args.ExamType = "quiz"
	}
	if (args.SubjectID == nil || strings.TrimSpace(*args.SubjectID) == "") && strings.TrimSpace(args.SubjectName) != "" {
		var subjectID string
		if err := a.db.QueryRowContext(ctx, `SELECT id::text FROM subjects WHERE tenant_id=$1 AND lower(name)=lower($2) LIMIT 1`, tenantID, strings.TrimSpace(args.SubjectName)).Scan(&subjectID); err != nil {
			return agentWorkflowResult{}, err
		}
		args.SubjectID = &subjectID
	}
	maxScore := 100.0
	if args.MaxScore != nil {
		maxScore = *args.MaxScore
	}
	passingScore := 70.0
	if args.PassingScore != nil {
		passingScore = *args.PassingScore
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return agentWorkflowResult{}, err
	}
	defer tx.Rollback()
	var examID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exams (
		    tenant_id, title, description, subject_id, grade_level, exam_type,
		    duration_minutes, max_score, passing_score,
		    shuffle_questions, shuffle_options, show_result_immediately,
		    created_by, owner_user_id, status, uses_kisi_kisi
		) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,'')::uuid, NULLIF($5,''), $6,
		          $7, $8, $9, $10, $11, $12, $13, $13, 'draft', $14)
		RETURNING id::text`,
		tenantID, strings.TrimSpace(args.Title), args.Description, ptrToString(args.SubjectID), ptrToString(args.GradeLevel), args.ExamType,
		args.DurationMinutes, maxScore, passingScore, args.ShuffleQuestions, args.ShuffleOptions, args.ShowResultImmediately,
		userID, args.UsesKisiKisi,
	).Scan(&examID); err != nil {
		return agentWorkflowResult{}, err
	}
	var sectionID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exam_sections (tenant_id, exam_id, title, sort_order)
		VALUES ($1, $2, 'Section 1', 0)
		RETURNING id::text`, tenantID, examID).Scan(&sectionID); err != nil {
		return agentWorkflowResult{}, err
	}
	blueprintID := ""
	if args.UsesKisiKisi {
		blueprintID, err = a.ensureExamBlueprintContainer(ctx, tx, tenantID, examID, strings.TrimSpace(args.Title))
		if err != nil {
			return agentWorkflowResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return agentWorkflowResult{}, err
	}
	return agentWorkflowResult{Workflow: agentWorkflowCreateExam, Data: map[string]any{
		"examId":           examID,
		"defaultSectionId": sectionID,
		"blueprintId":      blueprintID,
		"usesKisiKisi":     args.UsesKisiKisi,
		"status":           "draft",
		"redirectUrl":      "/app/exams/" + examID,
	}}, nil
}
