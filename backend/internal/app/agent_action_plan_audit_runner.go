package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	registerAgentActionPlanRunner("audit_blueprint_slots", &auditBlueprintSlotsRunner{})
}

type auditBlueprintSlotsRunnerArgs struct {
	ExamID string `json:"examId"`
}

type auditBlueprintSlotsRunner struct{}

func (r *auditBlueprintSlotsRunner) RunBatch(ctx context.Context, app *App, tenantID, userID string, plan agentActionPlanDetail, batch agentActionPlanBatch) (agentWorkflowResult, error) {
	examID := strings.TrimSpace(plan.ExamID)
	if examID == "" {
		var args auditBlueprintSlotsRunnerArgs
		if batch.ArgsJSON != nil {
			_ = json.Unmarshal(batch.ArgsJSON, &args)
			examID = strings.TrimSpace(args.ExamID)
		}
	}
	if examID == "" {
		return agentWorkflowResult{}, fmt.Errorf("examId is required for blueprint audit")
	}

	type slotRow struct {
		Position       int
		ElemenCP       string
		TP             string
		Materi         string
		Indikator      string
		CognitiveLevel string
		QuestionType   string
		Connected      bool
	}
	rows, err := app.db.QueryContext(ctx, `
		SELECT s.position,
		       COALESCE(NULLIF(TRIM(s.elemen_cp), ''), ''),
		       COALESCE(NULLIF(TRIM(s.tujuan_pembelajaran), ''), ''),
		       COALESCE(NULLIF(TRIM(COALESCE(s.materi_pokok, s.materi, '')), ''), ''),
		       COALESCE(NULLIF(TRIM(COALESCE(s.indikator_soal, s.indikator, '')), ''), ''),
		       COALESCE(NULLIF(TRIM(s.cognitive_level), ''), ''),
		       COALESCE(NULLIF(TRIM(s.question_type), ''), ''),
		       EXISTS(SELECT 1 FROM exam_questions eq WHERE eq.blueprint_slot_id=s.id)
		FROM exam_blueprint_slots s
		JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		WHERE b.exam_id=$1
		ORDER BY s.position
	`, examID)
	if err != nil {
		return agentWorkflowResult{}, err
	}
	defer rows.Close()

	var slots []slotRow
	missingTP := 0
	missingMateri := 0
	missingIndikator := 0
	disconnected := 0
	for rows.Next() {
		var s slotRow
		if err := rows.Scan(&s.Position, &s.ElemenCP, &s.TP, &s.Materi, &s.Indikator, &s.CognitiveLevel, &s.QuestionType, &s.Connected); err != nil {
			return agentWorkflowResult{}, err
		}
		slots = append(slots, s)
		if strings.TrimSpace(s.TP) == "" {
			missingTP++
		}
		if strings.TrimSpace(s.Materi) == "" {
			missingMateri++
		}
		if strings.TrimSpace(s.Indikator) == "" {
			missingIndikator++
		}
		if !s.Connected {
			disconnected++
		}
	}

	issues := []string{}
	if missingTP > 0 {
		issues = append(issues, fmt.Sprintf("%d slot tanpa TP", missingTP))
	}
	if missingMateri > 0 {
		issues = append(issues, fmt.Sprintf("%d slot tanpa materi pokok", missingMateri))
	}
	if missingIndikator > 0 {
		issues = append(issues, fmt.Sprintf("%d slot tanpa indikator", missingIndikator))
	}
	if disconnected > 0 {
		issues = append(issues, fmt.Sprintf("%d slot belum punya soal", disconnected))
	}

	return agentWorkflowResult{
		Workflow: "audit_blueprint_slots",
		Data: map[string]any{
			"examId":           examID,
			"totalSlots":       len(slots),
			"missingTP":        missingTP,
			"missingMateri":    missingMateri,
			"missingIndikator": missingIndikator,
			"disconnected":     disconnected,
			"issues":           issues,
		},
	}, nil
}
