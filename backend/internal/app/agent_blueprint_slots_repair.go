package app

import (
	"context"
	"encoding/json"
)

// repairInvalidBlueprintSlots re-prompts the LLM to fix only the slots that
// fail the blocking Kurikulum Merdeka gates. Runs at most two passes. Slots
// that still fail are returned as-is so the caller's validation surfaces a
// clear, friendly message. This stays LLM-only; there is no template fallback.
func (a *App) repairInvalidBlueprintSlots(ctx context.Context, provider resolvedAIProvider, prompt string, slots []agentBlueprintSlotDraft) []agentBlueprintSlotDraft {
	const repairNote = " Beberapa slot berikut melanggar aturan: TP wajib audience 'Peserta didik' + KKO terukur sesuai level; indikator wajib berisi pola stimulus 'Disajikan ...'; level harus C1-C6; tanpa KD/SK. Perbaiki SEMUA slot ini dan kembalikan JSON object {\"slots\":[...]} dengan jumlah dan urutan position yang sama persis."
	for pass := 0; pass < 1; pass++ {
		var badIdx []int
		for i := range slots {
			if hasBlockingCurriculumIssues(validateKurikulumMerdekaBlueprintSlot(slots[i])) {
				badIdx = append(badIdx, i)
			}
		}
		if len(badIdx) == 0 {
			return slots
		}
		bad := make([]agentBlueprintSlotDraft, 0, len(badIdx))
		for _, i := range badIdx {
			bad = append(bad, slots[i])
		}
		badJSON, _ := json.Marshal(map[string]any{"slots": bad})
		content, err := a.callBlueprintSlotsLLMJSON(ctx, provider, prompt+repairNote, string(badJSON))
		if err != nil {
			return slots
		}
		var fixed blueprintSlotsLLMOutput
		if err := json.Unmarshal([]byte(content), &fixed); err != nil || len(fixed.Slots) != len(badIdx) {
			return slots
		}
		for j, i := range badIdx {
			f := fixed.Slots[j]
			f.Position = slots[i].Position
			f.CognitiveLevel = normalizeCognitiveLevel(f.CognitiveLevel)
			f.QuestionType = normalizeQuestionType(f.QuestionType)
			if f.Points <= 0 {
				f.Points = slots[i].Points
			}
			if f.SourceConfidence == "" {
				f.SourceConfidence = slots[i].SourceConfidence
			}
			slots[i] = f
		}
	}
	return slots
}
