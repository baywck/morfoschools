package app

import "testing"

func TestValidateAgentCreateBlueprintSlotsDoesNotBlockPedagogicalIssues(t *testing.T) {
	a := &App{}
	args := agentCreateBlueprintSlotsArgs{
		ExamID: "",
		Slots: []agentBlueprintSlotDraft{{
			CapaianPembelajaran: "KD 3.1 contoh lama",
			ElemenCP:            "Pancasila",
			TujuanPembelajaran:  "Menganalisis nilai Pancasila",
			MateriPokok:         "Nilai Pancasila",
			CognitiveLevel:      "C4",
			IndikatorSoal:       "Peserta didik dapat menganalisis nilai Pancasila",
			QuestionType:        "multiple_choice",
		}},
	}
	fields := a.validateAgentCreateBlueprintSlotsArgs(t.Context(), "tenant-1", "user-1", args)
	if _, ok := fields["slots.0"]; ok {
		t.Fatalf("pedagogical/curriculum issues must be warnings, not proposal blockers: %#v", fields)
	}
	if _, ok := fields["examId"]; !ok {
		t.Fatalf("expected technical missing examId validation to remain blocking: %#v", fields)
	}
}

func TestAppendBlueprintSlotQualityWarningsIncludesFatalCurriculumIssues(t *testing.T) {
	args := appendBlueprintSlotQualityWarnings(agentCreateBlueprintSlotsArgs{Slots: []agentBlueprintSlotDraft{{
		Position:            21,
		CapaianPembelajaran: "KD 3.1 contoh lama",
		ElemenCP:            "Pancasila",
		TujuanPembelajaran:  "Menganalisis nilai Pancasila",
		MateriPokok:         "Nilai Pancasila",
		CognitiveLevel:      "C4",
		IndikatorSoal:       "Peserta didik dapat menganalisis nilai Pancasila",
		QuestionType:        "multiple_choice",
	}}})
	if len(args.Warnings) == 0 {
		t.Fatalf("expected curriculum issues to be surfaced as warnings")
	}
	if args.Warnings[0][:7] != "Slot 21" {
		t.Fatalf("expected actual slot number in warning, got %#v", args.Warnings)
	}
}
