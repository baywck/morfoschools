package app

import (
	"context"
	"encoding/json"
	"fmt"
)

type agentEditBlueprintSlotsArgs struct {
	Items []agentEditBlueprintSlotArgs `json:"items"`
}

func (a *App) executeAgentEditBlueprintSlots(ctx context.Context, tenantID, userID string, raw json.RawMessage) (agentWorkflowResult, error) {
	var args agentEditBlueprintSlotsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return agentWorkflowResult{}, err
	}
	if len(args.Items) == 0 {
		return agentWorkflowResult{}, fmt.Errorf("items is required")
	}
	updated := 0
	var examID string
	for _, item := range args.Items {
		b, _ := json.Marshal(item)
		res, err := a.executeAgentEditBlueprintSlot(ctx, tenantID, userID, b)
		if err != nil {
			return agentWorkflowResult{}, err
		}
		updated++
		if v, ok := res.Data["examId"].(string); ok && examID == "" {
			examID = v
		}
	}
	return agentWorkflowResult{Workflow: agentWorkflowEditBlueprintSlots, Data: map[string]any{"examId": examID, "updatedSlots": updated}}, nil
}
