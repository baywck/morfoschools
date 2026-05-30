package app

import (
	"context"
	"encoding/json"
	"fmt"
)

func init() {
	registerAgentActionPlanRunner("action_plan_batch", &agentActionPlanBatchWorkflow{})
}

type agentActionPlanBatchWorkflowArgs struct {
	PlanID        string          `json:"planId"`
	BatchIndex    int             `json:"batchIndex"`
	ApproveBatch  bool            `json:"approveBatch,omitempty"`
	Summary       string          `json:"summary,omitempty"`
	ChildWorkflow string          `json:"childWorkflow,omitempty"`
	ChildArgs     json.RawMessage `json:"childArgs,omitempty"`
}

type agentActionPlanBatchWorkflow struct{}

func (w *agentActionPlanBatchWorkflow) RunBatch(ctx context.Context, app *App, tenantID, userID string, plan agentActionPlanDetail, batch agentActionPlanBatch) (agentWorkflowResult, error) {
	if batch.ArgsJSON == nil || string(batch.ArgsJSON) == "{}" {
		return agentWorkflowResult{
			Workflow: "action_plan_batch",
			Data: map[string]any{
				"planId":     plan.ID,
				"batchIndex": batch.BatchIndex,
				"status":     string(batch.Status),
			},
		}, nil
	}
	var args agentActionPlanBatchWorkflowArgs
	if err := json.Unmarshal(batch.ArgsJSON, &args); err != nil {
		return agentWorkflowResult{}, err
	}
	if args.PlanID == "" {
		args.PlanID = plan.ID
	}
	if args.BatchIndex == 0 {
		args.BatchIndex = batch.BatchIndex
	}
	childWorkflow := args.ChildWorkflow
	childArgs := args.ChildArgs
	if len(childArgs) == 0 {
		childArgs = json.RawMessage(`{}`)
	}
	if childWorkflow == "" {
		return agentWorkflowResult{
			Workflow: "action_plan_batch",
			Data: map[string]any{
				"planId":     args.PlanID,
				"batchIndex": args.BatchIndex,
				"summary":    args.Summary,
				"status":     "noop",
			},
		}, nil
	}
	if childWorkflow == "delete_exam" || childWorkflow == "delete_question" {
		return agentWorkflowResult{}, fmt.Errorf("forbidden agent workflow: %s", childWorkflow)
	}
	result, err := app.executeAgentWorkflow(ctx, tenantID, userID, agentWorkflow(childWorkflow), childArgs)
	if err != nil {
		return agentWorkflowResult{}, err
	}
	return agentWorkflowResult{
		Workflow: "action_plan_batch",
		Data: map[string]any{
			"planId":        args.PlanID,
			"batchIndex":    args.BatchIndex,
			"childWorkflow": childWorkflow,
			"childResult":   result.Data,
		},
	}, nil
}
