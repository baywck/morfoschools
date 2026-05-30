package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type agentEditBlueprintSlotArgs struct {
	SlotID      string      `json:"slotId"`
	Instruction string      `json:"instruction,omitempty"`
	Before      slotPayload `json:"before"`
	After       slotPayload `json:"after"`
}

type blueprintSlotAIDiff struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type blueprintSlotAIEditResponse struct {
	ProposalID string                `json:"proposalId"`
	Workflow   agentWorkflow         `json:"workflow"`
	Before     slotPayload           `json:"before"`
	After      slotPayload           `json:"after"`
	Diff       []blueprintSlotAIDiff `json:"diff"`
	Warnings   []string              `json:"warnings,omitempty"`
	Preview    string                `json:"preview"`
}

func (a *App) registerBlueprintSlotAIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/exam-blueprint-slots/{slotId}/ai-edit", a.handleAIEditExamBlueprintSlot)
}

func (a *App) handleAIEditExamBlueprintSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "blueprints:write") {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	auth := AuthFromContext(r.Context())
	if auth == nil || auth.UserID == "" {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "Not authenticated", r)
		return
	}
	slotID := r.PathValue("slotId")
	examID, _, blueprintStatus, tenantID, ok := a.resolveExamSlotParent(w, r, slotID)
	if !ok {
		return
	}
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	if blueprintStatus == "locked" {
		writeErrorJSON(w, http.StatusConflict, "invalid_state", "Blueprint is locked", r)
		return
	}
	var req struct {
		Instruction string `json:"instruction"`
	}
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Instruction) == "" {
		writeValidationError(w, map[string]string{"instruction": "Instruksi perubahan wajib diisi"}, r)
		return
	}
	before, err := a.loadSlotPayload(r.Context(), "exam_blueprint_slots", slotID)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Slot not found", r)
		return
	}
	after, err := a.generateBlueprintSlotEditDraft(r.Context(), tenantID, auth.UserID, slotID, req.Instruction, before)
	if err != nil {
		a.logger.Error("AI blueprint slot edit failed", "error", err)
		writeErrorJSON(w, http.StatusBadGateway, "ai_error", "AI belum bisa membuat perubahan slot. Coba instruksi yang lebih spesifik.", r)
		return
	}
	merged := mergeSlotPayload(before, after)
	if errs := a.validateTenantKisiKisiPayload(r.Context(), tenantID, merged); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}
	diff := buildBlueprintSlotAIDiff(before, merged)
	if len(diff) == 0 {
		writeValidationError(w, map[string]string{"instruction": "AI tidak menghasilkan perubahan bermakna. Coba instruksi lebih spesifik (mis. sebutkan field dan target perubahan)."}, r)
		return
	}
	warnings := a.blueprintSlotEditWarnings(r.Context(), slotID)
	args := agentEditBlueprintSlotArgs{SlotID: slotID, Instruction: req.Instruction, Before: before, After: merged}
	raw, _ := json.Marshal(args)
	preview := buildBlueprintSlotEditPreview(diff, warnings)
	proposalID, err := a.createAgentProposal(r, tenantID, auth.UserID, "", agentWorkflowEditBlueprintSlot, raw, preview)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "proposal_failed", "Could not create proposal", r)
		return
	}
	writeJSON(w, http.StatusOK, blueprintSlotAIEditResponse{ProposalID: proposalID, Workflow: agentWorkflowEditBlueprintSlot, Before: before, After: merged, Diff: diff, Warnings: warnings, Preview: preview})
}

func (a *App) executeAgentEditBlueprintSlot(ctx context.Context, tenantID, userID string, raw json.RawMessage) (agentWorkflowResult, error) {
	var args agentEditBlueprintSlotArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return agentWorkflowResult{}, err
	}
	if args.SlotID == "" {
		return agentWorkflowResult{}, fmt.Errorf("slotId is required")
	}
	var examID, blueprintID, status, slotTenant string
	if err := a.db.QueryRowContext(ctx, `
		SELECT b.exam_id::text, b.id::text, b.status, b.tenant_id::text
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id=s.exam_blueprint_id
		 WHERE s.id=$1`, args.SlotID).Scan(&examID, &blueprintID, &status, &slotTenant); err != nil {
		return agentWorkflowResult{}, fmt.Errorf("slot not found")
	}
	if slotTenant != tenantID {
		return agentWorkflowResult{}, fmt.Errorf("slot tenant mismatch")
	}
	if status == "locked" {
		return agentWorkflowResult{}, fmt.Errorf("blueprint is locked")
	}
	access, err := a.resolveExamAccess(ctx, tenantID, &AuthContext{UserID: userID}, examID)
	if err != nil || !access.Allows(ActionWrite) {
		return agentWorkflowResult{}, fmt.Errorf("exam write access required")
	}
	if errs := a.validateTenantKisiKisiPayload(ctx, tenantID, args.After); len(errs) > 0 {
		a.logger.Error("executeAgentEditBlueprintSlot rejected payload", "slotID", args.SlotID, "errors", errs, "after", args.After)
		return agentWorkflowResult{}, fmt.Errorf("invalid kisi-kisi update")
	}
	q, qArgs := buildSlotUpdateSQL("exam_blueprint_slots", args.SlotID, args.After)
	if q == "" {
		return agentWorkflowResult{}, fmt.Errorf("no changes")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return agentWorkflowResult{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, q, qArgs...); err != nil {
		return agentWorkflowResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE exam_blueprints SET
		    total_slots=(SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
		    total_points=(SELECT COALESCE(SUM(points),0) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
		    updated_at=now()
		 WHERE id=$1`, blueprintID); err != nil {
		return agentWorkflowResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return agentWorkflowResult{}, err
	}
	return agentWorkflowResult{Workflow: agentWorkflowEditBlueprintSlot, Data: map[string]any{"slotId": args.SlotID, "examId": examID, "status": "updated"}}, nil
}

func (a *App) generateBlueprintSlotEditDraft(ctx context.Context, tenantID, userID, slotID, instruction string, before slotPayload) (slotPayload, error) {
	provider, err := a.resolveAIProvider(ctx, &AuthContext{UserID: userID, EffectiveTenantID: &tenantID}, tenantID)
	if err != nil {
		return slotPayload{}, err
	}
	beforeJSON, _ := json.Marshal(before)
	prompt := `Anda mengubah SATU slot kisi-kisi Kurikulum Merdeka. Jangan hapus slot. Jangan membuat soal. Revisi hanya field slot.
Wajib patuh: tanpa KD/SK; CP, Elemen CP, TP, Materi, Level, Indikator. TP minimal lengkap ABCD: Audience = "Peserta didik"; Behavior = KKO + kompetensi; Condition = konteks/kondisi belajar; Degree = tingkat keberhasilan. Indikator wajib diawali/berisi stimulus: 'Disajikan ... peserta didik dapat ...'. KKO TP, level, dan indikator harus selaras.
Jika instruksi user umum seperti "perbaiki kisi-kisi", lakukan minimal satu peningkatan bermakna pada tujuanPembelajaran dan/atau indikatorSoal agar lebih spesifik, ABCD, terukur, dan selaras level kognitif. Jangan mengembalikan slot identik.
Kembalikan JSON object valid saja. Boleh root {"slot":{...}} atau langsung field di root. Field: capaianPembelajaran, elemenCp, tujuanPembelajaran, materiPokok, kelas, semester, cognitiveLevel, difficulty, indikatorSoal, questionType, points. WAJIB sertakan SEMUA field di atas dengan nilai final (yang berubah maupun yang dipertahankan), bukan hanya yang berubah.`
	user := fmt.Sprintf("SlotID: %s\nSlot saat ini JSON:\n%s\n\nInstruksi user:\n%s", slotID, string(beforeJSON), instruction)
	content, err := a.callBlueprintSlotEditLLMJSON(ctx, provider, prompt, user)
	if err != nil {
		return slotPayload{}, err
	}
	parsed, parseErr := parseBlueprintSlotEditPayload(content)
	if parseErr == nil {
		return parsed, nil
	}
	repairPrompt := prompt + " Output sebelumnya tidak valid/terpotong. Perbaiki menjadi JSON valid saja, satu object slot lengkap, tanpa markdown."
	repaired, repairErr := a.callBlueprintSlotEditLLMJSON(ctx, provider, repairPrompt, "Perbaiki JSON slot edit ini menjadi valid dan lengkap: "+content)
	if repairErr == nil {
		parsed, parseErr = parseBlueprintSlotEditPayload(repaired)
		if parseErr == nil {
			return parsed, nil
		}
	}
	a.logger.Error("blueprint slot edit parse failed", "content", content, "error", parseErr, "repairError", repairErr)
	return slotPayload{}, parseErr
}

func parseBlueprintSlotEditPayload(content string) (slotPayload, error) {
	// LLM may return either {"slot":{...}} or the slot fields at root.
	var wrapped struct {
		Slot *slotPayload `json:"slot"`
	}
	if err := json.Unmarshal([]byte(content), &wrapped); err == nil && wrapped.Slot != nil && slotPayloadHasEditableValue(*wrapped.Slot) {
		return *wrapped.Slot, nil
	}
	var flat slotPayload
	if err := json.Unmarshal([]byte(content), &flat); err != nil {
		return slotPayload{}, err
	}
	return flat, nil
}

func slotPayloadHasEditableValue(p slotPayload) bool {
	return p.CapaianPembelajaran != nil || p.ElemenCP != nil || p.TujuanPembelajaran != nil || p.MateriPokok != nil || p.Kelas != nil || p.Semester != nil || p.CognitiveLevel != nil || p.Difficulty != nil || p.IndikatorSoal != nil || p.QuestionType != nil || p.Points != nil
}

func (a *App) callBlueprintSlotEditLLMJSON(ctx context.Context, provider resolvedAIProvider, prompt, userMessage string) (string, error) {
	extra := map[string]any{"response_format": map[string]string{"type": "json_object"}}
	resp, err := a.callLLMWithProviderOptions(ctx, provider, []llmMessage{{Role: "system", Content: prompt}, {Role: "user", Content: userMessage}}, 0.2, 3000, extra)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty slot edit response")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			content = content[start : end+1]
		}
	}
	if content == "" {
		return "", fmt.Errorf("empty slot edit content")
	}
	return content, nil
}

func buildBlueprintSlotAIDiff(before, after slotPayload) []blueprintSlotAIDiff {
	fields := []struct {
		key, label string
		get        func(slotPayload) string
	}{
		{"capaianPembelajaran", "Capaian Pembelajaran", func(p slotPayload) string { return strPtrValue(p.CapaianPembelajaran) }},
		{"elemenCp", "Elemen CP", func(p slotPayload) string { return strPtrValue(p.ElemenCP) }},
		{"tujuanPembelajaran", "Tujuan Pembelajaran", func(p slotPayload) string { return strPtrValue(p.TujuanPembelajaran) }},
		{"materiPokok", "Materi Pokok", func(p slotPayload) string { return strPtrValue(p.MateriPokok) }},
		{"cognitiveLevel", "Level", func(p slotPayload) string { return strPtrValue(p.CognitiveLevel) }},
		{"difficulty", "Kesulitan", func(p slotPayload) string { return strPtrValue(p.Difficulty) }},
		{"indikatorSoal", "Indikator Soal", func(p slotPayload) string { return strPtrValue(p.IndikatorSoal) }},
		{"questionType", "Bentuk Soal", func(p slotPayload) string { return strPtrValue(p.QuestionType) }},
	}
	out := []blueprintSlotAIDiff{}
	for _, f := range fields {
		b, a := f.get(before), f.get(after)
		if strings.TrimSpace(b) != strings.TrimSpace(a) {
			out = append(out, blueprintSlotAIDiff{Field: f.key, Label: f.label, Before: b, After: a})
		}
	}
	return out
}

func strPtrValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func buildBlueprintSlotEditPreview(diff []blueprintSlotAIDiff, warnings []string) string {
	var b strings.Builder
	b.WriteString("Proposal: ubah slot kisi-kisi\n")
	for _, w := range warnings {
		b.WriteString("\n⚠️ " + w)
	}
	for _, d := range diff {
		b.WriteString("\n\n" + d.Label + "\nBefore: " + d.Before + "\nAfter: " + d.After)
	}
	return b.String()
}

func (a *App) blueprintSlotEditWarnings(ctx context.Context, slotID string) []string {
	var sort sql.NullInt64
	err := a.db.QueryRowContext(ctx, `SELECT sort_order FROM exam_questions WHERE blueprint_slot_id=$1 LIMIT 1`, slotID).Scan(&sort)
	if err == nil {
		if sort.Valid {
			return []string{fmt.Sprintf("Slot ini terhubung ke soal nomor %d. Soal perlu ditinjau ulang setelah perubahan.", sort.Int64+1)}
		}
		return []string{"Slot ini terhubung ke soal. Soal perlu ditinjau ulang setelah perubahan."}
	}
	return nil
}
