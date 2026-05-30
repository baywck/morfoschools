package app

import (
	"regexp"
	"strings"
)

type questionQualityIssue struct {
	Code     string                  `json:"code"`
	Severity curriculumIssueSeverity `json:"severity"`
	Field    string                  `json:"field,omitempty"`
	Message  string                  `json:"message"`
}

type agentQuestionOptionDraft struct {
	Text      string `json:"text"`
	IsCorrect bool   `json:"isCorrect"`
}

type agentQuestionDraft struct {
	Content        string                     `json:"content"`
	QuestionType   string                     `json:"questionType"`
	CognitiveLevel string                     `json:"cognitiveLevel,omitempty"`
	Stimulus       string                     `json:"stimulus,omitempty"`
	Options        []agentQuestionOptionDraft `json:"options,omitempty"`
	Explanation    string                     `json:"explanation,omitempty"`
}

type agentBlueprintSlotDraft struct {
	Position            int    `json:"position"`
	CapaianPembelajaran string `json:"capaianPembelajaran"`
	ElemenCP            string `json:"elemenCp"`
	TujuanPembelajaran  string `json:"tujuanPembelajaran"`
	MateriPokok         string `json:"materiPokok"`
	KelasSemester       string `json:"kelasSemester,omitempty"`
	CognitiveLevel      string `json:"cognitiveLevel"`
	IndikatorSoal       string `json:"indikatorSoal"`
	QuestionType        string `json:"questionType"`
	Points              int    `json:"points,omitempty"`
	SourceConfidence    string `json:"sourceConfidence,omitempty"`
}

var doubleNegativePattern = regexp.MustCompile(`(?i)\b(bukan\s+bukan|tidak\s+.*\s+bukan|bukan\s+.*\s+kecuali)\b`)

func validateAgentQuestionQuality(q agentQuestionDraft, slot *agentBlueprintSlotDraft) []questionQualityIssue {
	var issues []questionQualityIssue
	if strings.TrimSpace(q.Content) == "" {
		issues = append(issues, questionQualityIssue{Code: "empty_content", Severity: curriculumIssueError, Field: "content", Message: "Konten soal wajib diisi"})
	}
	level := normalizeCognitiveLevel(q.CognitiveLevel)
	if slot != nil {
		level = normalizeCognitiveLevel(slot.CognitiveLevel)
		if strings.TrimSpace(q.QuestionType) != "" && strings.TrimSpace(slot.QuestionType) != "" && !sameQuestionType(q.QuestionType, slot.QuestionType) {
			issues = append(issues, questionQualityIssue{Code: "slot_question_type_mismatch", Severity: curriculumIssueError, Field: "questionType", Message: "Bentuk soal tidak sesuai dengan slot kisi-kisi"})
		}
	}
	if isHOTSLevel(level) && !questionHasStimulus(q, slot) {
		issues = append(issues, questionQualityIssue{Code: "hots_missing_stimulus", Severity: curriculumIssueError, Field: "stimulus", Message: "Soal HOTS C4-C6 wajib memiliki stimulus kontekstual"})
	}
	if sameQuestionType(q.QuestionType, "multiple_choice") {
		issues = append(issues, validateMCQDraft(q.Options)...)
	}
	if doubleNegativePattern.MatchString(q.Content) {
		issues = append(issues, questionQualityIssue{Code: "double_negative", Severity: curriculumIssueWarning, Field: "content", Message: "Kalimat soal terdeteksi memakai negasi ganda; perjelas bahasa soal"})
	}
	if len([]rune(stripTags(q.Content))) > 700 {
		issues = append(issues, questionQualityIssue{Code: "long_stem", Severity: curriculumIssueWarning, Field: "content", Message: "Stem soal cukup panjang; pastikan tetap jelas dan tidak berbelit"})
	}
	if referencesOtherQuestion(q.Content) {
		issues = append(issues, questionQualityIssue{Code: "not_independent", Severity: curriculumIssueError, Field: "content", Message: "Soal harus mandiri dan tidak bergantung pada soal lain"})
	}
	return issues
}

func validateMCQDraft(options []agentQuestionOptionDraft) []questionQualityIssue {
	var issues []questionQualityIssue
	if len(options) < 2 {
		issues = append(issues, questionQualityIssue{Code: "mcq_min_options", Severity: curriculumIssueError, Field: "options", Message: "Pilihan ganda membutuhkan minimal 2 opsi"})
	}
	correct := 0
	seen := map[string]bool{}
	for _, opt := range options {
		text := normalizeOptionText(opt.Text)
		if text == "" {
			issues = append(issues, questionQualityIssue{Code: "mcq_empty_option", Severity: curriculumIssueError, Field: "options", Message: "Opsi pilihan ganda tidak boleh kosong"})
			continue
		}
		if seen[text] {
			issues = append(issues, questionQualityIssue{Code: "mcq_duplicate_option", Severity: curriculumIssueError, Field: "options", Message: "Opsi pilihan ganda tidak boleh sama/duplikat"})
		}
		seen[text] = true
		if opt.IsCorrect {
			correct++
		}
	}
	if correct != 1 {
		issues = append(issues, questionQualityIssue{Code: "mcq_one_correct", Severity: curriculumIssueError, Field: "options", Message: "Pilihan ganda harus memiliki tepat 1 jawaban benar"})
	}
	return issues
}

func isHOTSLevel(level string) bool {
	switch normalizeCognitiveLevel(level) {
	case "C4", "C5", "C6":
		return true
	default:
		return false
	}
}

func questionHasStimulus(q agentQuestionDraft, slot *agentBlueprintSlotDraft) bool {
	if strings.TrimSpace(q.Stimulus) != "" || hasStimulusPhrase(q.Content) {
		return true
	}
	return slot != nil && hasStimulusPhrase(slot.IndikatorSoal)
}

func sameQuestionType(a, b string) bool {
	return normalizeQuestionType(a) == normalizeQuestionType(b)
}

func normalizeQuestionType(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "_")
	switch v {
	case "pg", "pilihan_ganda", "multiple-choice":
		return "multiple_choice"
	case "bs", "benar_salah", "true-false":
		return "true_false"
	case "isian", "isian_singkat":
		return "short_answer"
	case "uraian":
		return "essay"
	default:
		return v
	}
}

func normalizeOptionText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(stripTags(s))), " ")
}

func referencesOtherQuestion(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "soal sebelumnya") || strings.Contains(lower, "pertanyaan sebelumnya") || strings.Contains(lower, "soal nomor")
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func hasBlockingQuestionQualityIssues(issues []questionQualityIssue) bool {
	for _, issue := range issues {
		if issue.Severity == curriculumIssueError {
			return true
		}
	}
	return false
}
