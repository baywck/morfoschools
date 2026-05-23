package app

// Compound tools — atomic multi-step constructors that the model can
// call in ONE turn instead of chaining stimulus → group → questions
// across 3 propose-confirm cycles. The reasoning model conservatively
// proposes the first step then waits for execution before knowing the
// new resource ID; this means a 3-step plan ("buat stimulus + group
// + soal") becomes 3 user "ya" confirmations and 3 LLM rounds even
// though the user wants one transaction.
//
// create_stimulus_block solves the dominant exam-authoring use case:
// a teacher wants a passage + a group + N soal that all reference
// the same stimulus. Wrapped in one DB transaction so partial
// failure rolls back cleanly.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// RegisterCompoundCapabilities wires multi-step transactional tools.
// Called by RegisterAllCapabilities after all the individual-step
// capabilities are registered.
func (a *App) RegisterCompoundCapabilities(reg *CapabilityRegistry) {
	reg.Register(Capability{
		Name:        "create_stimulus_block",
		Description: "Atomic: create stimulus + group + N questions in ONE transaction. Use when user asks for a passage with multiple linked soal. All questions auto-link to the new group + stimulus. Tip: pakai ini daripada create_stimulus + create_question_group + create_question terpisah karena step-step itu butuh ID dari step sebelumnya.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string","description":"Exam tujuan"},
			"sectionId":{"type":"string","description":"Section tujuan; default = first section of exam"},
			"stimulus":{"type":"object","properties":{
				"title":{"type":"string"},
				"content":{"type":"string"},
				"source":{"type":"string"},
				"lifecycle":{"type":"string","enum":["exam_scoped","shared"],"default":"exam_scoped"}
			},"required":["title","content"]},
			"groupTitle":{"type":"string","description":"Optional override; default = stimulus title"},
			"questions":{"type":"array","items":{"type":"object","properties":{
				"questionType":{"type":"string","enum":["multiple_choice","true_false","short_answer","essay"]},
				"content":{"type":"string"},
				"explanation":{"type":"string"},
				"correctAnswer":{"type":"string"},
				"points":{"type":"number","default":1},
				"options":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"isCorrect":{"type":"boolean"}},"required":["content","isCorrect"]}}
			},"required":["questionType","content"]}}
		},"required":["examId","stimulus","questions"]}`),
	}, a.capCreateStimulusBlock)
}

func (a *App) capCreateStimulusBlock(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	var p struct {
		ExamID    string         `json:"examId"`
		Stimulus  map[string]any `json:"stimulus"`
		Questions []struct {
			QuestionType string   `json:"questionType"`
			Content      string   `json:"content"`
			Points       *float64 `json:"points"`
			Options      []struct {
				Content   string `json:"content"`
				IsCorrect bool   `json:"isCorrect"`
			} `json:"options"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(args, &p)

	// Defensive validation: reject empty soal entries before they
	// reach the executor (where empty question_type violates the DB
	// CHECK constraint exam_questions_question_type_check). The model
	// occasionally emits skeleton {questionType:'', content:''} entries
	// when the user only asked for a stimulus — we should not turn
	// those into a no-op DB write that fails halfway.
	stimTitleRaw, _ := p.Stimulus["title"].(string)
	stimContentRaw, _ := p.Stimulus["content"].(string)
	if strings.TrimSpace(stimContentRaw) == "" {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":        "INVALID_STIMULUS",
				"message":     "create_stimulus_block butuh stimulus.content yang non-kosong.",
				"recoverable": true,
				"recovery": map[string]any{
					"hint": "Generate stimulus body sebagai passage/teks/kasus yang lengkap (3+ paragraf), bukan placeholder kosong.",
				},
			},
		})
		return string(b), nil
	}
	// Filter out skeleton/empty soal entries. If user wants stimulus
	// only (questions=[] or all empty), route to update_question_group
	// path instead by suggesting it.
	validQs := p.Questions[:0]
	for _, q := range p.Questions {
		if strings.TrimSpace(q.Content) == "" || strings.TrimSpace(q.QuestionType) == "" {
			continue
		}
		validQs = append(validQs, q)
	}
	p.Questions = validQs
	if len(p.Questions) == 0 {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":        "NO_VALID_QUESTIONS",
				"message":     "create_stimulus_block butuh minimal 1 soal dengan questionType + content non-kosong.",
				"recoverable": true,
				"recovery": map[string]any{
					"hint": "Kalau user hanya minta stimulus tanpa soal, JANGAN pakai create_stimulus_block. Gunakan create_question_group + update_question_group dengan titleSnapshot/bodySnapshot, atau create_stimulus stand-alone. create_stimulus_block khusus untuk paket stimulus + N soal sekaligus.",
				},
			},
		})
		return string(b), nil
	}

	// Re-marshal args with the filtered Questions so the executor
	// receives the cleaned payload (otherwise raw args still has the
	// skeleton entries and explodes downstream).
	cleaned := map[string]any{
		"examId":    p.ExamID,
		"stimulus":  p.Stimulus,
		"questions": p.Questions,
	}
	// Preserve any extra top-level keys (sectionId, groupTitle, etc.)
	// that the original payload carried.
	var raw map[string]any
	_ = json.Unmarshal(args, &raw)
	for k, v := range raw {
		if _, ok := cleaned[k]; !ok {
			cleaned[k] = v
		}
	}
	cleanedArgs, _ := json.Marshal(cleaned)

	// Build a rich confirmation that lets the user review the entire
	// payload before committing. User-trust > brevity here — they're
	// about to write 5+ database rows in a single 'ya'.
	var sb strings.Builder
	stimTitle := stimTitleRaw
	stimContent := stimContentRaw
	if stimTitle == "" {
		// Mirror executor auto-derive so the preview matches what will
		// actually be inserted.
		stimTitle = strings.SplitN(strings.TrimSpace(stimContent), "\n", 2)[0]
		if len(stimTitle) > 80 {
			stimTitle = stimTitle[:80] + "…"
		}
	}
	sb.WriteString("**Stimulus + group + ")
	sb.WriteString(fmt.Sprintf("%d soal dalam satu transaksi**\n\n", len(p.Questions)))
	sb.WriteString("**📄 Stimulus:** ")
	sb.WriteString(stimTitle)
	sb.WriteString("\n")
	if stimContent != "" {
		preview := stimContent
		if len(preview) > 280 {
			preview = preview[:280] + "…"
		}
		sb.WriteString("> ")
		sb.WriteString(strings.ReplaceAll(strings.TrimSpace(preview), "\n", "\n> "))
		sb.WriteString("\n\n")
	}
	sb.WriteString("**❓ Soal:**\n")
	for i, q := range p.Questions {
		pts := 1.0
		if q.Points != nil {
			pts = *q.Points
		}
		content := q.Content
		if len(content) > 140 {
			content = content[:140] + "…"
		}
		sb.WriteString(fmt.Sprintf("%d. **[%s, %.0fpt]** %s\n", i+1, q.QuestionType, pts, content))
		// For multiple_choice show options + mark correct
		if q.QuestionType == "multiple_choice" && len(q.Options) > 0 {
			for j, opt := range q.Options {
				letter := string(rune('A' + j))
				mark := ""
				if opt.IsCorrect {
					mark = " ✅"
				}
				oc := opt.Content
				if len(oc) > 90 {
					oc = oc[:90] + "…"
				}
				sb.WriteString(fmt.Sprintf("   %s. %s%s\n", letter, oc, mark))
			}
		}
	}
	confirm := sb.String()
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_stimulus_block", cleanedArgs, confirm)
}

func (a *App) execCreateStimulusBlock(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID    string `json:"examId"`
		SectionID string `json:"sectionId"`
		Stimulus  struct {
			Title     string `json:"title"`
			Content   string `json:"content"`
			Source    string `json:"source"`
			Lifecycle string `json:"lifecycle"`
		} `json:"stimulus"`
		GroupTitle string            `json:"groupTitle"`
		Questions  []json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ExamID); denied != "" {
		return denied, nil
	}
	if strings.TrimSpace(p.Stimulus.Title) == "" {
		// Auto-derive title from first line of content when model omits it
		// (Gemini frequently does this on compound calls). Caps at 80 chars.
		title := strings.SplitN(strings.TrimSpace(p.Stimulus.Content), "\n", 2)[0]
		if len(title) > 80 {
			title = title[:80] + "…"
		}
		if title == "" {
			return errValidationFailed("stimulus.title", "stimulus.title atau content wajib"), nil
		}
		p.Stimulus.Title = title
	}
	if strings.TrimSpace(p.Stimulus.Content) == "" {
		return errValidationFailed("stimulus.content", "stimulus.content wajib"), nil
	}
	if len(p.Questions) == 0 {
		return errValidationFailed("questions", "questions tidak boleh kosong"), nil
	}
	if p.Stimulus.Lifecycle == "" {
		p.Stimulus.Lifecycle = "exam_scoped"
	}

	// Resolve sectionId: explicit > first section of exam.
	if p.SectionID == "" {
		_ = a.db.QueryRowContext(ctx,
			`SELECT id::text FROM exam_sections
			  WHERE exam_id = $1 AND tenant_id = $2
			  ORDER BY sort_order ASC, created_at ASC LIMIT 1`,
			p.ExamID, tenantID,
		).Scan(&p.SectionID)
	}
	if p.SectionID == "" {
		return errValidationFailed("sectionId", "exam belum punya section. Buat section dulu."), nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// 1. Insert stimulus.
	var stimulusID string
	parentExam := sql.NullString{}
	if p.Stimulus.Lifecycle == "exam_scoped" {
		parentExam = sql.NullString{String: p.ExamID, Valid: true}
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO stimuli (tenant_id, owner_user_id, type, title, content, source, lifecycle, parent_exam_id)
		VALUES ($1, $2, 'text', $3, $4, NULLIF($5,''), $6, $7)
		RETURNING id::text`,
		tenantID, userID, p.Stimulus.Title, p.Stimulus.Content, p.Stimulus.Source,
		p.Stimulus.Lifecycle, parentExam,
	).Scan(&stimulusID); err != nil {
		return "", err
	}

	// 2. Insert group with stimulus snapshot.
	groupTitle := p.GroupTitle
	if groupTitle == "" {
		groupTitle = p.Stimulus.Title
	}
	var displayOrder int
	// Section-unified: account for both existing groups and standalone
	// questions in this section so the new group appends after the
	// last visible block on the canvas, not at a position that may
	// collide with a standalone q's sort_order.
	displayOrder = nextSectionPosition(ctx, tx, p.SectionID)
	var groupID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exam_question_groups (
		    tenant_id, exam_id, section_id, stimulus_id,
		    stimulus_title_snapshot, stimulus_body_snapshot,
		    group_type, display_order
		) VALUES ($1, $2, $3, $4, $5, $6, 'stimulus', $7)
		RETURNING id::text`,
		tenantID, p.ExamID, p.SectionID, stimulusID,
		groupTitle, p.Stimulus.Content, displayOrder,
	).Scan(&groupID); err != nil {
		return "", err
	}

	// 3. Insert questions, all linked to group + stimulus.
	var usesKisiKisi bool
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(uses_kisi_kisi,false) FROM exams WHERE id=$1 AND tenant_id=$2`, p.ExamID, tenantID).Scan(&usesKisiKisi)
	bpID := ""
	bpPos := 0
	if usesKisiKisi {
		bpID, _ = ensureExamBlueprintTx(ctx, tx, tenantID, p.ExamID, "merdeka")
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM exam_blueprint_slots WHERE exam_blueprint_id = $1`, bpID).Scan(&bpPos)
	}

	createdQuestionIDs := make([]string, 0, len(p.Questions))
	linkedKisi := 0
	for _, qRaw := range p.Questions {
		var qm map[string]any
		_ = json.Unmarshal(qRaw, &qm)
		qm["examId"] = p.ExamID
		qm["sectionId"] = p.SectionID
		qm["groupId"] = groupID
		// DB constraint: questions XOR (group_id, stimulus_id). When a
		// question lives in a group, the group already carries the
		// stimulus link — setting both violates exam_questions_stimulus
		// _xor_group_chk. Strip stimulusId here so the model can pass
		// it for clarity but we won't write the duplicate.
		delete(qm, "stimulusId")
		merged, _ := json.Marshal(qm)

		id, err := a.insertQuestionWithOptionsTx(ctx, tx, tenantID, userID, merged)
		if err != nil {
			return "", err
		}
		createdQuestionIDs = append(createdQuestionIDs, id)
		if usesKisiKisi && bpID != "" {
			if _, hasSlot := qm["blueprintSlotId"]; !hasSlot {
				item := kisiItemFromQuestionMap(id, qm, bpPos)
				var slotID string
				if err := tx.QueryRowContext(ctx, `
					INSERT INTO exam_blueprint_slots (
					    exam_blueprint_id, position,
					    competency_code, competency_description, materi, indikator,
					    cognitive_level, difficulty, question_type, points
					) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10)
					RETURNING id::text`,
					bpID, bpPos, item.CompetencyCode, item.CompetencyDescription, item.Materi, item.Indikator,
					item.CognitiveLevel, item.Difficulty, item.QuestionType, item.points,
				).Scan(&slotID); err != nil {
					return "", err
				}
				if _, err := tx.ExecContext(ctx, `UPDATE exam_questions SET blueprint_slot_id=$1, updated_at=now() WHERE id=$2 AND tenant_id=$3`, slotID, id, tenantID); err != nil {
					return "", err
				}
				bpPos++
				linkedKisi++
			}
		}
	}
	if usesKisiKisi && bpID != "" && linkedKisi > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE exam_blueprints SET
			    total_slots = (SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
			    total_points = (SELECT COALESCE(SUM(points),0) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
			    updated_at = now()
			WHERE id=$1`, bpID); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	a.auditAI(ctx, tenantID, userID, "stimuli.create", "stimulus", stimulusID)
	a.auditAI(ctx, tenantID, userID, "groups.create", "exam_question_group", groupID)
	for _, qid := range createdQuestionIDs {
		a.auditAI(ctx, tenantID, userID, "questions.create", "exam_question", qid)
	}

	b, _ := json.Marshal(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Stimulus + group + %d soal berhasil dibuat dalam satu transaksi%s", len(createdQuestionIDs), func() string {
			if linkedKisi > 0 {
				return fmt.Sprintf("; %d kisi-kisi otomatis dibuat", linkedKisi)
			}
			return ""
		}()),
		"stimulusId":     stimulusID,
		"groupId":        groupID,
		"questionIds":    createdQuestionIDs,
		"questionCount":  len(createdQuestionIDs),
		"linkedKisiKisi": linkedKisi,
	})
	return string(b), nil
}
