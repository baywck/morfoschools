package app

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func (a *App) tryHandleBlueprintSlotsRequest(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, lower string) bool {
	if !strings.Contains(lower, "kisi") && !strings.Contains(lower, "blueprint") {
		return false
	}
	if !strings.Contains(lower, "buat") && !strings.Contains(lower, "bikin") && !strings.Contains(lower, "generate") && !strings.Contains(lower, "tambahkan") && !strings.Contains(lower, "tambah") {
		return false
	}
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	if examID == "" {
		return false
	}
	args, err := a.generateBlueprintSlotsDraft(r.Context(), tenantID, userID, req, lower)
	if err != nil {
		return false
	}
	if fields := a.validateAgentCreateBlueprintSlotsArgs(r.Context(), tenantID, userID, args); len(fields) > 0 {
		writeValidationError(w, fields, r)
		return true
	}
	cleanArgs, _ := json.Marshal(args)
	preview := a.buildAgentCreateBlueprintSlotsPreview(r.Context(), tenantID, args)
	proposalID, err := a.createAgentProposal(r, tenantID, userID, sessionID, agentWorkflowCreateBlueprintSlots, cleanArgs, preview)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "proposal_failed", "Could not create proposal", r)
		return true
	}
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, preview)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    map[string]string{"role": "assistant", "content": preview},
		"sessionId":  sessionID,
		"tokens":     0,
		"proposalId": proposalID,
		"proposal":   map[string]any{"id": proposalID, "proposalId": proposalID, "workflow": string(agentWorkflowCreateBlueprintSlots), "toolName": string(agentWorkflowCreateBlueprintSlots), "preview": preview, "confirmationText": preview},
	})
	return true
}

var blueprintCountPattern = regexp.MustCompile(`(?i)(\d+)\s*(slot|soal|nomor|butir)?`)

func requestedBlueprintSlotCount(message string) int {
	m := blueprintCountPattern.FindStringSubmatch(message)
	if len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n <= 100 {
			return n
		}
	}
	return 5
}
