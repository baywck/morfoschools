package app

import (
	"context"
	"encoding/json"
	"fmt"
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
	// ALWAYS check for existing positions — not just when fromMemory.
	// If LLM generates slots that overlap existing positions (e.g. audit repair),
	// split into edit-existing + create-new proposals.
	if handled := a.handleMixedEditOrCreateBlueprintSlots(w, r, tenantID, userID, sessionID, req, args); handled {
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

func ptrIntVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
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
		Kelas:               safeGradeLevelPtr(slot.KelasSemester),
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

func safeGradeLevelPtr(value string) *string {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	if gradeLevelToPhase(v) == "" {
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

// handleMixedEditOrCreateBlueprintSlots checks ALL generated slots against existing
// DB positions. Existing positions become edit proposals; truly new positions become
// create proposals. Returns true if any proposals were created.
// This prevents the LLM from accidentally creating duplicate slots when it should
// be editing existing ones (e.g. audit repair generating slots 1-50 when 1-25 exist).
func (a *App) handleMixedEditOrCreateBlueprintSlots(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, args agentCreateBlueprintSlotsArgs) bool {
	examID := strings.TrimSpace(req.Shadow.ActiveEntities["examId"])
	if examID == "" || len(args.Slots) == 0 {
		return false
	}

	var editItems []agentEditBlueprintSlotArgs
	var editPreviews []string
	var createSlots []agentBlueprintSlotDraft

	for _, slot := range args.Slots {
		if slot.Position <= 0 {
			createSlots = append(createSlots, slot)
			continue
		}
		slotID, err := a.findExamBlueprintSlotIDByPosition(r.Context(), tenantID, examID, slot.Position)
		if err != nil || slotID == "" {
			// Position doesn't exist yet — it's a new slot
			createSlots = append(createSlots, slot)
			continue
		}
		// Position exists — build edit item
		before, err := a.loadSlotPayload(r.Context(), "exam_blueprint_slots", slotID)
		if err != nil {
			createSlots = append(createSlots, slot)
			continue
		}
		after := slotPayloadFromBlueprintDraft(slot)
		merged := mergeSlotPayload(before, after)
		diff := buildBlueprintSlotAIDiff(before, merged)
		if len(diff) == 0 {
			continue
		}
		editItems = append(editItems, agentEditBlueprintSlotArgs{SlotID: slotID, Instruction: req.Message, Before: before, After: merged})
		editPreviews = append(editPreviews, buildBlueprintSlotEditPreview(diff, a.blueprintSlotEditWarnings(r.Context(), slotID)))
	}

	if len(editItems) == 0 && len(createSlots) == 0 {
		return false
	}

	proposalIDs := make([]string, 0, 2)
	var combinedPreview strings.Builder

	// Create edit proposal for existing positions
	if len(editItems) > 0 {
		workflow := agentWorkflowEditBlueprintSlot
		raw, _ := json.Marshal(editItems[0])
		if len(editItems) > 1 {
			workflow = agentWorkflowEditBlueprintSlots
			raw, _ = json.Marshal(agentEditBlueprintSlotsArgs{Items: editItems})
		}
		var previewBuilder strings.Builder
		if len(editItems) == 1 {
			previewBuilder.WriteString(fmt.Sprintf("✏️ Edit slot %d\n", ptrIntVal(editItems[0].Before.Position)))
		} else {
			previewBuilder.WriteString(fmt.Sprintf("✏️ Edit %d slot kisi-kisi\n", len(editItems)))
		}
		for i, p := range editPreviews {
			previewBuilder.WriteString("\nSlot " + strconv.Itoa(ptrIntVal(editItems[i].Before.Position)) + ": " + p)
		}
		editPreview := previewBuilder.String()
		pid, err := a.createAgentProposal(r, tenantID, userID, sessionID, workflow, raw, editPreview)
		if err == nil {
			proposalIDs = append(proposalIDs, pid)
			combinedPreview.WriteString(editPreview)
		}
	}

	// Create create proposal for truly new positions
	if len(createSlots) > 0 {
		cleanArgs, _ := json.Marshal(agentCreateBlueprintSlotsArgs{ExamID: examID, Topic: args.Topic, Slots: createSlots, Warnings: args.Warnings, CPStatus: args.CPStatus, CPSource: args.CPSource, Confirmable: true})
		createPreview := a.buildAgentCreateBlueprintSlotsPreview(r.Context(), tenantID, agentCreateBlueprintSlotsArgs{ExamID: examID, Topic: args.Topic, Slots: createSlots})
		pid, err := a.createAgentProposal(r, tenantID, userID, sessionID, agentWorkflowCreateBlueprintSlots, cleanArgs, createPreview)
		if err == nil {
			proposalIDs = append(proposalIDs, pid)
			if combinedPreview.Len() > 0 {
				combinedPreview.WriteString("\n\n---\n\n")
			}
			combinedPreview.WriteString(createPreview)
		}
	}

	if len(proposalIDs) == 0 {
		return false
	}

	finalPreview := combinedPreview.String()
	_, _ = a.db.ExecContext(r.Context(), `INSERT INTO ai_messages (session_id, role, content, tokens_used) VALUES ($1, 'assistant', $2, 0)`, sessionID, finalPreview)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":     map[string]string{"role": "assistant", "content": finalPreview},
		"sessionId":   sessionID,
		"tokens":      0,
		"proposalId":  proposalIDs[0],
		"proposalIds": proposalIDs,
		"proposal":    map[string]any{"id": proposalIDs[0], "proposalId": proposalIDs[0]},
	})
	return true
}

func (a *App) createAgentProposalFromClassifiedIntent(w http.ResponseWriter, r *http.Request, tenantID, userID, sessionID string, req aiChatRequest, classification agentTurnClassification) bool {
	intent := agentIntentResponse{Intent: classification.Workflow, Workflow: classification.Workflow, Args: classification.Args}
	return a.createAgentProposalFromIntentResponse(w, r, tenantID, userID, sessionID, req, intent)
}
