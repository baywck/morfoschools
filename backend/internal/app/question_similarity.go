package app

import (
	"context"
	"regexp"
	"strings"
)

// Content normalization + similarity helpers (Phase 9.13).
//
// Why this exists: AI agents on exams with 50+ questions occasionally
// propose paraphrase duplicates that slip past md5(lower(trim(content)))
// because the original hash didn't strip:
//   - HTML tags (RichEditor wraps prose in <p>...)
//   - LaTeX math wrappers (when text inside formulas equals each other)
//   - Punctuation differences (?, ., !)
//   - Polite imperative prefixes (Sebutkan, Tuliskan, Jelaskan, Apa, ...)
//   - Repeated whitespace
//   - Unicode look-alikes (smart quotes, full-width digits)
//
// normalizeQuestionContent() collapses all of those so the hash is
// stable across every minor reword. The string is also stored in
// content_normalized column (migration 000019) for trigram similarity
// ranking.
//
// Note: this normalization is destructive — it produces a 'fingerprint'
// not a display string. The original content is preserved in
// exam_questions.content untouched.

var (
	htmlTagRe   = regexp.MustCompile(`<[^>]+>`)
	mathBlockRe = regexp.MustCompile(`\$\$[^$]+\$\$`)
	mathInlineRe = regexp.MustCompile(`\$[^$\n]+\$`)
	wsRe        = regexp.MustCompile(`\s+`)
	punctRe     = regexp.MustCompile(`[\p{P}\p{S}]+`)
)

// Politeness prefixes that frequently appear at the start of question
// stems and don't carry semantic load. We strip them so "Sebutkan ibu
// kota Indonesia" and "Apa ibu kota Indonesia?" produce the same
// fingerprint. Order matters: longer phrases first so we strip greedy.
//
// Stripping is iterated up to 3 times so chained prefixes like
// "Sebutkan dengan jelas, manakah ..." collapse to the semantic core.
var politenessPrefixes = []string{
	"jelaskan secara singkat dan padat",
	"jelaskan secara rinci dan jelas",
	"jelaskan dengan singkat dan jelas",
	"jelaskan secara singkat",
	"jelaskan secara rinci",
	"jelaskan secara detail",
	"jelaskan dengan rinci",
	"jelaskan dengan jelas",
	"jelaskan dengan detail",
	"jelaskan dengan lengkap",
	"berikan contoh dari",
	"berikan contoh konkret dari",
	"sebutkan dan jelaskan",
	"sebutkan dengan jelas",
	"sebutkan secara lengkap",
	"sebutkan secara rinci",
	"sebutkan dengan rinci",
	"sebutkan dengan detail",
	"manakah yang paling tepat",
	"manakah yang paling benar",
	"manakah yang lebih tepat",
	"manakah yang tepat",
	"manakah yang benar",
	"manakah yang merupakan",
	"berdasarkan teks tersebut",
	"berdasarkan stimulus tersebut",
	"berdasarkan teks di atas",
	"berdasarkan bacaan tersebut",
	"berdasarkan informasi tersebut",
	"berdasarkan informasi di atas",
	"berdasarkan stimulus",
	"berdasarkan teks",
	"berdasarkan bacaan",
	"perhatikan gambar berikut",
	"perhatikan tabel berikut",
	"perhatikan diagram berikut",
	"hitunglah nilai dari",
	"hitunglah hasil dari",
	"hitunglah berapa",
	"tentukan nilai dari",
	"tentukan hasil dari",
	"tentukan berapa",
	"dengan jelas",
	"dengan tepat",
	"dengan rinci",
	"dengan detail",
	"secara singkat",
	"secara rinci",
	"secara detail",
	"secara lengkap",
	"jelaskan",
	"sebutkan",
	"tuliskan",
	"uraikan",
	"hitunglah",
	"tentukan",
	"manakah",
	"apakah",
	"berikan",
	"bagaimana",
	"mengapa",
	"kapan",
	"dimana",
	"di mana",
	"siapa",
	"apa",
}

// NormalizeQuestionContent is the exported alias for tools (e.g. the
// one-shot normalize-questions cmd) that need to recompute the
// canonical fingerprint outside the request flow.
func NormalizeQuestionContent(s string) string { return normalizeQuestionContent(s) }

// normalizeQuestionContent produces a canonical fingerprint for
// duplicate detection. Idempotent: normalize(normalize(x)) == normalize(x).
func normalizeQuestionContent(s string) string {
	if s == "" {
		return ""
	}
	// 1. Strip HTML tags (RichEditor wraps in <p>, <br>, etc.)
	s = htmlTagRe.ReplaceAllString(s, " ")
	// 2. Strip LaTeX math wrappers — keep the inner formula tokens so
	//    questions that differ only in the text AROUND the formula are
	//    distinguished, but identical formulas don't collide on $ vs $$.
	s = mathBlockRe.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Trim(m, "$")
	})
	s = mathInlineRe.ReplaceAllStringFunc(s, func(m string) string {
		return strings.Trim(m, "$")
	})
	// 3. Lowercase + smart-quote / unicode normalize. Keep this simple:
	//    the Indonesian content set rarely sees full-width digits; the
	//    main offenders are curly quotes from copy-paste.
	s = strings.ToLower(s)
	s = strings.NewReplacer(
		"\u201c", `"`, "\u201d", `"`,
		"\u2018", "'", "\u2019", "'",
		"\u2013", "-", "\u2014", "-",
		"\u00a0", " ",
	).Replace(s)
	// 4. Strip punctuation + symbols (keep digits + letters + space).
	s = punctRe.ReplaceAllString(s, " ")
	// 5. Collapse whitespace.
	s = strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
	// 6. Strip leading politeness prefixes iteratively (max 3 layers
	// so we don't loop on weird input). Handles chained patterns like
	// "sebutkan dengan jelas manakah ..." → strip "sebutkan" → strip
	// "dengan jelas" → strip "manakah" → semantic core.
	for pass := 0; pass < 3; pass++ {
		stripped := false
		for _, p := range politenessPrefixes {
			if strings.HasPrefix(s, p+" ") {
				s = strings.TrimSpace(s[len(p):])
				stripped = true
				break
			}
			if s == p {
				s = ""
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}
	return s
}

// findSimilarQuestion returns the (id, content, similarity) of the
// most-similar existing question in the exam, or zero values if no
// match crosses the threshold. Uses pg_trgm similarity() which is
// O(log N) thanks to the GIN index.
//
// threshold range: 0.0 (everything matches) → 1.0 (exact). Recommended:
//   - 0.95+ : near-certain duplicate, hard block
//   - 0.85+ : probable duplicate, return as warning
//   - 0.70+ : possible paraphrase, surface for AI review
func findSimilarQuestion(
	ctx context.Context, db dbExecer, examID, normalized string, threshold float64,
) (id, content string, similarity float64, found bool) {
	if normalized == "" || examID == "" {
		return "", "", 0, false
	}
	row := db.QueryRowContext(ctx, `
		SELECT id::text, content, similarity(content_normalized, $1) AS sim
		  FROM exam_questions
		 WHERE exam_id = $2
		   AND content_normalized % $1
		   AND similarity(content_normalized, $1) >= $3
		 ORDER BY sim DESC
		 LIMIT 1`,
		normalized, examID, threshold,
	)
	var sim float64
	err := row.Scan(&id, &content, &sim)
	if err != nil {
		return "", "", 0, false
	}
	return id, content, sim, true
}

// findSimilarQuestionsTopK returns up to k questions ordered by
// similarity descending. Used by the AI tool find_similar_questions
// so the model can scan top candidates before proposing.
type SimilarQuestionHit struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	Similarity float64 `json:"similarity"`
	SortOrder  int     `json:"sortOrder"`
}

func findSimilarQuestionsTopK(
	ctx context.Context, db dbExecer, examID, normalized string, threshold float64, k int,
) ([]SimilarQuestionHit, error) {
	if normalized == "" || examID == "" {
		return nil, nil
	}
	if k <= 0 || k > 20 {
		k = 5
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id::text, content, similarity(content_normalized, $1) AS sim, sort_order
		  FROM exam_questions
		 WHERE exam_id = $2
		   AND content_normalized % $1
		   AND similarity(content_normalized, $1) >= $3
		 ORDER BY sim DESC
		 LIMIT $4`,
		normalized, examID, threshold, k,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SimilarQuestionHit
	for rows.Next() {
		var hit SimilarQuestionHit
		if err := rows.Scan(&hit.ID, &hit.Content, &hit.Similarity, &hit.SortOrder); err != nil {
			continue
		}
		out = append(out, hit)
	}
	return out, nil
}
