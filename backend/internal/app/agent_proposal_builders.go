package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (a *App) handleBlueprintSlotsProposalRequest(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, classification agentTurnClassification) bool {
	args, fromMemory := a.blueprintSlotsArgsFromApprovedMemory(r.Context(), tenantID, sessionID, req)
	var err error
	if !fromMemory {
		args, err = a.generateBlueprintSlotsDraft(r.Context(), tenantID, userID, req, req.Message)
	}
	if err != nil {
		a.logger.Error("create blueprint slots draft failed", "error", err, "classifierReason", classification.Reason)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI belum bisa membuat proposal kisi-kisi. Coba ulang sebentar lagi atau beri topik lebih spesifik.", r)
		return true
	}
	if r.Context().Err() != nil {
		return true
	}
	if fromMemory {
		args = repairMemoryBlueprintSlots(args)
	}
	args = appendBlueprintSlotQualityWarnings(args)
	if fields := a.validateAgentCreateBlueprintSlotsArgs(r.Context(), tenantID, userID, args); len(fields) > 0 {
		content := buildAgentProposalValidationMessage(fields)
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
	if fromMemory && a.createBlueprintSlotEditProposalFromExistingPositions(w, r, tenantID, userID, sessionID, req, args) {
		a.markLatestBlueprintDraftStatus(r.Context(), tenantID, sessionID, deriveScopeKey(req.Shadow.ActiveEntities), "proposal_created")
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

func (a *App) blueprintSlotsArgsFromApprovedMemory(ctx context.Context, tenantID, sessionID string, req aiChatRequest) (agentCreateBlueprintSlotsArgs, bool) {
	if !(classifyShortReply(strings.ToLower(req.Message)) == "affirm" || isBlueprintDraftSaveRequest(strings.ToLower(req.Message))) {
		return agentCreateBlueprintSlotsArgs{}, false
	}
	draft, ok := a.latestBlueprintDraft(ctx, tenantID, sessionID, deriveScopeKey(req.Shadow.ActiveEntities))
	if !ok || len(draft.Slots) == 0 {
		return agentCreateBlueprintSlotsArgs{}, false
	}
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	ctxResp, _ := a.ensureExamCurriculumContext(ctx, tenantID, examID)
	warnings := append([]string{}, ctxResp.Warnings...)
	if ctxResp.Status != "ready" {
		warnings = append(warnings, "CP resmi belum siap; kisi-kisi wajib diverifikasi manual sebelum dipakai.")
	}
	slots := append([]agentBlueprintSlotDraft(nil), draft.Slots...)
	return agentCreateBlueprintSlotsArgs{ExamID: examID, Topic: "Draft kisi-kisi yang sudah disetujui", Slots: slots, Warnings: warnings, CPStatus: ctxResp.Status, CPSource: ctxResp.Source, Confirmable: true}, true
}

func (a *App) createBlueprintSlotEditProposalFromExistingPositions(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, args agentCreateBlueprintSlotsArgs) bool {
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	if examID == "" || len(args.Slots) == 0 {
		return false
	}
	items := make([]agentEditBlueprintSlotArgs, 0, len(args.Slots))
	previews := make([]string, 0, len(args.Slots))
	for _, slot := range args.Slots {
		if slot.Position <= 0 {
			return false
		}
		slotID, err := a.findExamBlueprintSlotIDByPosition(r.Context(), tenantID, examID, slot.Position)
		if err != nil || slotID == "" {
			return false
		}
		before, err := a.loadSlotPayload(r.Context(), "exam_blueprint_slots", slotID)
		if err != nil {
			return false
		}
		after := slotPayloadFromBlueprintDraft(slot)
		merged := mergeSlotPayload(before, after)
		diff := buildBlueprintSlotAIDiff(before, merged)
		if len(diff) == 0 {
			continue
		}
		items = append(items, agentEditBlueprintSlotArgs{SlotID: slotID, Instruction: req.Message, Before: before, After: merged})
		previews = append(previews, "Slot "+strconv.Itoa(slot.Position)+"\n"+buildBlueprintSlotEditPreview(diff, a.blueprintSlotEditWarnings(r.Context(), slotID)))
	}
	if len(items) == 0 {
		return false
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
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, preview)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    map[string]string{"role": "assistant", "content": preview},
		"sessionId":  sessionID,
		"tokens":     0,
		"proposalId": proposalID,
		"proposal":   map[string]any{"id": proposalID, "proposalId": proposalID, "workflow": string(workflow), "toolName": string(workflow), "preview": preview, "confirmationText": preview},
	})
	return true
}

func slotPayloadFromBlueprintDraft(slot agentBlueprintSlotDraft) slotPayload {
	points := float64(slot.Points)
	if points <= 0 {
		points = 1
	}
	return slotPayload{
		CapaianPembelajaran: strPtrIfNotEmpty(slot.CapaianPembelajaran),
		ElemenCP:            strPtrIfNotEmpty(slot.ElemenCP),
		TujuanPembelajaran:  strPtrIfNotEmpty(slot.TujuanPembelajaran),
		MateriPokok:         strPtrIfNotEmpty(slot.MateriPokok),
		Kelas:               strPtrIfNotEmpty(slot.KelasSemester),
		CognitiveLevel:      strPtrIfNotEmpty(normalizeCognitiveLevel(slot.CognitiveLevel)),
		QuestionType:        strPtrIfNotEmpty(normalizeQuestionType(slot.QuestionType)),
		Points:              &points,
		IndikatorSoal:       strPtrIfNotEmpty(slot.IndikatorSoal),
	}
}

func strPtrIfNotEmpty(value string) *string {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	return &v
}

func repairMemoryBlueprintSlots(args agentCreateBlueprintSlotsArgs) agentCreateBlueprintSlotsArgs {
	for i := range args.Slots {
		if args.Slots[i].Position <= 0 {
			args.Slots[i].Position = i + 1
		}
		args.Slots[i].CognitiveLevel = normalizeCognitiveLevel(args.Slots[i].CognitiveLevel)
		args.Slots[i].QuestionType = normalizeQuestionType(args.Slots[i].QuestionType)
		if strings.TrimSpace(args.Slots[i].QuestionType) == "" {
			args.Slots[i].QuestionType = "multiple_choice"
		}
		if args.Slots[i].Points <= 0 {
			args.Slots[i].Points = 1
		}
		if strings.TrimSpace(args.Slots[i].KelasSemester) == "" {
			args.Slots[i].KelasSemester = "Sesuai exam aktif"
		}
	}
	return args
}

func (a *App) createAgentProposalFromClassifiedIntent(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, classification agentTurnClassification) bool {
	intent := agentIntentResponse{Intent: classification.Workflow, Workflow: classification.Workflow, Args: classification.Args}
	return a.createAgentProposalFromIntentResponse(w, r, tenantID, userID, sessionID, req, intent)
}
