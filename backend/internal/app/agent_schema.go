package app

import "encoding/json"

type agentIntentName string

const (
	agentIntentDiscussion  agentIntentName = "discussion"
	agentIntentUnsupported agentIntentName = "unsupported"
)

type agentIntentEnvelope struct {
	Intent        string          `json:"intent"`
	Workflow      string          `json:"workflow,omitempty"`
	Args          json.RawMessage `json:"args"`
	MissingFields []string        `json:"missingFields"`
}

func supportedAgentWorkflows() []agentWorkflow {
	return []agentWorkflow{agentWorkflowCreateExam, agentWorkflowEditExam, agentWorkflowCreateExamSection}
}
