package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type agentActionPlanRunner interface {
	RunBatch(ctx context.Context, app *App, tenantID, userID string, plan agentActionPlanDetail, batch agentActionPlanBatch) (agentWorkflowResult, error)
}

var agentActionPlanRunners = map[string]agentActionPlanRunner{}

func registerAgentActionPlanRunner(workflow string, runner agentActionPlanRunner) {
	agentActionPlanRunners[workflow] = runner
}

func (a *App) runAgentActionPlanBatch(ctx context.Context, tenantID, userID string, plan agentActionPlanDetail, batch agentActionPlanBatch) (agentWorkflowResult, error) {
	workflow := strings.TrimSpace(batch.Workflow)
	if workflow == "" {
		return agentWorkflowResult{}, fmt.Errorf("batch workflow is required")
	}
	runner, ok := agentActionPlanRunners[workflow]
	if !ok {
		return a.executeAgentWorkflow(ctx, tenantID, userID, agentWorkflow(workflow), batch.ArgsJSON)
	}
	return runner.RunBatch(ctx, a, tenantID, userID, plan, batch)
}

func (a *App) loadActiveAgentActionPlanForExam(ctx context.Context, examID string) (agentActionPlanDetail, error) {
	if examID == "" || a.db == nil {
		return agentActionPlanDetail{}, fmt.Errorf("examId is required")
	}
	var planID string
	err := a.db.QueryRowContext(ctx, `
		SELECT id::text
		  FROM agent_action_plans
		 WHERE exam_id=$1 AND status IN ('draft','active','paused')
		 ORDER BY updated_at DESC
		 LIMIT 1
	`, examID).Scan(&planID)
	if err != nil {
		return agentActionPlanDetail{}, err
	}
	return a.loadAgentActionPlan(ctx, planID)
}

func activePlanSummary(detail agentActionPlanDetail) map[string]any {
	if detail.ID == "" {
		return nil
	}
	var nextBatchIndex int
	var nextBatchStatus string
	for _, batch := range detail.Batches {
		if batch.Status == agentBatchPending || batch.Status == agentBatchProposed || batch.Status == agentBatchRunning {
			nextBatchIndex = batch.BatchIndex
			nextBatchStatus = string(batch.Status)
			break
		}
	}
	return map[string]any{
		"planId":            detail.ID,
		"goal":              detail.Goal,
		"status":            string(detail.Status),
		"currentBatchIndex": detail.CurrentBatchIndex,
		"totalBatches":      detail.TotalBatches,
		"progressPercent":   detail.ProgressPercent,
		"nextBatchIndex":    nextBatchIndex,
		"nextBatchStatus":   nextBatchStatus,
	}
}

func planPreview(detail agentActionPlanDetail, maxBatches int) string {
	var b strings.Builder
	b.WriteString("Action plan: ")
	if detail.Goal != "" {
		b.WriteString(detail.Goal)
	} else {
		b.WriteString(detail.IntentSummary)
	}
	if detail.TotalBatches > 0 {
		b.WriteString(fmt.Sprintf("\nBatch: %d/%d", detail.CurrentBatchIndex, detail.TotalBatches))
	}
	if detail.ProgressPercent > 0 {
		b.WriteString(fmt.Sprintf("\nProgress: %d%%", detail.ProgressPercent))
	}
	count := 0
	for _, batch := range detail.Batches {
		if maxBatches > 0 && count >= maxBatches {
			break
		}
		count++
		b.WriteString(fmt.Sprintf("\nBatch %d [%s] %s", batch.BatchIndex, batch.Status, batch.ActionType))
		if batch.Workflow != "" {
			b.WriteString(" workflow=")
			b.WriteString(batch.Workflow)
		}
		if batch.Preview != "" {
			b.WriteString(": ")
			b.WriteString(truncateForPrompt(batch.Preview, 160))
		}
	}
	return b.String()
}

func summarizeActionPlanBatchResult(batch agentActionPlanBatch, result agentWorkflowResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("✅ Batch %d selesai.", batch.BatchIndex))
	if result.Data != nil {
		if examID, ok := result.Data["examId"].(string); ok && examID != "" {
			b.WriteString("\nexamId=")
			b.WriteString(examID)
		}
		if updated, ok := result.Data["updatedSlots"]; ok {
			b.WriteString(fmt.Sprintf("\nupdatedSlots=%v", updated))
		}
		if created, ok := result.Data["createdSlots"]; ok {
			b.WriteString(fmt.Sprintf("\ncreatedSlots=%v", created))
		}
	}
	return b.String()
}

func mustJSONRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return json.RawMessage(`{}`)
	}
	return b
}
