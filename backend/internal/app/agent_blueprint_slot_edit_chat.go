package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var chatBlueprintSlotEditRe = regexp.MustCompile(`(?i)\b(?:slot|nomor|nomer|no\.?)\s*(\d+)(?:\s*[-–]\s*(\d+))?\b`)

func (a *App) tryCreateChatBlueprintSlotEditProposal(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest) bool {
	if !isBlueprintSlotEditChatRequest(req) {
		return false
	}
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	if examID == "" {
		return false
	}
	positions := extractBlueprintSlotPositions(req.Message)
	if len(positions) == 0 {
		// Do not let backend author semantic clarification text. Fall through to the
		// LLM discussion/classifier path so the agent, not hardcoded BE copy, handles it.
		return false
	}
	items := []agentEditBlueprintSlotArgs{}
	previews := []string{}
	for _, position := range positions {
		slotID, err := a.findExamBlueprintSlotIDByPosition(r.Context(), tenantID, examID, position)
		if err != nil || slotID == "" {
			content := a.askLLMForErrorMessage(r.Context(), tenantID, userID, fmt.Sprintf("Slot %d tidak ditemukan di kisi-kisi exam aktif", position), "Slot mungkin belum dibuat atau posisi salah.")
			if content == "" {
				content = fmt.Sprintf("Slot %d tidak ditemukan di kisi-kisi exam aktif.", position)
			}
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
			a.logger.Error("chat AI blueprint slot edit failed", "error", err, "slot", position)
			content := a.askLLMForErrorMessage(r.Context(), tenantID, userID, fmt.Sprintf("Gagal generate draft edit untuk slot %d", position), "Response model tidak valid atau terpotong. Coba instruksi lebih pendek atau pecah jadi range kecil.")
			if content == "" {
				content = fmt.Sprintf("Gagal membuat draft edit untuk slot %d. Coba instruksi lebih pendek.", position)
			}
			_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
			writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
			return true
		}
		merged := mergeSlotPayload(before, after)
		if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, merged); len(errs) > 0 {
			content := a.buildAgentProposalValidationMessageWithLLM(r.Context(), tenantID, userID, errs)
			_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
			writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0, "mutated": false, "validation": errs})
			return true
		}
		diff := buildBlueprintSlotAIDiff(before, merged)
		if len(diff) == 0 {
			content := a.askLLMForErrorMessage(r.Context(), tenantID, userID, fmt.Sprintf("Tidak ada perubahan bermakna untuk slot %d", position), "AI menghasilkan draft yang sama dengan data existing. Minta instruksi lebih spesifik.")
			if content == "" {
				content = fmt.Sprintf("Tidak ada perubahan bermakna untuk slot %d. Coba instruksi lebih spesifik.", position)
			}
			_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
			writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
			return true
		}
		warnings := a.blueprintSlotEditWarnings(r.Context(), slotID)
		items = append(items, agentEditBlueprintSlotArgs{SlotID: slotID, Instruction: req.Message, Before: before, After: merged})
		previews = append(previews, fmt.Sprintf("Slot %d\n%s", position, buildBlueprintSlotEditPreview(diff, warnings)))
	}
	workflow := agentWorkflowEditBlueprintSlot
	raw, _ := json.Marshal(items[0])
	if len(items) > 1 {
		workflow = agentWorkflowEditBlueprintSlots
		raw, _ = json.Marshal(agentEditBlueprintSlotsArgs{Items: items})
	}
	preview := strings.Join(previews, "\n\n---\n\n")
	proposalID, err := a.createAgentProposal(r, tenantID, userID, sessionID, workflow, raw, preview)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "proposal_failed", "Could not create proposal", r)
		return true
	}
	content := preview + "\n\nBalas `simpan` untuk menerapkan perubahan ini."
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
	writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0, "proposalId": proposalID, "proposal": map[string]any{"id": proposalID, "proposalId": proposalID, "workflow": string(workflow), "preview": preview}})
	return true
}

func isBlueprintSlotEditChatRequest(req aiChatRequest) bool {
	lower := strings.ToLower(req.Message)
	if !strings.Contains(req.Shadow.Route, "/kisi-kisi") {
		return false
	}
	if !(strings.Contains(lower, "slot") || strings.Contains(lower, "nomor") || strings.Contains(lower, "nomer") || strings.Contains(lower, "no ") || strings.Contains(lower, "no.")) {
		return false
	}
	return strings.Contains(lower, "ubah") || strings.Contains(lower, "perbaiki") || strings.Contains(lower, "edit") || strings.Contains(lower, "revisi") || strings.Contains(lower, "tp") || strings.Contains(lower, "tujuan pembelajaran") || strings.Contains(lower, "indikator") || strings.Contains(lower, "condition") || strings.Contains(lower, "degree")
}

func extractBlueprintSlotPositions(message string) []int {
	match := chatBlueprintSlotEditRe.FindStringSubmatch(message)
	if len(match) < 2 {
		return nil
	}
	var start, end int
	_, _ = fmt.Sscanf(match[1], "%d", &start)
	end = start
	if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
		_, _ = fmt.Sscanf(match[2], "%d", &end)
	}
	if start <= 0 || end <= 0 {
		return nil
	}
	if end < start {
		start, end = end, start
	}
	if end-start > 20 {
		end = start + 20
	}
	out := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		out = append(out, i)
	}
	return out
}

func (a *App) findExamBlueprintSlotIDByPosition(ctx context.Context, tenantID, examID string, position int) (string, error) {
	var slotID string
	err := a.db.QueryRowContext(ctx, `
		SELECT s.id::text
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id=s.exam_blueprint_id
		 WHERE (NULLIF($1,'')::uuid IS NULL OR b.tenant_id=NULLIF($1,'')::uuid) AND b.exam_id=$2 AND s.position=$3
		 LIMIT 1`, tenantID, examID, position).Scan(&slotID)
	return slotID, err
}
