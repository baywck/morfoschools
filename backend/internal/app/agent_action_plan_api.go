package app

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (a *App) registerAgentActionPlanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ai/action-plans/current", a.handleGetCurrentAgentActionPlan)
	mux.HandleFunc("GET /api/v1/ai/action-plans/current/summary", a.handleGetCurrentAgentActionPlanSummary)
	mux.HandleFunc("GET /api/v1/ai/action-plans/{planId}", a.handleGetAgentActionPlan)
	mux.HandleFunc("POST /api/v1/ai/action-plans", a.handleCreateAgentActionPlan)
	mux.HandleFunc("POST /api/v1/ai/action-plans/{planId}/run-next", a.handleRunNextAgentActionPlanBatch)
}

func (a *App) handleGetCurrentAgentActionPlan(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	examID := strings.TrimSpace(r.URL.Query().Get("examId"))
	if examID == "" {
		writeValidationError(w, map[string]string{"examId": "examId is required"}, r)
		return
	}
	plan, err := a.loadActiveAgentActionPlanForExam(r.Context(), examID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "No active action plan found", r)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (a *App) handleGetAgentActionPlan(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	planID := strings.TrimSpace(r.PathValue("planId"))
	if planID == "" {
		writeValidationError(w, map[string]string{"planId": "planId is required"}, r)
		return
	}
	plan, err := a.loadAgentActionPlan(r.Context(), planID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Action plan not found", r)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (a *App) handleCreateAgentActionPlan(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := ""
	if auth.EffectiveTenantID != nil {
		tenantID = *auth.EffectiveTenantID
	}
	var req struct {
		SessionID string              `json:"sessionId"`
		Message   string              `json:"message"`
		Planned   *agentPlannedAction `json:"planned,omitempty"`
		agentActionPlanRequest
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Goal == "" {
		req.Goal = req.Message
	}
	if req.ScopeType == "" {
		req.ScopeType = "generic"
	}
	if req.Source == "" {
		req.Source = "chat"
	}
	var planned agentPlannedAction
	if req.Planned != nil && len(req.Planned.Batches) > 0 {
		planned = *req.Planned
	} else {
		generated, err := a.generateAgentActionPlanFromLLM(r.Context(), tenantID, auth.UserID, req.agentActionPlanRequest, req.Message)
		if err != nil {
			a.logger.Error("agent action plan generation failed", "error", err)
			writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI belum bisa membuat rencana eksekusi. Coba permintaan lebih spesifik.", r)
			return
		}
		planned = generated
	}
	detail, err := a.createAgentActionPlanFromLLM(r.Context(), tenantID, auth.UserID, req.SessionID, req.agentActionPlanRequest, planned)
	if err != nil {
		a.logger.Error("agent action plan creation failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "plan_failed", "Could not create action plan", r)
		return
	}
	content := a.summarizeAgentActionPlanCreation(detail)
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, req.SessionID, content)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":   map[string]string{"role": "assistant", "content": content},
		"sessionId": req.SessionID,
		"planId":    detail.ID,
		"plan":      detail,
	})
}

func (a *App) handleGetCurrentAgentActionPlanSummary(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	examID := strings.TrimSpace(r.URL.Query().Get("examId"))
	if examID == "" {
		writeValidationError(w, map[string]string{"examId": "examId is required"}, r)
		return
	}
	plan, err := a.loadActiveAgentActionPlanForExam(r.Context(), examID, true)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "No active action plan found", r)
		return
	}
	summary := activePlanSummary(plan)
	issues := map[string]any{}
	for _, batch := range plan.Batches {
		if batch.ResultJSON == nil {
			continue
		}
		var parsed map[string]any
		if err := json.Unmarshal(*batch.ResultJSON, &parsed); err == nil {
			if childResult, ok := parsed["childResult"].(map[string]any); ok {
				for key, val := range childResult {
					if key == "examId" {
						continue
					}
					issues[key] = val
				}
			} else {
				for key, val := range parsed {
					if key == "examId" {
						continue
					}
					issues[key] = val
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"planId":        plan.ID,
		"status":        string(plan.Status),
		"summary":       summary,
		"domainSummary": issues,
		"batches":       plan.Batches,
	})
}

func (a *App) handleRunNextAgentActionPlanBatch(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	tenantID := ""
	if auth.EffectiveTenantID != nil {
		tenantID = *auth.EffectiveTenantID
	}
	planID := strings.TrimSpace(r.PathValue("planId"))
	if planID == "" {
		writeValidationError(w, map[string]string{"planId": "planId is required"}, r)
		return
	}
	plan, err := a.loadAgentActionPlan(r.Context(), planID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Action plan not found", r)
		return
	}
	var nextBatch *agentActionPlanBatch
	for i := range plan.Batches {
		batch := plan.Batches[i]
		if batch.Status == agentBatchPending || batch.Status == agentBatchProposed || batch.Status == agentBatchRunning || batch.Status == agentBatchFailed {
			nextBatch = &batch
			break
		}
	}
	if nextBatch == nil {
		writeErrorJSON(w, http.StatusConflict, "no_next_batch", "No pending batch to run", r)
		return
	}
	if plan.Status == agentActionPlanDraft || plan.Status == agentActionPlanFailed {
		if _, err := a.db.ExecContext(r.Context(), `UPDATE agent_action_plans SET status=$2, updated_at=now() WHERE id=$1`, planID, agentActionPlanActive); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "plan_update_failed", "Could not activate action plan", r)
			return
		}
	}
	if nextBatch.Status == agentBatchFailed {
		if _, err := a.db.ExecContext(r.Context(), `UPDATE agent_action_plan_batches SET status=$3, error='', updated_at=now() WHERE id=$1 AND status=$2`, nextBatch.ID, agentBatchFailed, agentBatchPending); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "batch_update_failed", "Could not reset failed batch", r)
			return
		}
		nextBatch.Status = agentBatchPending
	}
	if _, err := a.db.ExecContext(r.Context(), `UPDATE agent_action_plan_batches SET status=$3, updated_at=now() WHERE id=$1 AND status=$2`, nextBatch.ID, nextBatch.Status, agentBatchRunning); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "batch_update_failed", "Could not start batch", r)
		return
	}
	result, execErr := a.runAgentActionPlanBatch(r.Context(), tenantID, auth.UserID, plan, *nextBatch)
	if execErr != nil {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_action_plan_batches SET status='failed', error=$2, updated_at=now() WHERE id=$1`, nextBatch.ID, execErr.Error())
		_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_action_plans SET status='failed', updated_at=now() WHERE id=$1`, planID)
		writeErrorJSON(w, http.StatusInternalServerError, "batch_failed", "Batch execution failed", r)
		return
	}
	resultJSON := mustJSONRaw(result)
	if _, err := a.db.ExecContext(r.Context(), `UPDATE agent_action_plan_batches SET status='confirmed', completed_units=progress_units, result_json=$2, updated_at=now() WHERE id=$1`, nextBatch.ID, resultJSON); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "batch_update_failed", "Could not finalize batch", r)
		return
	}
	if err := a.refreshAgentActionPlanProgress(r.Context(), planID); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "plan_update_failed", "Could not refresh plan progress", r)
		return
	}
	updatedPlan, err := a.loadAgentActionPlan(r.Context(), planID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "plan_reload_failed", "Could not reload action plan", r)
		return
	}
	content := summarizeActionPlanBatchResult(*nextBatch, result)
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, plan.SessionID, content)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    map[string]string{"role": "assistant", "content": content},
		"planId":     planID,
		"batchIndex": nextBatch.BatchIndex,
		"result":     result,
		"plan":       updatedPlan,
	})
}
