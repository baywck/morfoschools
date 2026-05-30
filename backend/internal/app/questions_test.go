package app

import "testing"

func TestHashContent_NormalizesWhitespace(t *testing.T) {
	a := hashContent("Apa ibukota Indonesia?")
	b := hashContent("  Apa  ibukota  Indonesia?  ")
	c := hashContent("apa ibukota indonesia?")
	d := hashContent("APA IBUKOTA INDONESIA?")
	if a != b {
		t.Errorf("expected hash to ignore extra whitespace; %q vs %q", a, b)
	}
	if a != c || a != d {
		t.Errorf("expected hash to be case-insensitive; got %q / %q / %q", a, c, d)
	}
}

func TestHashContent_DistinguishesDifferentTexts(t *testing.T) {
	a := hashContent("Apa ibukota Indonesia?")
	b := hashContent("Apa ibukota Malaysia?")
	if a == b {
		t.Errorf("expected different content to hash differently")
	}
}

func TestHashContent_EmptyStringReturnsEmpty(t *testing.T) {
	if h := hashContent(""); h != "" {
		t.Errorf("expected empty hash for empty input, got %q", h)
	}
	if h := hashContent("   "); h != "" {
		t.Errorf("expected empty hash for whitespace-only input, got %q", h)
	}
}

func TestValidateQuestionPayload_MultipleChoice(t *testing.T) {
	cases := []struct {
		name    string
		options []questionOption
		mode    string
		wantErr string // expected key in errs map; empty = clean
	}{
		{
			name:    "valid mcq with one correct",
			options: []questionOption{{Content: "A", IsCorrect: true}, {Content: "B"}, {Content: "C"}, {Content: "D"}, {Content: "E"}},
			mode:    "correct_all",
			wantErr: "",
		},
		{
			name:    "multiple correct is invalid",
			options: []questionOption{{Content: "A", IsCorrect: true}, {Content: "B", IsCorrect: true}, {Content: "C"}, {Content: "D"}, {Content: "E"}},
			mode:    "percentage",
			wantErr: "options",
		},
		{
			name:    "too few options",
			options: []questionOption{{Content: "Only one", IsCorrect: true}},
			wantErr: "options",
		},
		{
			name: "no correct option",
			options: []questionOption{
				{Content: "A"}, {Content: "B"}, {Content: "C"}, {Content: "D"}, {Content: "E"},
			},
			wantErr: "options",
		},
		{
			name:    "invalid scoring mode",
			options: []questionOption{{Content: "A", IsCorrect: true}, {Content: "B"}, {Content: "C"}, {Content: "D"}, {Content: "E"}},
			mode:    "weird_mode",
			wantErr: "scoringMode",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := validateQuestionPayload("multiple_choice", "Q?", c.mode, c.options)
			_, has := errs[c.wantErr]
			if c.wantErr == "" && len(errs) > 0 {
				t.Errorf("expected clean, got %v", errs)
			}
			if c.wantErr != "" && !has {
				t.Errorf("expected error key %q, got keys %v", c.wantErr, errs)
			}
		})
	}
}

func TestValidateQuestionPayload_TrueFalse(t *testing.T) {
	clean := validateQuestionPayload("true_false", "Statement", "",
		[]questionOption{{Content: "True", IsCorrect: true}, {Content: "False"}})
	if len(clean) > 0 {
		t.Errorf("expected clean true_false, got %v", clean)
	}

	wrongCount := validateQuestionPayload("true_false", "Q", "",
		[]questionOption{{Content: "True", IsCorrect: true}})
	if _, ok := wrongCount["options"]; !ok {
		t.Errorf("expected options error for 1-option true_false, got %v", wrongCount)
	}

	bothCorrect := validateQuestionPayload("true_false", "Q", "",
		[]questionOption{{Content: "True", IsCorrect: true}, {Content: "False", IsCorrect: true}})
	if _, ok := bothCorrect["options"]; !ok {
		t.Errorf("expected options error when both true_false marked correct")
	}
}

func TestValidateQuestionPayload_FreeText(t *testing.T) {
	for _, qType := range []string{"short_answer", "essay"} {
		t.Run(qType, func(t *testing.T) {
			clean := validateQuestionPayload(qType, "Explain RBAC.", "", nil)
			if len(clean) > 0 {
				t.Errorf("expected clean %s, got %v", qType, clean)
			}
			rejectsOptions := validateQuestionPayload(qType, "Q", "",
				[]questionOption{{Content: "should not exist"}})
			if _, ok := rejectsOptions["options"]; !ok {
				t.Errorf("expected %s to reject options, got %v", qType, rejectsOptions)
			}
		})
	}
}

func TestValidateQuestionPayload_RequiresContent(t *testing.T) {
	errs := validateQuestionPayload("essay", "   ", "", nil)
	if _, ok := errs["content"]; !ok {
		t.Errorf("expected content error for whitespace-only, got %v", errs)
	}
}

// Post-state simulation — these mirror the option-CRUD post-state checks
// that protect MCQ/TF invariants from being broken by add/update/delete.
// We test the underlying validateQuestionPayload behaviour with simulated
// future option lists.

func TestPostStateValidation_MCQAddBeyondMax(t *testing.T) {
	// Start with 10 options and simulate adding an 11th.
	existing := make([]questionOption, 10)
	for i := range existing {
		existing[i] = questionOption{Content: "opt", IsCorrect: i == 0}
	}
	simulated := append(existing, questionOption{Content: "11th"})
	errs := validateQuestionPayload("multiple_choice", "Q?", "correct_all", simulated)
	if _, ok := errs["options"]; !ok {
		t.Errorf("expected options error for 11th option, got %v", errs)
	}
}

func TestPostStateValidation_MCQDeleteBelowMin(t *testing.T) {
	// Start with 2 options, simulate deleting one.
	existing := []questionOption{
		{ID: "a", Content: "A", IsCorrect: true},
		{ID: "b", Content: "B"},
	}
	simulated := []questionOption{existing[0]}
	errs := validateQuestionPayload("multiple_choice", "Q?", "correct_all", simulated)
	if _, ok := errs["options"]; !ok {
		t.Errorf("expected options error after delete to 1 option, got %v", errs)
	}
}

func TestPostStateValidation_MCQRemoveLastCorrect(t *testing.T) {
	// 2 options, only first correct; flip first to !correct — no correct left.
	existing := []questionOption{
		{ID: "a", Content: "A", IsCorrect: true},
		{ID: "b", Content: "B"},
	}
	simulated := []questionOption{
		{ID: "a", Content: "A", IsCorrect: false},
		existing[1],
	}
	errs := validateQuestionPayload("multiple_choice", "Q?", "correct_all", simulated)
	if _, ok := errs["options"]; !ok {
		t.Errorf("expected options error after removing last correct, got %v", errs)
	}
}

func TestPostStateValidation_TrueFalseFlipBothCorrect(t *testing.T) {
	simulated := []questionOption{
		{ID: "a", Content: "True", IsCorrect: true},
		{ID: "b", Content: "False", IsCorrect: true},
	}
	errs := validateQuestionPayload("true_false", "Q?", "", simulated)
	if _, ok := errs["options"]; !ok {
		t.Errorf("expected options error for 2-correct true_false, got %v", errs)
	}
}

func TestHasPermission(t *testing.T) {
	none := hasPermission(nil, "exams:write")
	if none {
		t.Errorf("nil auth should never have permissions")
	}
	auth := &AuthContext{Permissions: []string{"exams:read", "exams:write"}}
	if !hasPermission(auth, "exams:write") {
		t.Errorf("expected exams:write granted")
	}
	if hasPermission(auth, "users:write") {
		t.Errorf("expected users:write denied")
	}
}
