package app

// Active-page context for the AI chat (Phase 9.11).
//
// When the user opens a resource page (e.g. /app/exams/{id}) the
// frontend ships the resolved entity IDs in shadow.activeEntities.
// buildActiveContext loads a compact, read-only summary of that
// resource and returns it as a markdown-ish block ready to embed in
// the system prompt.
//
// Goals:
//   - Ground the model in real state so suggestions don't duplicate
//     existing items (no "buat 10 soal" that re-creates rows).
//   - Carry enough kisi-kisi metadata that the model can write soal
//     yang sesuai dengan KD/Materi/Indikator.
//   - Stay short — the system prompt budget is limited.
//
// Tenant + access enforcement: every query filters by tenant_id and
// the resolveExamAccess / resolveBlueprintAccess helpers gate the
// caller's view. If the caller can't read the resource, we skip
// silently (no context, no leak).

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const activeCtxQueryTimeout = 2 * time.Second

// buildActiveContext renders the system-prompt block for the
// resource(s) referenced in active. Empty string when nothing
// applies — the caller drops the wrapping section in that case.
func (a *App) buildActiveContext(tenantID string, auth *AuthContext, active map[string]string) string {
	if len(active) == 0 || tenantID == "" || auth == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), activeCtxQueryTimeout)
	defer cancel()

	var sb strings.Builder

	if examID := active["examId"]; examID != "" {
		if a.userCanReadExam(ctx, tenantID, auth, examID) {
			a.appendExamContext(ctx, &sb, tenantID, examID)
		}
	}
	if templateID := active["templateId"]; templateID != "" {
		if a.userCanReadBlueprint(ctx, tenantID, auth, templateID) {
			a.appendBlueprintContext(ctx, &sb, tenantID, templateID)
		}
	}
	return sb.String()
}

func (a *App) userCanReadExam(ctx context.Context, tenantID string, auth *AuthContext, examID string) bool {
	access, err := a.resolveExamAccess(ctx, tenantID, auth, examID)
	if err != nil {
		return false
	}
	return access.CanRead
}

func (a *App) userCanReadBlueprint(ctx context.Context, tenantID string, auth *AuthContext, templateID string) bool {
	access, err := a.resolveBlueprintAccess(ctx, tenantID, auth, templateID)
	if err != nil {
		return false
	}
	return access.CanRead
}

// appendExamContext writes a compact summary of the exam into sb:
//   - title, type, status, points
//   - section list with question counts
//   - blueprint type + slot count + total points
//   - per-slot kisi-kisi line (KD / Materi / Indikator / Cog / Diff)
//   - already-authored question stems (truncated) so the model
//     doesn't propose duplicates
func (a *App) appendExamContext(
	ctx context.Context, sb *strings.Builder, tenantID, examID string,
) {
	var (
		title, examType, status string
		maxScore, passing       float64
		usesKisi                bool
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT title, exam_type, status,
		       COALESCE(max_score, 0), COALESCE(passing_score, 0),
		       uses_kisi_kisi
		  FROM exams
		 WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&title, &examType, &status, &maxScore, &passing, &usesKisi)
	if err != nil {
		return
	}

	fmt.Fprintf(sb, "Exam aktif: %q (id=%s)\n", title, examID)
	fmt.Fprintf(sb, "Tipe: %s | Status: %s | Max: %.0f | Lulus: %.0f | Kisi-kisi: %t\n",
		examType, status, maxScore, passing, usesKisi)

	// Section + question count
	srows, err := a.db.QueryContext(ctx, `
		SELECT s.id::text, s.title, s.sort_order,
		       COALESCE((SELECT COUNT(*) FROM exam_questions q
		                  WHERE q.section_id = s.id), 0)
		  FROM exam_sections s
		 WHERE s.exam_id = $1 AND s.tenant_id = $2
		 ORDER BY s.sort_order`,
		examID, tenantID,
	)
	if err == nil {
		defer srows.Close()
		var sections []string
		total := 0
		for srows.Next() {
			var sid, stitle string
			var sord, qc int
			if err := srows.Scan(&sid, &stitle, &sord, &qc); err == nil {
				sections = append(sections, fmt.Sprintf("  - %q (id=%s): %d soal", stitle, sid, qc))
				total += qc
			}
		}
		fmt.Fprintf(sb, "Section (%d total soal):\n", total)
		for _, l := range sections {
			sb.WriteString(l + "\n")
		}
	}

	// Blueprint + slot summary (only when uses_kisi_kisi=true and a
	// blueprint exists; otherwise skip to keep the prompt short).
	if usesKisi {
		var bpID, bpType string
		var totalSlots int
		var totalPoints float64
		_ = a.db.QueryRowContext(ctx, `
			SELECT id::text, blueprint_type, total_slots, total_points
			  FROM exam_blueprints
			 WHERE exam_id = $1 AND tenant_id = $2`,
			examID, tenantID,
		).Scan(&bpID, &bpType, &totalSlots, &totalPoints)
		if bpID != "" {
			fmt.Fprintf(sb, "Blueprint: %s | %d slot | %.0f pts\n",
				bpType, totalSlots, totalPoints)
			a.appendSlotLines(ctx, sb, bpID)
		}
	}

	// Existing question stems (truncated, with type + points). Lets
	// the model see what's already written so it doesn't propose
	// duplicates. Cap at 30 to keep budget sane on big exams.
	qrows, err := a.db.QueryContext(ctx, `
		SELECT q.id::text, q.sort_order, q.question_type, q.points,
		       LEFT(COALESCE(q.content,''), 80)
		  FROM exam_questions q
		 WHERE q.exam_id = $1 AND q.tenant_id = $2
		 ORDER BY q.sort_order
		 LIMIT 20`,
		examID, tenantID,
	)
	if err == nil {
		defer qrows.Close()
		var lines []string
		for qrows.Next() {
			var qid string
			var ord int
			var qt string
			var pts float64
			var content string
			if err := qrows.Scan(&qid, &ord, &qt, &pts, &content); err == nil {
				content = strings.TrimSpace(content)
				if content == "" {
					content = "(kosong)"
				}
				lines = append(lines, fmt.Sprintf("  #%d (id=%s) [%s, %.0fpt] %s",
					ord+1, qid, qt, pts, oneLine(content)))
			}
		}
		if len(lines) > 0 {
			sb.WriteString("Soal yang sudah ada (jangan duplikasi):\n")
			for _, l := range lines {
				sb.WriteString(l + "\n")
			}
		}
	}
}

// appendSlotLines lists every slot under blueprintID with its
// kisi-kisi metadata in a single line per slot. Skipped slots (already
// linked to a question) are flagged so the model knows what's free.
func (a *App) appendSlotLines(ctx context.Context, sb *strings.Builder, blueprintID string) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT s.position,
		       COALESCE(s.competency_code, ''),
		       COALESCE(s.materi, ''),
		       COALESCE(s.indikator, ''),
		       COALESCE(s.cognitive_level, ''),
		       COALESCE(s.difficulty, ''),
		       COALESCE(s.question_type, ''),
		       s.points,
		       (SELECT id::text FROM exam_questions q
		         WHERE q.blueprint_slot_id = s.id LIMIT 1) AS qid
		  FROM exam_blueprint_slots s
		 WHERE s.exam_blueprint_id = $1
		 ORDER BY s.position`,
		blueprintID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	sb.WriteString("Slot kisi-kisi:\n")
	for rows.Next() {
		var pos int
		var kd, mat, indi, cog, diff, qtype string
		var pts float64
		var qid *string
		if err := rows.Scan(&pos, &kd, &mat, &indi, &cog, &diff, &qtype, &pts, &qid); err != nil {
			continue
		}
		filled := "kosong"
		if qid != nil && *qid != "" {
			filled = "terisi"
		}
		fmt.Fprintf(sb,
			"  %d. KD=%s | Materi=%s | Indikator=%s | %s/%s/%s/%.0fpt [%s]\n",
			pos+1, dash(kd), dash(mat), oneLine(dash(indi)),
			dash(cog), dash(diff), dash(qtype), pts, filled,
		)
	}
}

// appendBlueprintContext writes the blueprint-template summary (when
// the user is on the blueprint detail page rather than an exam).
func (a *App) appendBlueprintContext(
	ctx context.Context, sb *strings.Builder, tenantID, templateID string,
) {
	var (
		title, bpType, status string
		totalSlots            int
		totalPoints           float64
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT title, blueprint_type, status, total_slots, total_points
		  FROM blueprint_templates
		 WHERE id = $1 AND tenant_id = $2`,
		templateID, tenantID,
	).Scan(&title, &bpType, &status, &totalSlots, &totalPoints)
	if err != nil {
		return
	}
	fmt.Fprintf(sb, "Blueprint template aktif: %q (id=%s)\n", title, templateID)
	fmt.Fprintf(sb, "Tipe: %s | Status: %s | %d slot | %.0f pts\n",
		bpType, status, totalSlots, totalPoints)

	// Slots inline (template_slots table, not exam_blueprint_slots).
	rows, err := a.db.QueryContext(ctx, `
		SELECT position,
		       COALESCE(competency_code, ''),
		       COALESCE(materi, ''),
		       COALESCE(indikator, ''),
		       COALESCE(cognitive_level, ''),
		       COALESCE(difficulty, ''),
		       COALESCE(question_type, ''),
		       points
		  FROM blueprint_template_slots
		 WHERE template_id = $1
		 ORDER BY position`,
		templateID,
	)
	if err == nil {
		defer rows.Close()
		sb.WriteString("Slot:\n")
		for rows.Next() {
			var pos int
			var kd, mat, indi, cog, diff, qtype string
			var pts float64
			if err := rows.Scan(&pos, &kd, &mat, &indi, &cog, &diff, &qtype, &pts); err != nil {
				continue
			}
			fmt.Fprintf(sb,
				"  %d. KD=%s | Materi=%s | Indikator=%s | %s/%s/%s/%.0fpt\n",
				pos+1, dash(kd), dash(mat), oneLine(dash(indi)),
				dash(cog), dash(diff), dash(qtype), pts,
			)
		}
	}
}

// dash replaces an empty string with "-" so the prompt stays readable.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// appendActiveDomains augments the keyword-derived domain list with
// domains implied by the active page. The page is a much stronger
// intent signal than message keywords — a teacher on /app/exams/{id}
// who types "buatkan 10 soal tentang himpunan" should see exam tools
// even if their message happens to lack a recognisable trigger word.
//
// Idempotent and safe to call with empty active map: returns the
// input unchanged. Token-economy version: includes blueprint+stimuli
// only when the message actually hints at those domains, since the
// majority of exam-authoring requests don't need them.
func appendActiveDomains(domains []string, active map[string]string) []string {
	return appendActiveDomainsForMessage(domains, active, "")
}

// appendActiveDomainsForMessage is the smart variant: filters which
// adjacent domains to include based on what the user message hints at.
// On /app/exams/{id} we expose:
//   - exams (always when examId active)
//   - blueprints only if message mentions kisi/blueprint/slot/AKM/etc.
//   - stimuli only if message mentions stimulus/passage/bacaan/teks
// Reduces tool count from ~48 to ~28 on the typical "buat soal" flow.
func appendActiveDomainsForMessage(domains []string, active map[string]string, msg string) []string {
	if len(active) == 0 {
		return domains
	}
	seen := make(map[string]bool, len(domains))
	for _, d := range domains {
		seen[d] = true
	}
	add := func(d string) {
		if !seen[d] {
			domains = append(domains, d)
			seen[d] = true
		}
	}
	lower := strings.ToLower(msg)
	hintsBlueprint := strings.Contains(lower, "kisi") || strings.Contains(lower, "blueprint") ||
		strings.Contains(lower, "slot") || strings.Contains(lower, "akm") ||
		strings.Contains(lower, "kompetensi") || strings.Contains(lower, "kompeten") ||
		strings.Contains(lower, "template") || strings.Contains(lower, "reverse") ||
		strings.Contains(lower, "analis")
	hintsStimulus := strings.Contains(lower, "stimulus") || strings.Contains(lower, "stimuli") ||
		strings.Contains(lower, "passage") || strings.Contains(lower, "bacaan") ||
		strings.Contains(lower, "teks") || strings.Contains(lower, "wacana") ||
		strings.Contains(lower, "kasus")

	if active["examId"] != "" {
		add("exams")
		if hintsBlueprint {
			add("blueprints")
		}
		if hintsStimulus {
			add("stimuli")
		}
	}
	if active["templateId"] != "" {
		add("blueprints")
		// Blueprint authoring may involve creating questions/stimuli
		// from slots; include those only when the message hints at it.
		if strings.Contains(lower, "soal") || strings.Contains(lower, "question") ||
			strings.Contains(lower, "isi") || strings.Contains(lower, "buat") ||
			strings.Contains(lower, "generate") {
			add("exams")
		}
		if hintsStimulus {
			add("stimuli")
		}
	}
	if active["courseId"] != "" {
		add("courses")
	}
	if active["stimulusId"] != "" {
		add("stimuli")
	}
	if active["programId"] != "" {
		add("programs")
	}
	return domains
}

// oneLine collapses internal newlines + carriage returns to spaces and
// trims runs of whitespace so a multi-line indikator fits on one
// system-prompt row.
func oneLine(s string) string {
	r := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ")
	out := r.Replace(s)
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return strings.TrimSpace(out)
}
