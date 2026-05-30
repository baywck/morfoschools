package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type agentIntentResponse struct {
	Intent        string          `json:"intent"`
	Workflow      string          `json:"workflow,omitempty"`
	Args          json.RawMessage `json:"args"`
	MissingFields []string        `json:"missingFields"`
}

func (a *App) tryCreateAgentProposalFromIntent(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest) bool {
	lower := strings.ToLower(req.Message)
	if isAgentExamDeleteIntent(lower) {
		content := a.askLLMForErrorMessage(r.Context(), tenantID, userID, "User ingin menghapus ujian melalui chat", "AI tidak memiliki izin untuk menghapus ujian. Penghapusan harus dilakukan langsung oleh user dari halaman Exams.")
		if content == "" {
			content = "Saya tidak bisa menghapus ujian lewat chat. Silakan hapus langsung dari halaman Exams."
		}
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
		return true
	}
	if a.tryCreateChatBlueprintSlotEditProposal(w, r, tenantID, userID, sessionID, req) {
		return true
	}
	if isBlueprintPageRequest(req) && (classifyShortReply(lower) == "affirm" || isBlueprintDraftSaveRequest(lower)) {
		scopeKey := deriveScopeKey(req.Shadow.ActiveEntities)
		if draft, ok := a.latestBlueprintDraft(r.Context(), tenantID, sessionID, scopeKey); ok {
			draftJSON := mustJSON(draft)
			req.Message = "Buatkan proposal tepat berdasarkan blueprintDraft JSON yang sudah disetujui user ini. Gunakan slots dari JSON sebagai sumber utama; jangan lanjut ke slot lain dan jangan membuat materi baru. blueprintDraft=" + draftJSON
			return a.handleBlueprintSlotsProposalRequest(w, r, tenantID, userID, sessionID, req, agentTurnClassification{Mode: "proposal_request", Workflow: string(agentWorkflowCreateBlueprintSlots), Reason: "affirmed memory blueprint draft"})
		}
		if previousDraft, ok := a.lastAssistantBlueprintDraft(r.Context(), sessionID); ok {
			req.Message = "Buatkan proposal 5 slot berdasarkan draft yang sudah disetujui user berikut. Pertahankan isi draft, jangan lanjut ke slot lain.\n\n" + previousDraft
			return a.handleBlueprintSlotsProposalRequest(w, r, tenantID, userID, sessionID, req, agentTurnClassification{Mode: "proposal_request", Workflow: string(agentWorkflowCreateBlueprintSlots), Reason: "affirmed previous blueprint draft"})
		}
	}
	auth := AuthFromContext(r.Context())
	roles := []string{}
	if auth != nil {
		roles = auth.Roles
	}
	classification, classErr := a.classifyAgentTurn(r.Context(), tenantID, userID, roles, sessionID, req)
	if classErr != nil {
		a.logger.Error("agent turn classification failed", "error", classErr)
		if isBlueprintPageRequest(req) && isExplicitBlueprintProposalFallbackCommand(lower) {
			return a.handleBlueprintSlotsProposalRequest(w, r, tenantID, userID, sessionID, req, agentTurnClassification{Mode: "proposal_request", Workflow: string(agentWorkflowCreateBlueprintSlots), Reason: "classifier unavailable; explicit proposal fallback"})
		}
		if !isAgentExamMutationCandidate(lower) {
			return false
		}
	} else if classification.Mode == "plan_request" {
		return a.handleAgentPlanRequestFromIntent(w, r, tenantID, userID, sessionID, req, classification)
	} else if classification.Mode != "proposal_request" {
		return false
	} else if agentWorkflow(classification.Workflow) == "" && isBlueprintPageRequest(req) {
		classification.Workflow = string(agentWorkflowCreateBlueprintSlots)
		return a.handleBlueprintSlotsProposalRequest(w, r, tenantID, userID, sessionID, req, classification)
	} else if agentWorkflow(classification.Workflow) == agentWorkflowCreateBlueprintSlots {
		return a.handleBlueprintSlotsProposalRequest(w, r, tenantID, userID, sessionID, req, classification)
	} else if classification.Args != nil && string(classification.Args) != "{}" {
		return a.createAgentProposalFromClassifiedIntent(w, r, tenantID, userID, sessionID, req, classification)
	}
	intent, err := a.extractAgentIntent(r.Context(), tenantID, userID, roles, sessionID, req)
	if err != nil {
		if fallback, ok := a.fallbackCreateExamIntent(r.Context(), tenantID, sessionID, req.Message); ok {
			intent = fallback
		} else {
			return false
		}
	} else if (intent.Intent == "discussion" || genericAgentCreateExamIntent(intent)) && strings.Contains(lower, "buat") {
		if fallback, ok := a.fallbackCreateExamIntent(r.Context(), tenantID, sessionID, req.Message); ok {
			intent = fallback
		}
	}
	return a.createAgentProposalFromIntentResponse(w, r, tenantID, userID, sessionID, req, intent)
}

func (a *App) createAgentProposalFromIntentResponse(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, intent agentIntentResponse) bool {
	workflow := agentWorkflow(intent.Workflow)
	if workflow == "" && intent.Intent == string(agentWorkflowCreateExam) {
		workflow = agentWorkflowCreateExam
	}
	if workflow == "" && intent.Intent == string(agentWorkflowEditExam) {
		workflow = agentWorkflowEditExam
	}
	if workflow == "" && intent.Intent == string(agentWorkflowCreateExamSection) {
		workflow = agentWorkflowCreateExamSection
	}
	if workflow != agentWorkflowCreateExam && workflow != agentWorkflowEditExam && workflow != agentWorkflowCreateExamSection {
		return false
	}
	if len(intent.MissingFields) > 0 {
		content := a.askLLMForErrorMessage(r.Context(), tenantID, userID, "User ingin membuat proposal tapi informasi belum lengkap", fmt.Sprintf("Field yang dibutuhkan: %s", strings.Join(intent.MissingFields, ", ")))
		if content == "" {
			content = "Saya butuh info berikut dulu sebelum membuat proposal: " + strings.Join(intent.MissingFields, ", ")
		}
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
		return true
	}
	var cleanArgs []byte
	var preview string
	if workflow == agentWorkflowCreateExam {
		var args agentCreateExamArgs
		if err := json.Unmarshal(intent.Args, &args); err != nil {
			return false
		}
		if args.ExamType == "" {
			args.ExamType = "quiz"
		}
		if fields := a.validateAgentCreateExamArgs(r.Context(), tenantID, args); len(fields) > 0 {
			writeValidationError(w, fields, r)
			return true
		}
		cleanArgs, _ = json.Marshal(args)
		preview = a.buildAgentCreateExamPreview(args)
	} else if workflow == agentWorkflowEditExam {
		var args agentEditExamArgs
		if err := json.Unmarshal(intent.Args, &args); err != nil {
			return false
		}
		if args.ExamID == "" {
			args.ExamID = req.Shadow.ActiveEntities["examId"]
		}
		if fields := a.validateAgentEditExamArgs(r.Context(), tenantID, userID, args); len(fields) > 0 {
			writeValidationError(w, fields, r)
			return true
		}
		cleanArgs, _ = json.Marshal(args)
		preview = a.buildAgentEditExamPreview(r.Context(), tenantID, args)
	} else {
		var args agentCreateExamSectionArgs
		if err := json.Unmarshal(intent.Args, &args); err != nil {
			return false
		}
		if args.ExamID == "" {
			args.ExamID = req.Shadow.ActiveEntities["examId"]
		}
		if fields := a.validateAgentCreateExamSectionArgs(r.Context(), tenantID, userID, args); len(fields) > 0 {
			writeValidationError(w, fields, r)
			return true
		}
		cleanArgs, _ = json.Marshal(args)
		preview = a.buildAgentCreateExamSectionPreview(r.Context(), tenantID, args)
	}
	proposalID, err := a.createAgentProposal(r, tenantID, userID, sessionID, workflow, cleanArgs, preview)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "proposal_failed", "Could not create proposal", r)
		return true
	}
	content := preview
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    map[string]string{"role": "assistant", "content": content},
		"sessionId":  sessionID,
		"tokens":     0,
		"proposalId": proposalID,
		"proposal": map[string]any{
			"id":               proposalID,
			"proposalId":       proposalID,
			"workflow":         string(workflow),
			"toolName":         string(workflow),
			"preview":          preview,
			"confirmationText": preview,
		},
	})
	return true
}

func genericAgentCreateExamIntent(intent agentIntentResponse) bool {
	if intent.Intent != string(agentWorkflowCreateExam) && intent.Workflow != string(agentWorkflowCreateExam) {
		return false
	}
	var args agentCreateExamArgs
	if err := json.Unmarshal(intent.Args, &args); err != nil {
		return false
	}
	return strings.TrimSpace(args.Title) == "" || strings.EqualFold(strings.TrimSpace(args.Title), "Ujian Baru")
}

func isAgentExamMutationCandidate(lower string) bool {
	mutationWords := []string{"buat", "create", "bikin", "buatkan", "membuat", "tambah", "tambahkan", "ubah", "edit", "update", "ganti", "set", "aktifkan", "nonaktifkan", "namanya", "judulnya", "judul"}
	examWords := []string{"exam", "exams", "ujian", "tes", "kuis", "section", "bagian", "sesi", "kisi-kisi", "blueprint"}
	hasMutation := false
	for _, word := range mutationWords {
		if strings.Contains(lower, word) {
			hasMutation = true
			break
		}
	}
	if !hasMutation {
		return false
	}
	for _, word := range examWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func isAgentExamDeleteIntent(lower string) bool {
	deleteWords := []string{"hapus", "delete", "remove", "buang"}
	examWords := []string{"exam", "exams", "ujian", "tes", "kuis"}
	hasDelete := false
	for _, word := range deleteWords {
		if strings.Contains(lower, word) {
			hasDelete = true
			break
		}
	}
	if !hasDelete {
		return false
	}
	for _, word := range examWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func (a *App) handleAgentPlanRequestFromIntent(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, classification agentTurnClassification) bool {
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	if examID == "" {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Saya butuh examId aktif sebelum membuat rencana eksekusi kisi-kisi.", r)
		return true
	}
	request := agentActionPlanRequest{
		ScopeType: "exam",
		ExamID:    examID,
		Source:    "chat",
		Goal:      strings.TrimSpace(classification.Reason),
	}
	if request.Goal == "" {
		request.Goal = strings.TrimSpace(req.Message)
	}
	planned, err := a.generateAgentActionPlanFromLLM(r.Context(), tenantID, userID, request, req.Message)
	if err != nil {
		a.logger.Error("agent intent plan generation failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI belum bisa menyusun rencana eksekusi kisi-kisi dari permintaan ini.", r)
		return true
	}
	detail, err := a.createAgentActionPlanFromLLM(r.Context(), tenantID, userID, sessionID, request, planned)
	if err != nil {
		a.logger.Error("agent intent plan creation failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "plan_failed", "Gagal menyimpan rencana eksekusi kisi-kisi.", r)
		return true
	}
	content := a.summarizeAgentActionPlanCreation(r.Context(), tenantID, userID, sessionID, detail)
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":   map[string]string{"role": "assistant", "content": content},
		"sessionId": sessionID,
		"planId":    detail.ID,
		"plan":      detail,
	})
	return true
}
