package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type agentWorkflow string

const (
	agentWorkflowCreateExam           agentWorkflow = "create_exam"
	agentWorkflowEditExam             agentWorkflow = "edit_exam"
	agentWorkflowCreateExamSection    agentWorkflow = "create_exam_section"
	agentWorkflowCreateBlueprintSlots agentWorkflow = "create_blueprint_slots"
)

type agentWorkflowResult struct {
	Workflow agentWorkflow  `json:"workflow"`
	Data     map[string]any `json:"data"`
}

func (a *App) executeAgentWorkflow(ctx context.Context, tenantID, userID string, workflow agentWorkflow, args json.RawMessage) (agentWorkflowResult, error) {
	switch workflow {
	case agentWorkflowCreateExam:
		return a.executeAgentCreateExam(ctx, tenantID, userID, args)
	case agentWorkflowEditExam:
		return a.executeAgentEditExam(ctx, tenantID, userID, args)
	case agentWorkflowCreateExamSection:
		return a.executeAgentCreateExamSection(ctx, tenantID, userID, args)
	case agentWorkflowCreateBlueprintSlots:
		return a.executeAgentCreateBlueprintSlots(ctx, tenantID, userID, args)
	default:
		return agentWorkflowResult{}, fmt.Errorf("unsupported agent workflow: %s", workflow)
	}
}

func (a *App) ensureExamBlueprintContainer(ctx context.Context, tx *sql.Tx, tenantID, examID, title string) (string, error) {
	var blueprintID string
	err := tx.QueryRowContext(ctx,
		`SELECT id::text FROM exam_blueprints WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&blueprintID)
	if err == nil {
		return blueprintID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	var curriculumID string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM curricula WHERE code = 'merdeka' LIMIT 1`).Scan(&curriculumID); err != nil {
		return "", err
	}
	if title == "" {
		title = "Kisi-Kisi"
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exam_blueprints (
		    tenant_id, exam_id, source_template_id, source_template_version,
		    created_via, title, description, curriculum_id,
		    blueprint_type, total_slots, total_points, strict_coverage, status
		) VALUES ($1, $2, NULL, NULL, 'manual', $3, NULL, $4,
		          'reguler', 0, 0, false, 'draft')
		RETURNING id::text`,
		tenantID, examID, title, curriculumID,
	).Scan(&blueprintID); err != nil {
		return "", err
	}
	return blueprintID, nil
}
