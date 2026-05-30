package app

import (
	"context"
	"strings"
	"testing"
)

func TestStaleBlueprintContextClaimDetected(t *testing.T) {
	for _, content := range []string{
		"existingSlotCount: 0",
		"Saya tidak memiliki akses langsung untuk membaca data slot 16-20",
		"data tidak termuat dalam konteks",
		"tampilkan dulu slot 16-20",
	} {
		if !staleBlueprintContextClaim(content) {
			t.Fatalf("expected stale claim to be detected: %q", content)
		}
	}
}

func TestDiscussionPromptRequiresExistingSlotContextUse(t *testing.T) {
	prompt := (&App{}).discussionSystemPrompt(context.Background(), "", map[string]string{"examId": "exam-1"})
	for _, want := range []string{"AgentContextPack.blueprint.slots", "nomor/nomer/no slot", "Jangan pernah berkata 'saya tidak memiliki akses langsung'", "tampilkan dulu slot"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got: %s", want, prompt)
		}
	}
}

func TestAgentBlueprintQualityRubricPersistsCoreStandards(t *testing.T) {
	rubric := strings.Join(agentBlueprintQualityRubric(), "\n")
	for _, want := range []string{"ABCD", "KKO", "Degree", "stimulus", "Satu indikator = satu soal", "KD/SK"} {
		if !strings.Contains(rubric, want) {
			t.Fatalf("expected rubric to contain %q, got %s", want, rubric)
		}
	}
}
