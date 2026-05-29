package app

import "strings"

func isBlueprintPageRequest(req aiChatRequest) bool {
	if strings.Contains(strings.ToLower(req.Shadow.Route), "kisi-kisi") {
		return strings.TrimSpace(req.Shadow.ActiveEntities["examId"]) != ""
	}
	return false
}

// isBlueprintSlotCreateCommand detects an explicit imperative command to
// create kisi-kisi slots now (e.g. "buatkan 10 slot", "buat 10 kisi-kisi",
// "langsung buatkan 10 sekaligus"). It deliberately requires an imperative
// creation verb so planning phrases like "aku berencana membuat 10 soal" or
// "aku ingin membuat kisi-kisi" do NOT match.
func isBlueprintSlotCreateCommand(lower string) bool {
	lower = strings.ToLower(strings.TrimSpace(lower))
	// Planning / intention phrasing must never be treated as a command.
	planningMarkers := []string{"aku ingin", "saya ingin", "aku mau", "saya mau", "aku berencana", "saya berencana", "rencana", "bagaimana", "gimana", "bantu aku", "tolong jelaskan", "diskusi"}
	for _, m := range planningMarkers {
		if strings.Contains(lower, m) {
			return false
		}
	}
	hasCreateVerb := strings.Contains(lower, "buatkan") ||
		strings.Contains(lower, "buat ") ||
		strings.HasPrefix(lower, "buat") ||
		strings.Contains(lower, "generate") ||
		strings.Contains(lower, "susun") ||
		strings.Contains(lower, "bikin")
	if !hasCreateVerb {
		return false
	}
	hasSlotTarget := strings.Contains(lower, "slot") ||
		strings.Contains(lower, "kisi-kisi") ||
		strings.Contains(lower, "kisi kisi")
	// A request that explicitly targets "soal" (questions) without naming a
	// slot/kisi-kisi is NOT a slot-creation command — questions are authored
	// from slots via a different flow.
	if !hasSlotTarget && strings.Contains(lower, "soal") {
		return false
	}
	// This helper only runs on the kisi-kisi page (isBlueprintPageRequest),
	// so the slot target is contextually implied even when not named. Gate on
	// an explicit count or a "sekaligus/langsung" marker so a vague
	// "buat kisi-kisi" still goes through the clarification/proposal flow.
	if !hasSlotTarget && !(blueprintCountPattern.MatchString(lower) || strings.Contains(lower, "sekaligus") || strings.Contains(lower, "langsung")) {
		return false
	}
	return blueprintCountPattern.MatchString(lower) ||
		strings.Contains(lower, "sekaligus") ||
		strings.Contains(lower, "langsung")
}
