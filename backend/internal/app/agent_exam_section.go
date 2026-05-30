package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type agentCreateExamSectionArgs struct {
	ExamID      string `json:"examId"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SortOrder   *int   `json:"sortOrder,omitempty"`
}

func (a *App) validateAgentCreateExamSectionArgs(ctx context.Context, tenantID, userID string, args agentCreateExamSectionArgs) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(args.ExamID) == "" {
		fields["examId"] = "Exam is required"
	}
	if strings.TrimSpace(args.Title) == "" {
		fields["title"] = "Section title is required"
	}
	if strings.TrimSpace(args.ExamID) != "" {
		if !a.agentCanWriteExam(ctx, tenantID, userID, strings.TrimSpace(args.ExamID)) {
			fields["examId"] = "Exam not found or you do not have write access"
		}
	}
	return fields
}

func (a *App) buildAgentCreateExamSectionPreview(ctx context.Context, tenantID string, args agentCreateExamSectionArgs) string {
	examTitle := "exam aktif"
	_ = a.db.QueryRowContext(ctx, `SELECT title FROM exams WHERE id=$1 AND tenant_id=$2`, strings.TrimSpace(args.ExamID), tenantID).Scan(&examTitle)
	lines := []string{
		"Saya akan membuat section baru di Questions Manager:",
		"",
		fmt.Sprintf("- Exam: %s", examTitle),
		fmt.Sprintf("- Section: %s", strings.TrimSpace(args.Title)),
	}
	if strings.TrimSpace(args.Description) != "" {
		lines = append(lines, fmt.Sprintf("- Deskripsi: %s", strings.TrimSpace(args.Description)))
	}
	lines = append(lines, "", "Konfirmasi dulu sebelum saya menyimpan data.")
	return strings.Join(lines, "\n")
}

func (a *App) agentCanWriteExam(ctx context.Context, tenantID, userID, examID string) bool {
	auth := &AuthContext{EffectiveTenantID: &tenantID, UserID: userID}
	access, err := a.resolveExamAccess(ctx, tenantID, auth, examID)
	return err == nil && access.CanWrite
}

func (a *App) executeAgentCreateExamSection(ctx context.Context, tenantID, userID string, raw json.RawMessage) (agentWorkflowResult, error) {
	var args agentCreateExamSectionArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return agentWorkflowResult{}, err
	}
	examID := strings.TrimSpace(args.ExamID)
	if !a.agentCanWriteExam(ctx, tenantID, userID, examID) {
		return agentWorkflowResult{}, fmt.Errorf("exam not found or no write access")
	}
	sortOrder := 0
	if args.SortOrder != nil {
		sortOrder = *args.SortOrder
	} else {
		err := a.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM exam_sections WHERE exam_id=$1 AND tenant_id=$2`, examID, tenantID).Scan(&sortOrder)
		if err != nil && err != sql.ErrNoRows {
			return agentWorkflowResult{}, err
		}
	}
	var id string
	if err := a.db.QueryRowContext(ctx, `
		INSERT INTO exam_sections (tenant_id, exam_id, title, description, sort_order)
		VALUES ($1, $2, $3, NULLIF($4,''), $5)
		RETURNING id::text`, tenantID, examID, strings.TrimSpace(args.Title), strings.TrimSpace(args.Description), sortOrder).Scan(&id); err != nil {
		return agentWorkflowResult{}, err
	}
	return agentWorkflowResult{Workflow: agentWorkflowCreateExamSection, Data: map[string]any{
		"examId":      examID,
		"sectionId":   id,
		"title":       strings.TrimSpace(args.Title),
		"status":      "created",
		"redirectUrl": "/app/exams/" + examID,
	}}, nil
}
