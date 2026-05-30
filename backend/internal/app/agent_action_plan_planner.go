package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type agentActionPlanRequest struct {
	ScopeType    string          `json:"scopeType"`
	Source       string          `json:"source"`
	Goal         string          `json:"goal"`
	ExamID       string          `json:"examId,omitempty"`
	SessionID    string          `json:"sessionId,omitempty"`
	TotalBatches int             `json:"totalBatches,omitempty"`
	AutoStart    bool            `json:"autoStart,omitempty"`
	PlanJSON     json.RawMessage `json:"planJson,omitempty"`
}

type agentPlannedBatch struct {
	BatchIndex    int             `json:"batchIndex"`
	ActionType    string          `json:"actionType"`
	Workflow      string          `json:"workflow"`
	TargetType    string          `json:"targetType"`
	TargetIDs     json.RawMessage `json:"targetIds,omitempty"`
	ArgsJSON      json.RawMessage `json:"argsJson,omitempty"`
	Preview       string          `json:"preview,omitempty"`
	ProgressUnits int             `json:"progressUnits,omitempty"`
}

type agentPlannedAction struct {
	ScopeType     string              `json:"scopeType"`
	Source        string              `json:"source"`
	Goal          string              `json:"goal"`
	IntentSummary string              `json:"intentSummary"`
	PlanJSON      json.RawMessage     `json:"planJson,omitempty"`
	Batches       []agentPlannedBatch `json:"batches"`
}

func (a *App) createAgentActionPlanFromLLM(ctx context.Context, tenantID, userID, sessionID string, req agentActionPlanRequest, planned agentPlannedAction) (agentActionPlanDetail, error) {
	if len(planned.Batches) == 0 {
		return agentActionPlanDetail{}, fmt.Errorf("planned batches are required")
	}
	if planned.ScopeType == "" {
		planned.ScopeType = req.ScopeType
	}
	if planned.Source == "" {
		planned.Source = req.Source
	}
	if planned.Goal == "" {
		planned.Goal = req.Goal
	}
	if planned.ScopeType == "" {
		planned.ScopeType = "generic"
	}
	if planned.Source == "" {
		planned.Source = "chat"
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return agentActionPlanDetail{}, err
	}
	defer tx.Rollback()

	var planID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_action_plans (
			tenant_id, user_id, session_id, exam_id, scope_type, source,
			goal, intent_summary, plan_json, status, current_batch_index, total_batches, progress_percent
		) VALUES ($1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, $5, $6, $7, $8, $9, $10, 0, $11, 0)
		RETURNING id::text
	`, tenantID, userID, sessionID, req.ExamID, planned.ScopeType, planned.Source,
		planned.Goal, planned.IntentSummary, string(planJSONOrObject(planned.PlanJSON)),
		agentActionPlanDraft, len(planned.Batches),
	).Scan(&planID)
	if err != nil {
		return agentActionPlanDetail{}, err
	}

	for _, batch := range planned.Batches {
		progressUnits := batch.ProgressUnits
		if progressUnits <= 0 {
			progressUnits = 1
		}
		if batch.ActionType == "" {
			batch.ActionType = "update"
		}
		if batch.TargetType == "" {
			batch.TargetType = "generic"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO agent_action_plan_batches (
				plan_id, batch_index, action_type, workflow, target_type, target_ids,
				args_json, preview, status, progress_units, completed_units
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,0)
		`, planID, batch.BatchIndex, batch.ActionType, batch.Workflow, batch.TargetType,
			rawJSONOrDefault(batch.TargetIDs, `[]`), rawJSONOrDefault(batch.ArgsJSON, `{}`),
			batch.Preview, agentBatchPending, progressUnits,
		); err != nil {
			return agentActionPlanDetail{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return agentActionPlanDetail{}, err
	}

	if req.AutoStart {
		if _, err := a.db.ExecContext(ctx, `
			UPDATE agent_action_plans SET status=$2, updated_at=now() WHERE id=$1
		`, planID, agentActionPlanActive); err != nil {
			return agentActionPlanDetail{}, err
		}
	}

	a.rememberAgentActionPlanGoal(ctx, tenantID, sessionID, req.ExamID, planned.Goal, planID, len(planned.Batches))
	return a.loadAgentActionPlan(ctx, planID)
}

func (a *App) summarizeAgentActionPlanCreation(ctx context.Context, tenantID, userID, sessionID string, detail agentActionPlanDetail) string {
	msg := a.askLLMForActionPlanMessage(ctx, tenantID, userID, sessionID, "plan_created", detail, "User akan menjalankan batch pertama. Jelaskan rencana dan cara memulai.")
	if msg != "" {
		return msg
	}
	// Fallback to structural summary if LLM fails
	var b strings.Builder
	b.WriteString(planPreview(detail, 5))
	b.WriteString("\n\nKetik `jalankan batch berikutnya` untuk mulai batch 1.")
	return b.String()
}

func planJSONOrObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return raw
}

func rawJSONOrDefault(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(fallback)
	}
	return raw
}

func (a *App) rememberAgentActionPlanGoal(ctx context.Context, tenantID, sessionID, examID, goal, planID string, totalBatches int) {
	if sessionID == "" {
		return
	}
	scopeKey := deriveScopeKey(map[string]string{"examId": examID})
	mem := a.loadAgentSessionMemory(ctx, tenantID, sessionID, scopeKey)
	mem.ActiveGoal = goal
	if mem.ActiveGoal != "" {
		mem.ActiveGoal = fmt.Sprintf("%s [planId=%s batches=%d at=%s]", goal, planID, totalBatches, time.Now().UTC().Format(time.RFC3339))
	}
	a.saveAgentSessionMemory(ctx, tenantID, sessionID, scopeKey, mem)
}
