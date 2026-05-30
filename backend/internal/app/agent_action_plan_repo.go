package app

import (
	"context"
	"encoding/json"
	"strings"
)

func (a *App) loadAgentActionPlan(ctx context.Context, planID string) (agentActionPlanDetail, error) {
	var detail agentActionPlanDetail
	var sessionID, examID, planJSONRaw *string
	err := a.db.QueryRowContext(ctx, `
		SELECT id::text,
		       tenant_id::text,
		       user_id::text,
		       session_id::text,
		       exam_id::text,
		       scope_type,
		       source,
		       goal,
		       intent_summary,
		       plan_json::text,
		       status,
		       current_batch_index,
		       total_batches,
		       progress_percent,
		       created_at,
		       updated_at
		  FROM agent_action_plans
		 WHERE id=$1
	`, planID).Scan(
		&detail.ID,
		&detail.TenantID,
		&detail.UserID,
		&sessionID,
		&examID,
		&detail.ScopeType,
		&detail.Source,
		&detail.Goal,
		&detail.IntentSummary,
		&planJSONRaw,
		&detail.Status,
		&detail.CurrentBatchIndex,
		&detail.TotalBatches,
		&detail.ProgressPercent,
		&detail.CreatedAt,
		&detail.UpdatedAt,
	)
	if err != nil {
		return agentActionPlanDetail{}, err
	}
	detail.SessionID = derefString(sessionID)
	detail.ExamID = derefString(examID)
	if planJSONRaw != nil {
		detail.PlanJSON = json.RawMessage(*planJSONRaw)
	} else {
		detail.PlanJSON = json.RawMessage(`{}`)
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT id::text,
		       plan_id::text,
		       batch_index,
		       action_type,
		       workflow,
		       target_type,
		       target_ids::text,
		       args_json::text,
		       preview,
		       status,
		       progress_units,
		       completed_units,
		       proposal_id::text,
		       result_json::text,
		       COALESCE(error, ''),
		       created_at,
		       updated_at
		  FROM agent_action_plan_batches
		 WHERE plan_id=$1
		 ORDER BY batch_index ASC
	`, planID)
	if err != nil {
		return agentActionPlanDetail{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var batch agentActionPlanBatch
		var proposalID, resultJSONRawBatch *string
		var targetJSON, argsJSON *string
		if err := rows.Scan(
			&batch.ID,
			&batch.PlanID,
			&batch.BatchIndex,
			&batch.ActionType,
			&batch.Workflow,
			&batch.TargetType,
			&targetJSON,
			&argsJSON,
			&batch.Preview,
			&batch.Status,
			&batch.ProgressUnits,
			&batch.CompletedUnits,
			&proposalID,
			&resultJSONRawBatch,
			&batch.Error,
			&batch.CreatedAt,
			&batch.UpdatedAt,
		); err != nil {
			continue
		}
		batch.TargetIDs = parseRawJSON(targetJSON, `[]`)
		batch.ArgsJSON = parseRawJSON(argsJSON, `{}`)
		batch.ProposalID = derefString(proposalID)
		if resultJSONRawBatch != nil && strings.TrimSpace(*resultJSONRawBatch) != "" {
			raw := json.RawMessage(*resultJSONRawBatch)
			batch.ResultJSON = &raw
		}
		detail.Batches = append(detail.Batches, batch)
	}
	return detail, nil
}

func parseRawJSON(ptr *string, fallback string) json.RawMessage {
	if ptr == nil {
		return json.RawMessage(fallback)
	}
	v := strings.TrimSpace(*ptr)
	if v == "" {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(v)
}

func derefString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
