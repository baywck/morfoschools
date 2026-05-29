package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var chatBlueprintSlotEditRe = regexp.MustCompile(`(?i)\b(?:slot|nomor)\s*(\d+)\b`)

func (a *App) tryCreateChatBlueprintSlotEditProposal(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest) bool {
	if !isBlueprintSlotEditChatRequest(req) {
		return false
	}
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	if examID == "" {
		return false
	}
	position := extractBlueprintSlotPosition(req.Message)
	if position <= 0 {
		content := "Slot mana yang ingin diubah? Sebutkan nomor slotnya, misalnya: `ubah TP di slot 2 agar mengikuti ABCD`."
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
		return true
	}
	slotID, err := a.findExamBlueprintSlotIDByPosition(r.Context(), tenantID, examID, position)
	if err != nil || slotID == "" {
		content := fmt.Sprintf("Saya tidak menemukan slot %d pada kisi-kisi exam aktif.", position)
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
		return true
	}
	before, err := a.loadSlotPayload(r.Context(), "exam_blueprint_slots", slotID)
	if err != nil {
		return false
	}
	after, err := a.generateBlueprintSlotEditDraft(r.Context(), tenantID, userID, slotID, req.Message, before)
	if err != nil {
		a.logger.Error("chat AI blueprint slot edit failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI belum bisa membuat perubahan slot. Coba instruksi yang lebih spesifik.", r)
		return true
	}
	merged := mergeSlotPayload(before, after)
	if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, merged); len(errs) > 0 {
		content := buildAgentProposalValidationMessage(errs)
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0, "mutated": false, "validation": errs})
		return true
	}
	diff := buildBlueprintSlotAIDiff(before, merged)
	if len(diff) == 0 {
		content := "AI belum menghasilkan perubahan bermakna untuk slot ini. Coba instruksi lebih spesifik, misalnya: `ubah TP slot 2 menjadi ABCD lengkap dengan kondisi dan degree`."
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
		return true
	}
	warnings := a.blueprintSlotEditWarnings(r.Context(), slotID)
	args := agentEditBlueprintSlotArgs{SlotID: slotID, Instruction: req.Message, Before: before, After: merged}
	raw, _ := json.Marshal(args)
	preview := buildBlueprintSlotEditPreview(diff, warnings)
	proposalID, err := a.createAgentProposal(r, tenantID, userID, sessionID, agentWorkflowEditBlueprintSlot, raw, preview)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "proposal_failed", "Could not create proposal", r)
		return true
	}
	content := preview + "\n\nBalas `simpan` untuk menerapkan perubahan ini."
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
	writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0, "proposalId": proposalID, "proposal": map[string]any{"id": proposalID, "proposalId": proposalID, "workflow": string(agentWorkflowEditBlueprintSlot), "preview": preview}})
	return true
}

func isBlueprintSlotEditChatRequest(req aiChatRequest) bool {
	lower := strings.ToLower(req.Message)
	if !strings.Contains(req.Shadow.Route, "/kisi-kisi") {
		return false
	}
	if !(strings.Contains(lower, "slot") || strings.Contains(lower, "nomor")) {
		return false
	}
	return strings.Contains(lower, "ubah") || strings.Contains(lower, "perbaiki") || strings.Contains(lower, "edit") || strings.Contains(lower, "revisi") || strings.Contains(lower, "tp") || strings.Contains(lower, "tujuan pembelajaran") || strings.Contains(lower, "indikator")
}

func extractBlueprintSlotPosition(message string) int {
	match := chatBlueprintSlotEditRe.FindStringSubmatch(message)
	if len(match) < 2 {
		return 0
	}
	var pos int
	_, _ = fmt.Sscanf(match[1], "%d", &pos)
	return pos
}

func (a *App) findExamBlueprintSlotIDByPosition(ctx context.Context, tenantID, examID string, position int) (string, error) {
	var slotID string
	err := a.db.QueryRowContext(ctx, `
		SELECT s.id::text
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id=s.exam_blueprint_id
		 WHERE b.tenant_id=$1 AND b.exam_id=$2 AND s.position=$3
		 LIMIT 1`, tenantID, examID, position).Scan(&slotID)
	return slotID, err
}
