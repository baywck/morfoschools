package app

import (
	"net/http"
	"strings"
)

func (a *App) registerAgentActionPlanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ai/action-plans/current", a.handleGetCurrentAgentActionPlan)
	mux.HandleFunc("GET /api/v1/ai/action-plans/{planId}", a.handleGetAgentActionPlan)
	mux.HandleFunc("POST /api/v1/ai/action-plans", a.handleCreateAgentActionPlan)
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
		SessionID string `json:"sessionId"`
		Message   string `json:"message"`
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
	planned, err := a.generateAgentActionPlanFromLLM(r.Context(), tenantID, auth.UserID, req.agentActionPlanRequest, req.Message)
	if err != nil {
		a.logger.Error("agent action plan generation failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI belum bisa membuat rencana eksekusi. Coba permintaan lebih spesifik.", r)
		return
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
