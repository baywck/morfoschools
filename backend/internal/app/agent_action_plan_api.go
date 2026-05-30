package app

import (
	"net/http"
	"strings"
)

func (a *App) registerAgentActionPlanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ai/action-plans/current", a.handleGetCurrentAgentActionPlan)
	mux.HandleFunc("GET /api/v1/ai/action-plans/{planId}", a.handleGetAgentActionPlan)
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
