package app

import (
	"regexp"
	"strings"
)

type curriculumIssueSeverity string

const (
	curriculumIssueError   curriculumIssueSeverity = "error"
	curriculumIssueWarning curriculumIssueSeverity = "warning"
)

type curriculumIssue struct {
	Code     string                  `json:"code"`
	Severity curriculumIssueSeverity `json:"severity"`
	Field    string                  `json:"field,omitempty"`
	Message  string                  `json:"message"`
}

var bloomKKOByLevel = map[string][]string{
	"C1": {"menyebutkan", "mendefinisikan", "menuliskan", "mengidentifikasi"},
	"C2": {"menjelaskan", "merangkum", "mengklasifikasikan", "membedakan"},
	"C3": {"menggunakan", "melaksanakan", "menerapkan", "menyelesaikan"},
	"C4": {"menganalisis", "membandingkan", "menguraikan", "menelaah", "mengkritisi"},
	"C5": {"menilai", "memutuskan", "mempertahankan", "mengkritik", "menyimpulkan"},
	"C6": {"merancang", "menyusun", "mengonstruksi", "merumuskan", "mengembangkan"},
}

var forbiddenKDCurriculumPattern = regexp.MustCompile(`(?i)\b(KD|SK|Kompetensi\s+Dasar|Standar\s+Kompetensi)\b|\bKD\s*[0-9]+\.[0-9]+\b`)

func validateKurikulumMerdekaBlueprintSlot(slot agentBlueprintSlotDraft) []curriculumIssue {
	var issues []curriculumIssue
	fields := map[string]string{
		"capaianPembelajaran": slot.CapaianPembelajaran,
		"elemenCp":            slot.ElemenCP,
		"tujuanPembelajaran":  slot.TujuanPembelajaran,
		"materiPokok":         slot.MateriPokok,
		"indikatorSoal":       slot.IndikatorSoal,
		"cognitiveLevel":      slot.CognitiveLevel,
		"questionType":        slot.QuestionType,
	}
	for field, value := range fields {
		if strings.TrimSpace(value) == "" {
			issues = append(issues, curriculumIssue{Code: "required", Severity: curriculumIssueError, Field: field, Message: field + " wajib diisi untuk kisi-kisi Kurikulum Merdeka"})
		}
		if forbiddenKDCurriculumPattern.MatchString(value) {
			issues = append(issues, curriculumIssue{Code: "forbidden_kd_sk", Severity: curriculumIssueError, Field: field, Message: "Kurikulum Merdeka tidak menggunakan KD/SK; gunakan CP, Elemen CP, dan TP"})
		}
	}
	level := normalizeCognitiveLevel(slot.CognitiveLevel)
	if level != "" && bloomKKOByLevel[level] == nil {
		issues = append(issues, curriculumIssue{Code: "invalid_cognitive_level", Severity: curriculumIssueError, Field: "cognitiveLevel", Message: "Level kognitif harus C1-C6"})
	}
	if strings.TrimSpace(slot.IndikatorSoal) != "" && !hasStimulusPhrase(slot.IndikatorSoal) {
		issues = append(issues, curriculumIssue{Code: "indicator_missing_stimulus", Severity: curriculumIssueError, Field: "indikatorSoal", Message: "Indikator soal wajib menyebut stimulus, misalnya: Disajikan [stimulus], peserta didik dapat ..."})
	}
	if strings.TrimSpace(slot.TujuanPembelajaran) != "" {
		if !hasTPAudience(slot.TujuanPembelajaran) {
			issues = append(issues, curriculumIssue{Code: "tp_missing_audience", Severity: curriculumIssueError, Field: "tujuanPembelajaran", Message: "TP wajib menyebut Audience, minimal 'Peserta didik'"})
		}
		if !hasAnyBloomKKO(slot.TujuanPembelajaran) {
			issues = append(issues, curriculumIssue{Code: "tp_missing_behavior", Severity: curriculumIssueError, Field: "tujuanPembelajaran", Message: "TP wajib memuat Behavior berupa KKO yang terukur"})
		}
	}
	if level != "" {
		if !containsKKOForLevel(slot.TujuanPembelajaran, level) {
			issues = append(issues, curriculumIssue{Code: "tp_kko_mismatch", Severity: curriculumIssueWarning, Field: "tujuanPembelajaran", Message: "KKO pada TP belum tampak selaras dengan level " + level})
		}
		if !containsKKOForLevel(slot.IndikatorSoal, level) {
			issues = append(issues, curriculumIssue{Code: "indicator_kko_mismatch", Severity: curriculumIssueWarning, Field: "indikatorSoal", Message: "KKO pada indikator belum tampak selaras dengan level " + level})
		}
	}
	if hasMultipleCompetencyConjunctions(slot.TujuanPembelajaran) {
		issues = append(issues, curriculumIssue{Code: "tp_multiple_competencies", Severity: curriculumIssueWarning, Field: "tujuanPembelajaran", Message: "Satu TP sebaiknya hanya memuat satu kompetensi; pecah TP bila ada beberapa KKO utama"})
	}
	return issues
}

func normalizeCognitiveLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))
	if len(level) >= 2 && level[0] == 'C' && level[1] >= '1' && level[1] <= '6' {
		return level[:2]
	}
	return level
}

func hasStimulusPhrase(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "disajikan") || strings.Contains(lower, "berdasarkan stimulus") || strings.Contains(lower, "berdasarkan wacana") || strings.Contains(lower, "berdasarkan kasus") || strings.Contains(lower, "berdasarkan data")
}

func hasTPAudience(tp string) bool {
	lower := strings.ToLower(tp)
	return strings.Contains(lower, "peserta didik") || strings.Contains(lower, "murid") || strings.Contains(lower, "siswa")
}

func hasAnyBloomKKO(text string) bool {
	lower := strings.ToLower(text)
	for _, verbs := range bloomKKOByLevel {
		for _, verb := range verbs {
			if strings.Contains(lower, verb) {
				return true
			}
		}
	}
	return false
}

func containsKKOForLevel(text, level string) bool {
	verbs := bloomKKOByLevel[normalizeCognitiveLevel(level)]
	if len(verbs) == 0 {
		return false
	}
	lower := strings.ToLower(text)
	for _, verb := range verbs {
		if strings.Contains(lower, verb) {
			return true
		}
	}
	return false
}

func hasMultipleCompetencyConjunctions(tp string) bool {
	lower := strings.ToLower(tp)
	count := 0
	for _, verbs := range bloomKKOByLevel {
		for _, verb := range verbs {
			if strings.Contains(lower, verb) {
				count++
			}
		}
	}
	return count > 1 && (strings.Contains(lower, " dan ") || strings.Contains(lower, ","))
}

func hasBlockingCurriculumIssues(issues []curriculumIssue) bool {
	for _, issue := range issues {
		if issue.Severity == curriculumIssueError {
			return true
		}
	}
	return false
}
