package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type autoKisiItem struct {
	CompetencyCode        string
	CompetencyDescription string
	Materi                string
	Indikator             string
	CognitiveLevel        string
	Difficulty            string
	QuestionType          string
	points                float64
}

func kisiItemFromQuestionMap(questionID string, qm map[string]any, pos int) autoKisiItem {
	get := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := qm[k]; ok {
				if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
					return s
				}
			}
		}
		return ""
	}
	materi := get("materi", "topic")
	if materi == "" {
		materi = "Topik sesuai konteks soal"
	}
	indikator := get("indikator", "indicator")
	if indikator == "" {
		indikator = "Menganalisis informasi kontekstual pada soal untuk menentukan jawaban yang tepat"
	}
	qtype := get("questionType")
	if qtype == "" {
		qtype = "multiple_choice"
	}
	points := 1.0
	if v, ok := qm["points"].(float64); ok && v > 0 {
		points = v
	}
	code := get("competencyCode")
	if code == "" {
		code = fmt.Sprintf("KOMP-%03d", pos+1)
	}
	desc := get("competencyDescription")
	code, desc, materi, indikator = normalizeKisiKisiFields(code, desc, materi, indikator)
	if desc == "" {
		desc = synthesizeCompetencyDescription(materi, indikator)
	}
	cog := get("cognitiveLevel")
	if cog == "" {
		cog = "C3"
	}
	diff := get("difficulty")
	if diff == "" {
		diff = "sedang"
	}
	return autoKisiItem{code, desc, materi, indikator, cog, diff, qtype, points}
}

func questionHasBlueprintSlot(ctx context.Context, tx *sql.Tx, questionID string) bool {
	var has bool
	_ = tx.QueryRowContext(ctx, `SELECT blueprint_slot_id IS NOT NULL FROM exam_questions WHERE id=$1`, questionID).Scan(&has)
	return has
}

func normalizeKisiKisiFields(code, desc, materi, indikator string) (string, string, string, string) {
	code = strings.TrimSpace(code)
	desc = strings.TrimSpace(desc)
	materi = stripKDLabelFromMateri(strings.TrimSpace(materi))
	indikator = strings.TrimSpace(indikator)
	if desc == "" || normalizedKisiText(desc) == normalizedKisiText(materi) || looksLikeGenericCompetencyDescription(desc, materi) {
		desc = synthesizeCompetencyDescription(materi, indikator)
	}
	if normalizedKisiText(desc) == normalizedKisiText(materi) {
		desc = "Menganalisis konsep dan penerapan " + materi
	}
	return code, desc, materi, indikator
}

func stripKDLabelFromMateri(s string) string {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	for _, prefix := range []string{"kd ", "kd.", "kompetensi dasar "} {
		if strings.HasPrefix(lower, prefix) {
			rest := strings.TrimSpace(s[len(prefix):])
			parts := strings.Fields(rest)
			if len(parts) > 1 && strings.ContainsAny(parts[0], ".0123456789") {
				return strings.TrimSpace(strings.TrimPrefix(rest, parts[0]))
			}
		}
	}
	return s
}

func normalizedKisiText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func looksLikeGenericCompetencyDescription(desc, materi string) bool {
	d := normalizedKisiText(desc)
	m := normalizedKisiText(materi)
	if strings.Contains(d, "memahami dan menerapkan konsep") && strings.Contains(d, "sesuai kompetensi") {
		return true
	}
	return m != "" && strings.Contains(d, "memahami dan menerapkan konsep "+m+" sesuai kompetensi")
}

func synthesizeCompetencyDescription(materi, indikator string) string {
	materi = strings.TrimSpace(materi)
	indikator = strings.TrimSpace(indikator)
	if indikator != "" {
		return strings.TrimSuffix(indikator, ".")
	}
	if materi == "" {
		return "Menganalisis kompetensi yang diukur dari butir soal"
	}
	return "Menganalisis konsep dan penerapan " + materi
}
