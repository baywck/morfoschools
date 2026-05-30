package app

import "strings"

func isBlueprintPageRequest(req aiChatRequest) bool {
	if strings.Contains(strings.ToLower(req.Shadow.Route), "kisi-kisi") {
		return strings.TrimSpace(req.Shadow.ActiveEntities["examId"]) != ""
	}
	return false
}

func isBlueprintDraftSaveRequest(lower string) bool {
	lower = strings.ToLower(strings.TrimSpace(lower))
	if !(strings.Contains(lower, "simpan") || strings.Contains(lower, "save") || strings.Contains(lower, "buatkan proposal")) {
		return false
	}
	return strings.Contains(lower, "slot") || strings.Contains(lower, "kisi-kisi") || strings.Contains(lower, "kisi kisi")
}

func isExplicitBlueprintProposalFallbackCommand(lower string) bool {
	lower = strings.ToLower(strings.TrimSpace(lower))
	if strings.Contains(lower, "buatkan proposal") && (strings.Contains(lower, "slot") || strings.Contains(lower, "kisi-kisi") || strings.Contains(lower, "kisi kisi")) {
		return true
	}
	if !(strings.Contains(lower, "slot") || strings.Contains(lower, "kisi-kisi") || strings.Contains(lower, "kisi kisi")) {
		return false
	}
	if !(strings.Contains(lower, "langsung") || strings.Contains(lower, "sekarang")) {
		return false
	}
	return strings.Contains(lower, "buatkan") || strings.HasPrefix(lower, "buat ") || strings.Contains(lower, " buat ") || strings.Contains(lower, "generate")
}
