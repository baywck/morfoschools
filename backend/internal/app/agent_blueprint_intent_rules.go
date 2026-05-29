package app

import "strings"

func isVagueBlueprintHelpRequest(lower string) bool {
	if !strings.Contains(lower, "kisi") && !strings.Contains(lower, "blueprint") {
		return false
	}
	if hasExplicitBlueprintGenerationCommand(lower) {
		return false
	}
	if hasPlanningOnlyBlueprintLanguage(lower) {
		return true
	}
	if hasExplicitBlueprintMutationIntent(lower) {
		return false
	}
	vagueMarkers := []string{
		"aku ingin", "saya ingin", "ingin membuat", "mau membuat", "bantu aku", "bantu saya", "bantu membuat", "bantu", "diskusi", "diskusikan", "bahas", "ayo", "mari", "gimana", "bagaimana",
	}
	for _, marker := range vagueMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func hasPlanningOnlyBlueprintLanguage(lower string) bool {
	planningMarkers := []string{"aku ingin", "saya ingin", "berencana", "rencana", "planning", "ingin membuat", "mau membuat"}
	commandMarkers := []string{"buatkan", "buat ", "bikin", "generate", "susun", "rancang", "simpan", "buat proposal", "ajukan proposal"}
	planning := false
	for _, marker := range planningMarkers {
		if strings.Contains(lower, marker) {
			planning = true
			break
		}
	}
	if !planning {
		return false
	}
	for _, marker := range commandMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func hasExplicitBlueprintGenerationCommand(lower string) bool {
	commandMarkers := []string{"buatkan", "buat ", "bikin", "generate", "susun", "rancang", "simpan", "buat proposal", "ajukan proposal"}
	for _, marker := range commandMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func hasExplicitBlueprintMutationIntent(lower string) bool {
	if strings.Contains(lower, "simpan") || strings.Contains(lower, "buat proposal") || strings.Contains(lower, "ajukan proposal") || strings.Contains(lower, "generate") {
		return true
	}
	if requestedBlueprintSlotCount(lower) > 0 {
		return true
	}
	if strings.Contains(lower, "tentang") || strings.Contains(lower, "materi") || strings.Contains(lower, "topik") || strings.Contains(lower, "bab") || strings.Contains(lower, "teks ") {
		return true
	}
	return false
}
