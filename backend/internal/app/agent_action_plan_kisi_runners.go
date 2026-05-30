package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	registerAgentActionPlanRunner("repair_kisi_kisi_slots", &repairKisiKisiSlotsRunner{})
	registerAgentActionPlanRunner("complete_kisi_kisi_slots", &completeKisiKisiSlotsRunner{})
}

type kisiKisiRepairArgs struct {
	ExamID string `json:"examId"`
}

type kisiKisiSlotIssue struct {
	Position int    `json:"position"`
	Issue    string `json:"issue"`
}

type kisiKisiRepairResult struct {
	ExamID       string              `json:"examId"`
	CheckedSlots int                 `json:"checkedSlots"`
	Issues       []kisiKisiSlotIssue `json:"issues"`
	ProposalID   string              `json:"proposalId,omitempty"`
}

type repairKisiKisiSlotsRunner struct{}

func (r *repairKisiKisiSlotsRunner) RunBatch(ctx context.Context, app *App, tenantID, userID string, plan agentActionPlanDetail, batch agentActionPlanBatch) (agentWorkflowResult, error) {
	examID := strings.TrimSpace(plan.ExamID)
	if examID == "" {
		var args kisiKisiRepairArgs
		if batch.ArgsJSON != nil {
			_ = json.Unmarshal(batch.ArgsJSON, &args)
			examID = strings.TrimSpace(args.ExamID)
		}
	}
	if examID == "" {
		return agentWorkflowResult{}, fmt.Errorf("examId is required for kisi-kisi repair")
	}

	issues, err := app.loadBlueprintSlotRepairIssues(ctx, examID)
	if err != nil {
		return agentWorkflowResult{}, err
	}

	return agentWorkflowResult{
		Workflow: "repair_kisi_kisi_slots",
		Data: map[string]any{
			"examId":       examID,
			"checkedSlots": len(issues),
			"issues":       issues,
		},
	}, nil
}

type completeKisiKisiSlotsRunner struct{}

func (r *completeKisiKisiSlotsRunner) RunBatch(ctx context.Context, app *App, tenantID, userID string, plan agentActionPlanDetail, batch agentActionPlanBatch) (agentWorkflowResult, error) {
	examID := strings.TrimSpace(plan.ExamID)
	if examID == "" {
		var args kisiKisiRepairArgs
		if batch.ArgsJSON != nil {
			_ = json.Unmarshal(batch.ArgsJSON, &args)
			examID = strings.TrimSpace(args.ExamID)
		}
	}
	if examID == "" {
		return agentWorkflowResult{}, fmt.Errorf("examId is required for kisi-kisi completion")
	}

	rows, err := app.db.QueryContext(ctx, `
		SELECT s.position,
		       NULLIF(TRIM(s.tujuan_pembelajaran), '') AS tp,
		       NULLIF(TRIM(COALESCE(s.materi_pokok, s.materi, '')), '') AS materi,
		       NULLIF(TRIM(COALESCE(s.indikator_soal, s.indikator, '')), '') AS indikator,
		       EXISTS(SELECT 1 FROM exam_questions eq WHERE eq.blueprint_slot_id=s.id) AS connected
		FROM exam_blueprint_slots s
		JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		WHERE b.exam_id=$1
		ORDER BY s.position
	`, examID)
	if err != nil {
		return agentWorkflowResult{}, err
	}
	defer rows.Close()

	missingTP := 0
	missingMateri := 0
	missingIndikator := 0
	disconnected := 0
	total := 0
	for rows.Next() {
		var position int
		var tp, materi, indikator *string
		var connected bool
		if err := rows.Scan(&position, &tp, &materi, &indikator, &connected); err != nil {
			return agentWorkflowResult{}, err
		}
		total++
		if tp == nil || strings.TrimSpace(*tp) == "" {
			missingTP++
		}
		if materi == nil || strings.TrimSpace(*materi) == "" {
			missingMateri++
		}
		if indikator == nil || strings.TrimSpace(*indikator) == "" {
			missingIndikator++
		}
		if !connected {
			disconnected++
		}
	}

	return agentWorkflowResult{
		Workflow: "complete_kisi_kisi_slots",
		Data: map[string]any{
			"examId":           examID,
			"totalSlots":       total,
			"missingTP":        missingTP,
			"missingMateri":    missingMateri,
			"missingIndikator": missingIndikator,
			"disconnected":     disconnected,
		},
	}, nil
}

func (a *App) loadBlueprintSlotRepairIssues(ctx context.Context, examID string) ([]kisiKisiSlotIssue, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT s.position,
		       NULLIF(TRIM(s.tujuan_pembelajaran), '') AS tp,
		       NULLIF(TRIM(COALESCE(s.materi_pokok, s.materi, '')), '') AS materi,
		       NULLIF(TRIM(COALESCE(s.indikator_soal, s.indikator, '')), '') AS indikator,
		       EXISTS(SELECT 1 FROM exam_questions eq WHERE eq.blueprint_slot_id=s.id) AS connected
		FROM exam_blueprint_slots s
		JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		WHERE b.exam_id=$1
		ORDER BY s.position
	`, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []kisiKisiSlotIssue
	for rows.Next() {
		var position int
		var tp, materi, indikator *string
		var connected bool
		if err := rows.Scan(&position, &tp, &materi, &indikator, &connected); err != nil {
			return nil, err
		}
		if tp == nil || strings.TrimSpace(*tp) == "" {
			issues = append(issues, kisiKisiSlotIssue{Position: position, Issue: "missing_tujuan_pembelajaran"})
		}
		if materi == nil || strings.TrimSpace(*materi) == "" {
			issues = append(issues, kisiKisiSlotIssue{Position: position, Issue: "missing_materi_pokok"})
		}
		if indikator == nil || strings.TrimSpace(*indikator) == "" {
			issues = append(issues, kisiKisiSlotIssue{Position: position, Issue: "missing_indikator"})
		}
		if !connected {
			issues = append(issues, kisiKisiSlotIssue{Position: position, Issue: "not_connected_to_question"})
		}
	}
	return issues, nil
}
