package app

import "strings"

func isBlueprintPageRequest(req aiChatRequest) bool {
	if strings.Contains(strings.ToLower(req.Shadow.Route), "kisi-kisi") {
		return strings.TrimSpace(req.Shadow.ActiveEntities["examId"]) != ""
	}
	return false
}
