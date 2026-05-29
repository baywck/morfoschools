package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (a *App) handleBlueprintSlotsProposalRequest(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, classification agentTurnClassification) bool {
	args, err := a.generateBlueprintSlotsDraft(r.Context(), tenantID, userID, req, req.Message)
	if err != nil {
		a.logger.Error("create blueprint slots draft failed", "error", err, "classifierReason", classification.Reason)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI belum bisa membuat proposal kisi-kisi. Coba ulang sebentar lagi atau beri topik lebih spesifik.", r)
		return true
	}
	if r.Context().Err() != nil {
		return true
	}
	if fields := a.validateAgentCreateBlueprintSlotsArgs(r.Context(), tenantID, userID, args); len(fields) > 0 {
		badSlots := 0
		for k := range fields {
			if strings.HasPrefix(k, "slots.") {
				badSlots++
			}
		}
		var content string
		if badSlots > 0 {
			content = fmt.Sprintf("Saya sudah menyusun draft, tapi %d slot belum memenuhi aturan Kurikulum Merdeka (TP harus ABCD dengan KKO terukur, indikator harus berbasis stimulus 'Disajikan ...'). Coba minta lagi dengan jumlah lebih kecil atau topik lebih spesifik, lalu saya susun ulang.", badSlots)
		} else {
			content = buildAgentProposalValidationMessage(fields)
		}
		if r.Context().Err() != nil {
			return true
		}
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0, "mutated": false, "validation": fields})
		return true
	}
	if r.Context().Err() != nil {
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
	a.markLatestBlueprintDraftStatus(r.Context(), tenantID, sessionID, deriveScopeKey(req.Shadow.ActiveEntities), "proposal_created")
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    map[string]string{"role": "assistant", "content": preview},
		"sessionId":  sessionID,
		"tokens":     0,
		"proposalId": proposalID,
		"proposal":   map[string]any{"id": proposalID, "proposalId": proposalID, "workflow": string(agentWorkflowCreateBlueprintSlots), "toolName": string(agentWorkflowCreateBlueprintSlots), "preview": preview, "confirmationText": preview},
	})
	return true
}

func (a *App) createAgentProposalFromClassifiedIntent(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, classification agentTurnClassification) bool {
	intent := agentIntentResponse{Intent: classification.Workflow, Workflow: classification.Workflow, Args: classification.Args}
	return a.createAgentProposalFromIntentResponse(w, r, tenantID, userID, sessionID, req, intent)
}
