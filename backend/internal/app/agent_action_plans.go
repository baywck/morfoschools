package app

import (
	"encoding/json"
	"time"
)

type agentActionPlanStatus string

const (
	agentActionPlanDraft     agentActionPlanStatus = "draft"
	agentActionPlanActive    agentActionPlanStatus = "active"
	agentActionPlanPaused    agentActionPlanStatus = "paused"
	agentActionPlanCompleted agentActionPlanStatus = "completed"
	agentActionPlanFailed    agentActionPlanStatus = "failed"
	agentActionPlanCancelled agentActionPlanStatus = "cancelled"
)

type agentActionPlanBatchStatus string

const (
	agentBatchPending   agentActionPlanBatchStatus = "pending"
	agentBatchProposed  agentActionPlanBatchStatus = "proposed"
	agentBatchRunning   agentActionPlanBatchStatus = "running"
	agentBatchConfirmed agentActionPlanBatchStatus = "confirmed"
	agentBatchFailed    agentActionPlanBatchStatus = "failed"
	agentBatchSkipped   agentActionPlanBatchStatus = "skipped"
)

type agentActionPlan struct {
	ID                string                `json:"id"`
	TenantID          string                `json:"tenantId"`
	UserID            string                `json:"userId"`
	SessionID         string                `json:"sessionId,omitempty"`
	ExamID            string                `json:"examId,omitempty"`
	ScopeType         string                `json:"scopeType"`
	Source            string                `json:"source"`
	Goal              string                `json:"goal"`
	IntentSummary     string                `json:"intentSummary"`
	PlanJSON          json.RawMessage       `json:"planJson"`
	Status            agentActionPlanStatus `json:"status"`
	CurrentBatchIndex int                   `json:"currentBatchIndex"`
	TotalBatches      int                   `json:"totalBatches"`
	ProgressPercent   int                   `json:"progressPercent"`
	CreatedAt         time.Time             `json:"createdAt"`
	UpdatedAt         time.Time             `json:"updatedAt"`
}

type agentActionPlanBatch struct {
	ID             string                     `json:"id"`
	PlanID         string                     `json:"planId"`
	BatchIndex     int                        `json:"batchIndex"`
	ActionType     string                     `json:"actionType"`
	Workflow       string                     `json:"workflow"`
	TargetType     string                     `json:"targetType"`
	TargetIDs      json.RawMessage            `json:"targetIds"`
	ArgsJSON       json.RawMessage            `json:"argsJson"`
	Preview        string                     `json:"preview"`
	Status         agentActionPlanBatchStatus `json:"status"`
	ProgressUnits  int                        `json:"progressUnits"`
	CompletedUnits int                        `json:"completedUnits"`
	ProposalID     string                     `json:"proposalId,omitempty"`
	ResultJSON     *json.RawMessage           `json:"resultJson,omitempty"`
	Error          string                     `json:"error,omitempty"`
	CreatedAt      time.Time                  `json:"createdAt"`
	UpdatedAt      time.Time                  `json:"updatedAt"`
}

type agentActionPlanDetail struct {
	agentActionPlan
	Batches []agentActionPlanBatch `json:"batches"`
}
