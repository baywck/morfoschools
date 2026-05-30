package app

import (
	"strings"
	"testing"
)

func TestAgentBlueprintQualityRubricPersistsCoreStandards(t *testing.T) {
	rubric := strings.Join(agentBlueprintQualityRubric(), "\n")
	for _, want := range []string{"ABCD", "KKO", "Degree", "stimulus", "Satu indikator = satu soal", "KD/SK"} {
		if !strings.Contains(rubric, want) {
			t.Fatalf("expected rubric to contain %q, got %s", want, rubric)
		}
	}
}
