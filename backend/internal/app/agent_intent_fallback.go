package app

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

func (a *App) fallbackCreateExamIntent(ctx context.Context, tenantID, sessionID, message string) (agentIntentResponse, bool) {
	combined := strings.TrimSpace(message + "\n" + a.agentRecentPlainContext(ctx, sessionID))
	lower := strings.ToLower(combined)
	if !isAgentExamMutationCandidate(strings.ToLower(message)) {
		return agentIntentResponse{}, false
	}
	if !(strings.Contains(lower, "ujian") || strings.Contains(lower, "exam") || strings.Contains(lower, "tes") || strings.Contains(lower, "kuis")) {
		return agentIntentResponse{}, false
	}

	args := agentCreateExamArgs{ExamType: "quiz"}
	if strings.Contains(lower, "uas") || strings.Contains(lower, "akhir semester") || strings.Contains(lower, "akhir sekolah") || strings.Contains(lower, "kenaikan kelas") || strings.Contains(lower, "pat") || strings.Contains(lower, "sat") || strings.Contains(lower, "sumatif akhir") {
		args.ExamType = "final"
	}
	if strings.Contains(lower, "kisi-kisi") || strings.Contains(lower, "kisi kisi") {
		args.UsesKisiKisi = true
	}
	if grade := inferAgentGradeLevel(lower); grade != "" {
		args.GradeLevel = &grade
	}
	if subject := a.inferAgentSubjectName(ctx, tenantID, lower); subject != "" {
		args.SubjectName = subject
	}
	args.Title = buildFallbackExamTitle(lower, args)
	if strings.TrimSpace(args.Title) == "" || strings.EqualFold(args.Title, "Ujian Baru") {
		return agentIntentResponse{}, false
	}
	raw, _ := json.Marshal(args)
	return agentIntentResponse{Intent: string(agentWorkflowCreateExam), Workflow: string(agentWorkflowCreateExam), Args: raw}, true
}

func (a *App) agentRecentPlainContext(ctx context.Context, sessionID string) string {
	if sessionID == "" {
		return ""
	}
	rows, err := a.db.QueryContext(ctx, `SELECT content FROM ai_messages WHERE session_id=$1 ORDER BY created_at DESC LIMIT 6`, sessionID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	parts := []string{}
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n")
}

func inferAgentGradeLevel(lower string) string {
	re := regexp.MustCompile(`kelas\s*(\d{1,2})`)
	m := re.FindStringSubmatch(lower)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func (a *App) inferAgentSubjectName(ctx context.Context, tenantID, lower string) string {
	rows, err := a.db.QueryContext(ctx, `SELECT name FROM subjects WHERE tenant_id=$1 ORDER BY length(name) DESC`, tenantID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil && strings.Contains(lower, strings.ToLower(name)) {
			return name
		}
	}
	return ""
}

func buildFallbackExamTitle(lower string, args agentCreateExamArgs) string {
	subject := strings.TrimSpace(args.SubjectName)
	grade := ""
	if args.GradeLevel != nil && strings.TrimSpace(*args.GradeLevel) != "" {
		grade = " - Kelas " + strings.TrimSpace(*args.GradeLevel)
	}
	kind := "Ujian"
	if strings.Contains(lower, "akhir sekolah") {
		kind = "Ujian Akhir Sekolah"
	} else if strings.Contains(lower, "kenaikan kelas") || strings.Contains(lower, "pat") || strings.Contains(lower, "sat") || strings.Contains(lower, "sumatif akhir") {
		kind = "Ujian Kenaikan Kelas"
	} else if strings.Contains(lower, "uas") || strings.Contains(lower, "akhir semester") {
		kind = "Ujian Akhir Semester"
	}
	if subject != "" {
		return strings.TrimSpace(kind + " " + subject + grade)
	}
	return strings.TrimSpace(kind + grade)
}
