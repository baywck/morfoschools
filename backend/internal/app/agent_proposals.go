package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type agentProposalPayload struct {
	Workflow agentWorkflow    `json:"workflow"`
	Args     json.RawMessage  `json:"args"`
	Preview  string           `json:"preview"`
	Result   *json.RawMessage `json:"result,omitempty"`
}

func (a *App) createAgentProposal(r *http.Request, tenantID, userID, sessionID string, workflow agentWorkflow, args json.RawMessage, preview string) (string, error) {
	var id string
	var sID sql.NullString
	if sessionID != "" {
		sID.String = sessionID
		sID.Valid = true
	}
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO agent_proposals (tenant_id, user_id, session_id, workflow, args, preview, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, now() + interval '30 minutes')
		RETURNING id::text`, tenantID, userID, sID, string(workflow), args, preview).Scan(&id)
	return id, err
}

func (a *App) handleAgentConfirm(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	var req struct {
		ProposalID string `json:"proposalId"`
	}
	if err := readJSON(r, &req); err != nil || req.ProposalID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "proposalId is required", r)
		return
	}
	var workflow string
	var args json.RawMessage
	var tenantID string
	var status string
	var expiresAt time.Time
	err := a.db.QueryRowContext(r.Context(), `
		SELECT workflow, args, tenant_id::text, status, expires_at
		  FROM agent_proposals
		 WHERE id=$1 AND user_id=$2`, req.ProposalID, auth.UserID).Scan(&workflow, &args, &tenantID, &status, &expiresAt)
	if err == sql.ErrNoRows {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Proposal not found", r)
		return
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed", "Could not load proposal", r)
		return
	}
	if status != "pending" {
		writeErrorJSON(w, http.StatusConflict, "already_processed", "This proposal has already been processed", r)
		return
	}
	if time.Now().After(expiresAt) {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='expired' WHERE id=$1`, req.ProposalID)
		writeErrorJSON(w, http.StatusGone, "expired", "This proposal has expired", r)
		return
	}
	result, err := a.executeAgentWorkflow(r.Context(), tenantID, auth.UserID, agentWorkflow(workflow), args)
	if err != nil {
		b, _ := json.Marshal(map[string]any{"error": err.Error()})
		_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='failed', error=$2, result=$3 WHERE id=$1`, req.ProposalID, err.Error(), b)
		writeErrorJSON(w, http.StatusInternalServerError, "execution_failed", "Could not execute proposal", r)
		return
	}
	resultJSON, _ := json.Marshal(result)
	_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='confirmed', confirmed_at=now(), result=$2 WHERE id=$1`, req.ProposalID, resultJSON)
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (a *App) tryConfirmLatestAgentProposalFromChat(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID, message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	confirmWords := map[string]bool{"ya": true, "iya": true, "ok": true, "oke": true, "simpan": true, "confirm": true, "konfirmasi": true, "lanjut": true, "save": true}
	matched := confirmWords[lower]
	if !matched && (strings.Contains(lower, "buat") || strings.Contains(lower, "bikin") || strings.Contains(lower, "create")) && (strings.Contains(lower, "ujiannya") || strings.Contains(lower, "examnya") || strings.Contains(lower, "ujianya") || strings.Contains(lower, "exam")) {
		matched = true
	}
	if !matched {
		return false
	}
	var proposalID, workflow string
	var args json.RawMessage
	var expiresAt time.Time
	var createdAt time.Time
	err := a.db.QueryRowContext(r.Context(), `
		SELECT id::text, workflow, args, expires_at, created_at
		  FROM agent_proposals
		 WHERE tenant_id=$1 AND user_id=$2 AND session_id=$3 AND status='pending'
		 ORDER BY created_at DESC
		 LIMIT 1`, tenantID, userID, sessionID).Scan(&proposalID, &workflow, &args, &expiresAt, &createdAt)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "lookup_failed", "Could not load pending proposal", r)
		return true
	}
	if time.Now().After(expiresAt) {
		_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='expired' WHERE id=$1`, proposalID)
		writeErrorJSON(w, http.StatusGone, "expired", "This proposal has expired", r)
		return true
	}
	if fields := a.validateAgentProposalBeforeConfirm(r.Context(), tenantID, userID, agentWorkflow(workflow), args); len(fields) > 0 {
		content := buildAgentProposalValidationMessage(fields)
		b, _ := json.Marshal(map[string]any{"fields": fields})
		_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='failed', error=$2, result=$3 WHERE id=$1`, proposalID, "validation failed", b)
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0, "mutated": false, "validation": fields})
		return true
	}
	result, err := a.executeAgentWorkflow(r.Context(), tenantID, userID, agentWorkflow(workflow), args)
	if err != nil {
		b, _ := json.Marshal(map[string]any{"error": err.Error()})
		_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='failed', error=$2, result=$3 WHERE id=$1`, proposalID, err.Error(), b)
		writeErrorJSON(w, http.StatusInternalServerError, "execution_failed", "Could not execute proposal", r)
		return true
	}
	resultJSON, _ := json.Marshal(result)
	_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='confirmed', confirmed_at=now(), result=$2 WHERE id=$1`, proposalID, resultJSON)
	content := summarizeAgentProposalResult(result)
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
	writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0, "mutated": true, "result": result})
	return true
}

// summarizeAgentProposalResult builds an honest confirmation message based on
// the concrete counts the workflow returned, so the chat can never claim a
// generic success when nothing was actually written.
func summarizeAgentProposalResult(result agentWorkflowResult) string {
	switch result.Workflow {
	case agentWorkflowCreateBlueprintSlots:
		n := intFromAny(result.Data["createdSlots"])
		if n <= 0 {
			return "⚠️ Tidak ada slot kisi-kisi yang dibuat. Proposal tidak berisi slot valid."
		}
		return fmt.Sprintf("✅ %d slot kisi-kisi berhasil dibuat.", n)
	case agentWorkflowEditBlueprintSlot:
		return "✅ Slot kisi-kisi berhasil diperbarui."
	case agentWorkflowEditBlueprintSlots:
		n := intFromAny(result.Data["updatedSlots"])
		if n <= 0 {
			return "⚠️ Tidak ada slot yang diperbarui."
		}
		return fmt.Sprintf("✅ %d slot kisi-kisi berhasil diperbarui.", n)
	case agentWorkflowCreateExam:
		content := "✅ Exam berhasil dibuat."
		if examID, ok := result.Data["examId"].(string); ok && examID != "" {
			content += "\n\nExam ID: " + examID
		}
		return content
	case agentWorkflowCreateExamSection:
		return "✅ Bagian/section exam berhasil dibuat."
	case agentWorkflowEditExam:
		return "✅ Detail exam berhasil diperbarui."
	}
	content := "✅ Proposal dikonfirmasi dan berhasil disimpan."
	if examID, ok := result.Data["examId"].(string); ok && examID != "" {
		content += "\n\nExam ID: " + examID
	}
	return content
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func (a *App) handleAgentCancel(w http.ResponseWriter, r *http.Request) {
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	var req struct {
		ProposalID string `json:"proposalId"`
	}
	if err := readJSON(r, &req); err != nil || req.ProposalID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "proposalId is required", r)
		return
	}
	_, _ = a.db.ExecContext(r.Context(), `UPDATE agent_proposals SET status='cancelled', cancelled_at=now() WHERE id=$1 AND user_id=$2 AND status='pending'`, req.ProposalID, auth.UserID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
}
