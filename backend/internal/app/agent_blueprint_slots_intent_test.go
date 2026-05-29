package app

import "testing"

func TestRequestedBlueprintSlotLimit(t *testing.T) {
	if maxBlueprintSlotsPerProposal != 5 {
		t.Fatalf("maxBlueprintSlotsPerProposal = %d, want 5", maxBlueprintSlotsPerProposal)
	}
	if got := requestedBlueprintSlotCount("buatkan proposal 10 slot"); got != 10 {
		t.Fatalf("requestedBlueprintSlotCount = %d, want 10", got)
	}
	if got := blueprintSlotCountOrDefault("buatkan proposal 3 slot"); got != 3 {
		t.Fatalf("blueprintSlotCountOrDefault = %d, want 3", got)
	}
}
