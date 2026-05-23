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
	if qt == "multiple_choice" {
		if policy.QuestionRules.RequireFourOptions && len(d.Options) < 4 {
			issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "multiple_choice wajib memiliki minimal 4 opsi"})
		}
		correct := 0
		seen := map[string]bool{}
		for _, opt := range d.Options {
			content := strings.TrimSpace(opt.Content)
			if content == "" {
				issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "opsi tidak boleh kosong"})
			}
			key := strings.ToLower(content)
			if key != "" && seen[key] {
				issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "opsi tidak boleh duplikat"})
			}
			seen[key] = true
			if opt.IsCorrect {
				correct++
			}
		}
		if correct != 1 {
			issues = append(issues, authoringValidationIssue{Field: "options", Severity: "error", Message: "multiple_choice wajib memiliki tepat 1 opsi benar"})
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
