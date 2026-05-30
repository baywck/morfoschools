package app

import "testing"

func TestValidateKurikulumMerdekaBlueprintSlotRejectsKDAndMissingStimulus(t *testing.T) {
	slot := agentBlueprintSlotDraft{
		CapaianPembelajaran: "KD 3.2 Menganalisis hak asasi manusia",
		ElemenCP:            "UUD NRI 1945",
		TujuanPembelajaran:  "Peserta didik dapat menganalisis mekanisme penegakan HAM di Indonesia berdasarkan kasus nyata dengan tepat",
		MateriPokok:         "Penegakan HAM",
		CognitiveLevel:      "C4",
		IndikatorSoal:       "Peserta didik dapat menganalisis pelanggaran HAM",
		QuestionType:        "multiple_choice",
	}
	issues := validateKurikulumMerdekaBlueprintSlot(slot)
	if !hasBlockingCurriculumIssues(issues) {
		t.Fatalf("expected blocking issues, got %#v", issues)
	}
	if !hasCurriculumIssue(issues, "forbidden_kd_sk") || !hasCurriculumIssue(issues, "indicator_missing_stimulus") {
		t.Fatalf("expected KD/SK and missing stimulus issues, got %#v", issues)
	}
}

func TestValidateKurikulumMerdekaBlueprintSlotAcceptsAlignedStimulusIndicator(t *testing.T) {
	slot := agentBlueprintSlotDraft{
		CapaianPembelajaran: "Peserta didik mampu menganalisis dinamika pelaksanaan hak dan kewajiban warga negara dalam kehidupan demokratis.",
		ElemenCP:            "UUD NRI 1945",
		TujuanPembelajaran:  "Peserta didik dapat menganalisis mekanisme penegakan HAM di Indonesia berdasarkan kasus nyata dengan tepat",
		MateriPokok:         "Mekanisme penegakan HAM di Indonesia",
		CognitiveLevel:      "C4",
		IndikatorSoal:       "Disajikan wacana tentang kasus pelanggaran HAM, peserta didik dapat menganalisis bentuk pelanggaran dan upaya penyelesaiannya.",
		QuestionType:        "multiple_choice",
	}
	issues := validateKurikulumMerdekaBlueprintSlot(slot)
	if hasBlockingCurriculumIssues(issues) {
		t.Fatalf("did not expect blocking issues, got %#v", issues)
	}
}

func TestValidateKurikulumMerdekaBlueprintSlotWarnsDegreeAndKKOMismatch(t *testing.T) {
	slot := agentBlueprintSlotDraft{
		CapaianPembelajaran: "Peserta didik mampu menganalisis fenomena sesuai konteks pembelajaran.",
		ElemenCP:            "Elemen CP",
		TujuanPembelajaran:  "Peserta didik dapat merumuskan solusi berdasarkan studi kasus",
		MateriPokok:         "Materi kontekstual",
		CognitiveLevel:      "C3",
		IndikatorSoal:       "Disajikan studi kasus, peserta didik dapat merumuskan solusi berdasarkan data yang tersedia.",
		QuestionType:        "essay",
	}
	issues := validateKurikulumMerdekaBlueprintSlot(slot)
	if hasBlockingCurriculumIssues(issues) {
		t.Fatalf("did not expect pedagogical warnings to block proposal, got %#v", issues)
	}
	if !hasCurriculumIssue(issues, "tp_kko_mismatch") || !hasCurriculumIssue(issues, "indicator_kko_mismatch") || !hasCurriculumIssue(issues, "tp_missing_degree") {
		t.Fatalf("expected KKO mismatch and missing degree warnings, got %#v", issues)
	}
}

func TestValidateAgentQuestionQualityBlocksHOTSWithoutStimulus(t *testing.T) {
	slot := agentBlueprintSlotDraft{CognitiveLevel: "C4", QuestionType: "multiple_choice", IndikatorSoal: "Peserta didik dapat menganalisis pelanggaran HAM"}
	q := agentQuestionDraft{Content: "Apa pengertian HAM?", QuestionType: "multiple_choice", Options: []agentQuestionOptionDraft{{Text: "Hak dasar", IsCorrect: true}, {Text: "Kewajiban", IsCorrect: false}}}
	issues := validateAgentQuestionQuality(q, &slot)
	if !hasBlockingQuestionQualityIssues(issues) || !hasQIssue(issues, "hots_missing_stimulus") {
		t.Fatalf("expected HOTS missing stimulus block, got %#v", issues)
	}
}

func TestValidateAgentQuestionQualityBlocksDuplicateOptionsAndMultipleCorrect(t *testing.T) {
	q := agentQuestionDraft{Content: "Disajikan kasus, peserta didik dapat menganalisis tindakan yang tepat.", QuestionType: "multiple_choice", CognitiveLevel: "C4", Options: []agentQuestionOptionDraft{{Text: "Melapor ke Komnas HAM", IsCorrect: true}, {Text: "Melapor ke Komnas HAM", IsCorrect: true}, {Text: "Membiarkan", IsCorrect: false}}}
	issues := validateAgentQuestionQuality(q, nil)
	if !hasBlockingQuestionQualityIssues(issues) {
		t.Fatalf("expected blocking issues, got %#v", issues)
	}
	if !hasQIssue(issues, "mcq_duplicate_option") || !hasQIssue(issues, "mcq_one_correct") {
		t.Fatalf("expected duplicate and one-correct issues, got %#v", issues)
	}
}

func hasCurriculumIssue(issues []curriculumIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func hasQIssue(issues []questionQualityIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
