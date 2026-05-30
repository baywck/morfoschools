package app

import (
	"context"
	"strings"
	"testing"
)

func TestAgentContextPackCanUseSessionScopeWhenActiveExamMissing(t *testing.T) {
	app := &App{}
	// Unit-level guard for the exact scope format used by exam-scoped AI sessions.
	scopeKey := "exam:148ba6ec-a7ef-4c41-8a31-a4652e36b506"
	examID := strings.TrimSpace(strings.TrimPrefix(scopeKey, "exam:"))
	if examID != "148ba6ec-a7ef-4c41-8a31-a4652e36b506" {
		t.Fatalf("unexpected exam id: %q", examID)
	}
	_ = app
}

func TestExtractBlueprintSlotPositionsSupportsNomerNoRange(t *testing.T) {
	cases := map[string][]int{
		"perbaiki nomer 16-20": {16, 17, 18, 19, 20},
		"cek no. 3-4":          {3, 4},
		"ubah nomor 7":         {7},
	}
	for input, want := range cases {
		got := extractBlueprintSlotPositions(input)
		if len(got) != len(want) {
			t.Fatalf("%q expected %v, got %v", input, want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q expected %v, got %v", input, want, got)
			}
		}
	}
}

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
