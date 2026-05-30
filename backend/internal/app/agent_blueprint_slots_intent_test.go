package app

import "testing"

func TestRequestedBlueprintSlotCount(t *testing.T) {
	if got := requestedBlueprintSlotCount("buatkan proposal 10 slot"); got != 10 {
		t.Fatalf("requestedBlueprintSlotCount = %d, want 10", got)
	}
	if got := blueprintSlotCountOrDefault("buatkan proposal 3 slot"); got != 3 {
		t.Fatalf("blueprintSlotCountOrDefault = %d, want 3", got)
	}
	if got := blueprintSlotCountOrDefault("buatkan proposal slot"); got != 5 {
		t.Fatalf("blueprintSlotCountOrDefault default = %d, want 5", got)
	}
}
