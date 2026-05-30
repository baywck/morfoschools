package app

import "testing"

func TestValidateGeneratedQuestionDraft_WarnsShortStandaloneStem(t *testing.T) {
	policy := defaultExamAuthoringPolicy("tenant", "exam")
	issues := validateGeneratedQuestionDraft(policy, generatedQuestionDraft{
		QuestionType: "multiple_choice",
		Content:      "Apa makna demokrasi Pancasila?",
		Explanation:  "Demokrasi Pancasila menekankan musyawarah dan keadilan.",
		Options: []questionOption{
			{Content: "Musyawarah", IsCorrect: true},
			{Content: "Otoritarianisme"},
			{Content: "Monarki absolut"},
			{Content: "Anarki"},
		},
	})
	if !hasIssue(issues, "content", "warning") {
		t.Fatalf("expected warning for short standalone stem, got %#v", issues)
	}
}

func TestValidateGeneratedQuestionDraft_GroupedStemMayBeShort(t *testing.T) {
	policy := defaultExamAuthoringPolicy("tenant", "exam")
	issues := validateGeneratedQuestionDraft(policy, generatedQuestionDraft{
		QuestionType: "multiple_choice",
		Content:      "Simpulan paling tepat adalah...",
		Explanation:  "Jawaban menuntut analisis stimulus kelompok.",
		Grouped:      true,
		Options: []questionOption{
			{Content: "A", IsCorrect: true}, {Content: "B"}, {Content: "C"}, {Content: "D"},
		},
	})
	if hasIssue(issues, "content", "warning") {
		t.Fatalf("did not expect short-stem warning for grouped question, got %#v", issues)
	}
}

func TestValidateGeneratedQuestionDraft_MCQRequiresExactlyOneCorrect(t *testing.T) {
	policy := defaultExamAuthoringPolicy("tenant", "exam")
	issues := validateGeneratedQuestionDraft(policy, generatedQuestionDraft{
		QuestionType: "multiple_choice",
		Content:      "Dalam suatu kasus pelanggaran HAM di sekolah, OSIS mengusulkan musyawarah dengan semua pihak. Keputusan perlu mempertimbangkan hak korban dan kewajiban pelaku. Pendekatan mana yang paling sesuai dengan nilai Pancasila?",
		Explanation:  "Pembahasan singkat.",
		Options: []questionOption{
			{Content: "A", IsCorrect: true}, {Content: "B", IsCorrect: true}, {Content: "C"}, {Content: "D"},
		},
	})
	if !hasIssue(issues, "options", "error") {
		t.Fatalf("expected error for multiple correct options, got %#v", issues)
	}
}

func hasIssue(issues []authoringValidationIssue, field, severity string) bool {
	for _, issue := range issues {
		if issue.Field == field && issue.Severity == severity {
			return true
		}
	}
	return false
}
