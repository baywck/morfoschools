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

func (a *App) loadActiveAgentActionPlanForExam(ctx context.Context, examID string, latest ...bool) (agentActionPlanDetail, error) {
	if examID == "" || a.db == nil {
		return agentActionPlanDetail{}, fmt.Errorf("examId is required")
	}
	var planID string
	if len(latest) > 0 && latest[0] {
		err := a.db.QueryRowContext(ctx, `
			SELECT id::text
			  FROM agent_action_plans
			 WHERE exam_id=$1
			 ORDER BY updated_at DESC
			 LIMIT 1
		`, examID).Scan(&planID)
		if err != nil {
			return agentActionPlanDetail{}, err
		}
		return a.loadAgentActionPlan(ctx, planID)
	}
	err := a.db.QueryRowContext(ctx, `
		SELECT id::text
		  FROM agent_action_plans
		 WHERE exam_id=$1
		 ORDER BY
			CASE status WHEN 'active' THEN 0 WHEN 'draft' THEN 1 WHEN 'paused' THEN 2 WHEN 'failed' THEN 3 WHEN 'completed' THEN 4 ELSE 5 END,
			updated_at DESC
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

// summarizeActionPlanBatchResult creates a structural summary of batch result data.
// This is used as context for LLM to generate user-facing messages.
func summarizeActionPlanBatchResultData(batch agentActionPlanBatch, result agentWorkflowResult) map[string]any {
	data := map[string]any{
		"batchIndex": batch.BatchIndex,
		"workflow":   batch.Workflow,
		"preview":    batch.Preview,
	}
	if result.Data != nil {
		for k, v := range result.Data {
			data[k] = v
		}
	}
	return data
}

// summarizeActionPlanBatchResultWithLLM generates user-facing message about batch completion via LLM.
func (a *App) summarizeActionPlanBatchResultWithLLM(ctx context.Context, tenantID, userID string, batch agentActionPlanBatch, result agentWorkflowResult) string {
	resultData := summarizeActionPlanBatchResultData(batch, result)
	resultJSON := mustJSON(resultData)

	prompt := "Kamu adalah AI assistant untuk sistem LMS. Sebuah batch eksekusi rencana sudah selesai. " +
		"Buat pesan singkat dalam Bahasa Indonesia yang menjelaskan hasilnya. " +
		"Gunakan data yang tersedia. Maksimal 3-4 baris. Jangan emoji berlebihan."
	userCtx := fmt.Sprintf("Batch result: %s", resultJSON)
	msg := a.askLLMForMessage(ctx, tenantID, userID, prompt, userCtx)
	if msg != "" {
		return msg
	}
	// Fallback to structural summary
	return summarizeActionPlanBatchResultFallback(batch, result)
}

// summarizeActionPlanBatchResultFallback is the minimal structural fallback.
func summarizeActionPlanBatchResultFallback(batch agentActionPlanBatch, result agentWorkflowResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Batch %d selesai.", batch.BatchIndex))
	if result.Data != nil {
		if totalSlots, ok := result.Data["totalSlots"]; ok {
			b.WriteString(fmt.Sprintf(" totalSlots=%v", totalSlots))
		}
		if updated, ok := result.Data["updatedSlots"]; ok {
			b.WriteString(fmt.Sprintf(" updatedSlots=%v", updated))
		}
		if created, ok := result.Data["createdSlots"]; ok {
			b.WriteString(fmt.Sprintf(" createdSlots=%v", created))
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
