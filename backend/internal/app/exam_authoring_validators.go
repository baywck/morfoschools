package app

import (
	"fmt"
	"strings"
	"unicode"
)

type generatedQuestionDraft struct {
	QuestionType  string
	Content       string
	Explanation   string
	CorrectAnswer string
	Options       []questionOption
	Grouped       bool
}

type authoringValidationIssue struct {
	Field    string
	Severity string // error | warning
	Message  string
}

func validateGeneratedQuestionDraft(policy examAuthoringPolicy, d generatedQuestionDraft) []authoringValidationIssue {
	issues := make([]authoringValidationIssue, 0, 4)
	qt := strings.TrimSpace(d.QuestionType)
	if qt == "" {
		qt = policy.QuestionRules.DefaultQuestionType
	}
	if strings.TrimSpace(d.Content) == "" {
		issues = append(issues, authoringValidationIssue{Field: "content", Severity: "error", Message: "content wajib diisi"})
		return issues
	}
	if (qt == "short_answer" || qt == "essay") && !policy.QuestionRules.AllowConstructedResponse {
		issues = append(issues, authoringValidationIssue{Field: "questionType", Severity: "error", Message: "short_answer/essay hanya boleh jika user eksplisit meminta bentuk jawaban uraian"})
	}
	if qt == "multiple_choice" {
		if len(d.Options) < 2 {
			issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "multiple_choice wajib memiliki minimal 2 opsi"})
		}
		correct := 0
		seen := map[string]bool{}
		lengths := make([]int, 0, len(d.Options))
		numericCount := 0
		for _, opt := range d.Options {
			content := strings.TrimSpace(opt.Content)
			if content == "" {
				issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "opsi tidak boleh kosong"})
			}
			key := strings.ToLower(content)
			if key != "" && seen[key] {
				issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "opsi tidak boleh duplikat"})
			}
			if containsStructuralOptionClue(key) {
				issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "opsi tidak boleh memakai petunjuk struktural seperti semua benar/semua salah/A dan B"})
			}
			seen[key] = true
			if opt.IsCorrect {
				correct++
			}
			if content != "" {
				lengths = append(lengths, countWords(content))
				if looksNumericOption(content) {
					numericCount++
				}
			}
		}
		if correct != 1 {
			issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "multiple_choice wajib memiliki tepat 1 opsi benar"})
		}
		if policy.QuestionRules.RequireHomogeneousOptions {
			issues = append(issues, validateOptionHomogeneity(lengths, numericCount)...)
		}
	}
	if policy.QuestionRules.RequireExplanation && strings.TrimSpace(d.Explanation) == "" {
		issues = append(issues, authoringValidationIssue{Field: "explanation", Severity: "warning", Message: "soal AI sebaiknya memiliki pembahasan singkat"})
	}
	if !d.Grouped {
		words := countWords(d.Content)
		if words < policy.QuestionRules.MinStandaloneWords {
			issues = append(issues, authoringValidationIssue{Field: "content", Severity: "warning", Message: "stem/konteks soal standalone terlalu pendek"})
		}
		if countSentences(d.Content) < policy.QuestionRules.MinStandaloneSent {
			issues = append(issues, authoringValidationIssue{Field: "content", Severity: "warning", Message: "stem/konteks soal standalone sebaiknya minimal 2 kalimat"})
		}
	}
	return issues
}

func appendAuthoringWarnings(sb *strings.Builder, issues []authoringValidationIssue) {
	warnCount := 0
	for _, issue := range issues {
		if issue.Severity != "warning" {
			continue
		}
		if warnCount == 0 {
			sb.WriteString("\n⚠️ **Catatan kualitas:**\n")
		}
		warnCount++
		sb.WriteString(fmt.Sprintf("- %s: %s\n", issue.Field, issue.Message))
	}
}

func containsStructuralOptionClue(lower string) bool {
	clues := []string{
		"semua jawaban benar", "semua benar", "semuanya benar",
		"semua jawaban salah", "semua salah", "tidak ada jawaban",
		"a dan b", "b dan c", "c dan d", "a, b", "b, c", "c, d",
	}
	for _, clue := range clues {
		if strings.Contains(lower, clue) {
			return true
		}
	}
	return false
}

func looksNumericOption(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	numeric := 0
	letters := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			numeric++
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return numeric > 0 && letters <= 3
}

func validateOptionHomogeneity(lengths []int, numericCount int) []authoringValidationIssue {
	if len(lengths) < 4 {
		return nil
	}
	minLen, maxLen := lengths[0], lengths[0]
	for _, l := range lengths[1:] {
		if l < minLen {
			minLen = l
		}
		if l > maxLen {
			maxLen = l
		}
	}
	issues := []authoringValidationIssue{}
	if minLen > 0 && maxLen > minLen*3 && maxLen-minLen >= 5 {
		issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "opsi harus homogen dengan panjang/detail yang seimbang"})
	}
	if numericCount > 0 && numericCount < len(lengths) {
		issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "opsi numerik harus homogen; jangan campur opsi angka dengan non-angka"})
	}
	return issues
}

func countWords(s string) int {
	return len(strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }))
}

func countSentences(s string) int {
	count := 0
	for _, r := range s {
		if r == '.' || r == '?' || r == '!' || r == '।' {
			count++
		}
	}
	return count
}
