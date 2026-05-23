package app

import "testing"

func TestInferNextCompetencyCode_NumericDotPattern(t *testing.T) {
	got := inferNextCompetencyCode([]string{"3.1", "3.2", "3.4", "3.5"}, 9)
	if got != "3.6" {
		t.Fatalf("expected next KD to follow numeric-dot max suffix, got %q", got)
	}
}

func TestInferNextCompetencyCode_IgnoresKOMPFallbacks(t *testing.T) {
	got := inferNextCompetencyCode([]string{"3.1", "KOMP-010", "3.5", "KOMP-011"}, 10)
	if got != "3.6" {
		t.Fatalf("expected KOMP fallback codes to be ignored when numeric KD exists, got %q", got)
	}
}

func TestInferNextCompetencyCode_FallbackWithoutPattern(t *testing.T) {
	got := inferNextCompetencyCode([]string{"KOMP-010", "", "ABC"}, 6)
	if got != "KD-7" {
		t.Fatalf("expected neutral KD fallback without numeric pattern, got %q", got)
	}
}
