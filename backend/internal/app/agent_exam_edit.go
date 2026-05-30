package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type agentEditExamArgs struct {
	ExamID                string   `json:"examId,omitempty"`
	Title                 *string  `json:"title,omitempty"`
	Description           *string  `json:"description,omitempty"`
	SubjectID             *string  `json:"subjectId,omitempty"`
	SubjectName           string   `json:"subjectName,omitempty"`
	GradeLevel            *string  `json:"gradeLevel,omitempty"`
	ExamType              *string  `json:"examType,omitempty"`
	DurationMinutes       *int     `json:"durationMinutes,omitempty"`
	MaxScore              *float64 `json:"maxScore,omitempty"`
	PassingScore          *float64 `json:"passingScore,omitempty"`
	ShuffleQuestions      *bool    `json:"shuffleQuestions,omitempty"`
	ShuffleOptions        *bool    `json:"shuffleOptions,omitempty"`
	ShowResultImmediately *bool    `json:"showResultImmediately,omitempty"`
	UsesKisiKisi          *bool    `json:"usesKisiKisi,omitempty"`
}

func (a *App) validateAgentEditExamArgs(ctx context.Context, tenantID, userID string, args agentEditExamArgs) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(args.ExamID) == "" {
		fields["examId"] = "Open an exam first or specify which exam to edit"
		return fields
	}
	auth := &AuthContext{UserID: userID}
	// Use direct DB owner/collaborator resolution through a lightweight synthetic auth is not enough for admin roles,
	// so this validator only checks existence/basic fields. Confirm path re-checks with stored user tenant context.
	var exists bool
	_ = auth
	_ = a.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exams WHERE id=$1 AND tenant_id=$2)`, args.ExamID, tenantID).Scan(&exists)
	if !exists {
		fields["examId"] = "Exam not found in this tenant"
		return fields
	}
	if args.Title != nil && strings.TrimSpace(*args.Title) == "" {
		fields["title"] = "Title cannot be empty"
	}
	if args.ExamType != nil && !validAgentExamType(*args.ExamType) {
		fields["examType"] = "Exam type must be one of quiz, midterm, final, tryout, daily"
	}
	if args.GradeLevel != nil && strings.TrimSpace(*args.GradeLevel) != "" {
		for key, message := range a.validateTenantGradeLevel(ctx, tenantID, *args.GradeLevel) {
			fields[key] = message
		}
	}
	if args.SubjectID != nil && strings.TrimSpace(*args.SubjectID) != "" {
		var ok bool
		_ = a.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM subjects WHERE id=$1 AND tenant_id=$2)`, *args.SubjectID, tenantID).Scan(&ok)
		if !ok {
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
	if !agentEditExamHasChanges(args) {
		fields["changes"] = "Tell me what exam detail should change"
	}
	return fields
}

func agentEditExamHasChanges(args agentEditExamArgs) bool {
	return args.Title != nil || args.Description != nil || args.SubjectID != nil || strings.TrimSpace(args.SubjectName) != "" || args.GradeLevel != nil || args.ExamType != nil || args.DurationMinutes != nil || args.MaxScore != nil || args.PassingScore != nil || args.ShuffleQuestions != nil || args.ShuffleOptions != nil || args.ShowResultImmediately != nil || args.UsesKisiKisi != nil
}

func (a *App) buildAgentEditExamPreview(ctx context.Context, tenantID string, args agentEditExamArgs) string {
	name := args.ExamID
	_ = a.db.QueryRowContext(ctx, `SELECT title FROM exams WHERE id=$1 AND tenant_id=$2`, args.ExamID, tenantID).Scan(&name)
	lines := []string{"Saya akan mengubah detail exam:", "", fmt.Sprintf("- Exam: %s", name)}
	if args.Title != nil {
		lines = append(lines, fmt.Sprintf("- Judul baru: %s", strings.TrimSpace(*args.Title)))
	}
	if args.Description != nil {
		lines = append(lines, "- Deskripsi: diperbarui")
	}
	if args.SubjectName != "" {
		lines = append(lines, fmt.Sprintf("- Subject: %s", args.SubjectName))
	}
	if args.GradeLevel != nil {
		lines = append(lines, fmt.Sprintf("- Kelas: %s", strings.TrimSpace(*args.GradeLevel)))
	}
	if args.ExamType != nil {
		lines = append(lines, fmt.Sprintf("- Tipe: %s", *args.ExamType))
	}
	if args.DurationMinutes != nil {
		lines = append(lines, fmt.Sprintf("- Durasi: %d menit", *args.DurationMinutes))
	}
	if args.MaxScore != nil {
		lines = append(lines, fmt.Sprintf("- Max score: %.0f", *args.MaxScore))
	}
	if args.PassingScore != nil {
		lines = append(lines, fmt.Sprintf("- Passing score: %.0f", *args.PassingScore))
	}
	if args.UsesKisiKisi != nil {
		if *args.UsesKisiKisi {
			lines = append(lines, "- Kisi-kisi: aktif; container kosong/default akan dibuat jika belum ada")
		} else {
			lines = append(lines, "- Kisi-kisi: nonaktif")
		}
	}
	lines = append(lines, "", "Konfirmasi dulu sebelum saya menyimpan perubahan.")
	return strings.Join(lines, "\n")
}

func (a *App) executeAgentEditExam(ctx context.Context, tenantID, userID string, raw json.RawMessage) (agentWorkflowResult, error) {
	var args agentEditExamArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return agentWorkflowResult{}, err
	}
	if strings.TrimSpace(args.ExamID) == "" {
		return agentWorkflowResult{}, fmt.Errorf("examId is required")
	}
	if (args.SubjectID == nil || strings.TrimSpace(*args.SubjectID) == "") && strings.TrimSpace(args.SubjectName) != "" {
		var subjectID string
		if err := a.db.QueryRowContext(ctx, `SELECT id::text FROM subjects WHERE tenant_id=$1 AND lower(name)=lower($2) LIMIT 1`, tenantID, strings.TrimSpace(args.SubjectName)).Scan(&subjectID); err != nil {
			return agentWorkflowResult{}, err
		}
		args.SubjectID = &subjectID
	}
	var currentStatus string
	var currentUsesKisiKisi bool
	var ownerID string
	if err := a.db.QueryRowContext(ctx, `SELECT status, uses_kisi_kisi, owner_user_id::text FROM exams WHERE id=$1 AND tenant_id=$2`, args.ExamID, tenantID).Scan(&currentStatus, &currentUsesKisiKisi, &ownerID); err != nil {
		if err == sql.ErrNoRows {
			return agentWorkflowResult{}, fmt.Errorf("exam not found")
		}
		return agentWorkflowResult{}, err
	}
	// Minimal server-side permission for agent path: owner only in phase 1. UI/API can still support editors elsewhere.
	if ownerID != userID {
		return agentWorkflowResult{}, fmt.Errorf("only the exam owner can edit exam details via chat")
	}
	if args.UsesKisiKisi != nil && *args.UsesKisiKisi != currentUsesKisiKisi && currentStatus != "draft" {
		return agentWorkflowResult{}, fmt.Errorf("kisi-kisi toggle can only change while exam is draft")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return agentWorkflowResult{}, err
	}
	defer tx.Rollback()
	parts := []string{"updated_at = now()"}
	vals := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, fmt.Sprintf("%s = $%d", col, idx))
		vals = append(vals, val)
		idx++
	}
	if args.Title != nil {
		add("title", strings.TrimSpace(*args.Title))
	}
	if args.Description != nil {
		add("description", *args.Description)
	}
	if args.SubjectID != nil {
		if strings.TrimSpace(*args.SubjectID) == "" {
			parts = append(parts, "subject_id = NULL")
		} else {
			add("subject_id", *args.SubjectID)
		}
	}
	if args.GradeLevel != nil {
		if strings.TrimSpace(*args.GradeLevel) == "" {
			parts = append(parts, "grade_level = NULL")
		} else {
			add("grade_level", strings.TrimSpace(*args.GradeLevel))
		}
	}
	if args.ExamType != nil {
		add("exam_type", *args.ExamType)
	}
	if args.DurationMinutes != nil {
		add("duration_minutes", *args.DurationMinutes)
	}
	if args.MaxScore != nil {
		add("max_score", *args.MaxScore)
	}
	if args.PassingScore != nil {
		add("passing_score", *args.PassingScore)
	}
	if args.ShuffleQuestions != nil {
		add("shuffle_questions", *args.ShuffleQuestions)
	}
	if args.ShuffleOptions != nil {
		add("shuffle_options", *args.ShuffleOptions)
	}
	if args.ShowResultImmediately != nil {
		add("show_result_immediately", *args.ShowResultImmediately)
	}
	if args.UsesKisiKisi != nil {
		add("uses_kisi_kisi", *args.UsesKisiKisi)
	}
	q := "UPDATE exams SET " + joinComma(parts) + fmt.Sprintf(" WHERE id = $%d AND tenant_id = $%d", idx, idx+1)
	vals = append(vals, args.ExamID, tenantID)
	if _, err := tx.ExecContext(ctx, q, vals...); err != nil {
		return agentWorkflowResult{}, err
	}
	blueprintID := ""
	if args.UsesKisiKisi != nil && *args.UsesKisiKisi {
		blueprintID, err = a.ensureExamBlueprintContainer(ctx, tx, tenantID, args.ExamID, "Kisi-Kisi")
		if err != nil {
			return agentWorkflowResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return agentWorkflowResult{}, err
	}
	return agentWorkflowResult{Workflow: agentWorkflowEditExam, Data: map[string]any{"examId": args.ExamID, "status": "updated", "blueprintId": blueprintID}}, nil
}
