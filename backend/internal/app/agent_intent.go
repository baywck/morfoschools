package app

import (
	"encoding/json"
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
		content := "Saya tidak punya izin untuk menghapus ujian lewat chat. Penghapusan ujian harus dilakukan langsung oleh pengguna yang berwenang dari halaman Exams. Jika Anda punya akses owner/admin, buka daftar Exams, pilih ujian yang ingin dihapus, lalu gunakan menu hapus/arsipkan di sana."
		_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, content)
		writeJSON(w, http.StatusOK, map[string]any{"message": map[string]string{"role": "assistant", "content": content}, "sessionId": sessionID, "tokens": 0})
		return true
	}
	if a.tryCreateChatBlueprintSlotEditProposal(w, r, tenantID, userID, sessionID, req) {
		return true
	}
	// Deterministic fast-path: a clear "buatkan N slot" command on the
	// kisi-kisi page goes straight to a proposal, skipping the classifier LLM
	// call. This removes one sequential round-trip so the whole chain stays
	// well under the client timeout.
	if isBlueprintPageRequest(req) && isBlueprintSlotCreateCommand(lower) {
		return a.handleBlueprintSlotsProposalRequest(w, r, tenantID, userID, sessionID, req, agentTurnClassification{Mode: "proposal_request", Workflow: string(agentWorkflowCreateBlueprintSlots), Reason: "deterministic slot-create command"})
	}
	auth := AuthFromContext(r.Context())
	roles := []string{}
	if auth != nil {
		roles = auth.Roles
	}
	classification, classErr := a.classifyAgentTurn(r.Context(), tenantID, userID, roles, sessionID, req)
	if classErr != nil {
		a.logger.Error("agent turn classification failed", "error", classErr)
		if !isAgentExamMutationCandidate(lower) {
			return false
		}
	} else if classification.Mode != "proposal_request" {
		// Safety net: a clear "create N slots" command on the kisi-kisi page
		// must become a real proposal, never discussion (which can fabricate
		// slots as plain text). The classifier occasionally misroutes these.
		if isBlueprintPageRequest(req) && isBlueprintSlotCreateCommand(lower) {
			classification.Workflow = string(agentWorkflowCreateBlueprintSlots)
			return a.handleBlueprintSlotsProposalRequest(w, r, tenantID, userID, sessionID, req, classification)
		}
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
		content := "Saya butuh info berikut dulu sebelum membuat proposal: " + strings.Join(intent.MissingFields, ", ")
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
