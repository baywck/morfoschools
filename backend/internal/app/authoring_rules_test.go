package app

import "testing"

func strptr(s string) *string { return &s }
func fptr(f float64) *float64 { return &f }

func TestValidateMerdekaKisiKisiPayloadSharedRules(t *testing.T) {
	errs := validateMerdekaKisiKisiPayload(slotPayload{
		IndikatorSoal:  strptr("Peserta didik menganalisis data"),
		CognitiveLevel: strptr("C7"),
		Difficulty:     strptr("rumit"),
		QuestionType:   strptr("matching"),
		Kelas:          strptr("13"),
		Semester:       strptr("3"),
		Points:         fptr(-1),
	})
	for _, field := range []string{"indikatorSoal", "cognitiveLevel", "difficulty", "questionType", "kelas", "semester", "points"} {
		if errs[field] == "" {
			t.Fatalf("expected %s validation error, got %#v", field, errs)
		}
	}
}

func TestValidateQuestionPayloadRequiresFiveMCQOptionsAndOneCorrect(t *testing.T) {
	four := []questionOption{
		{Content: "A", IsCorrect: true},
		{Content: "B"},
		{Content: "C"},
		{Content: "D"},
	}
	if errs := validateQuestionPayload("multiple_choice", "Disajikan konteks yang cukup panjang. Peserta didik memilih jawaban.", "", four); errs["options"] == "" {
		t.Fatalf("expected five-option validation error, got %#v", errs)
	}
	fiveTwoCorrect := append(four, questionOption{Content: "E", IsCorrect: true})
	if errs := validateQuestionPayload("multiple_choice", "Disajikan konteks yang cukup panjang. Peserta didik memilih jawaban.", "", fiveTwoCorrect); errs["options"] == "" {
		t.Fatalf("expected exactly-one-correct validation error, got %#v", errs)
	}
}
