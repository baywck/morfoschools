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
		Name: "create_stimulus_block",
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
	// Build a confirmation summary that previews the whole block.
	var p struct {
		ExamID    string                   `json:"examId"`
		Stimulus  map[string]any           `json:"stimulus"`
		Questions []map[string]interface{} `json:"questions"`
	}
	_ = json.Unmarshal(args, &p)
	stimTitle, _ := p.Stimulus["title"].(string)
	confirm := fmt.Sprintf("Buat stimulus %q + group + %d soal dalam satu transaksi.", stimTitle, len(p.Questions))
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_stimulus_block", args, confirm)
}

func (a *App) execCreateStimulusBlock(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID     string `json:"examId"`
		SectionID  string `json:"sectionId"`
		Stimulus   struct {
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
	_ = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(display_order), -1) + 1 FROM exam_question_groups WHERE section_id = $1`,
		p.SectionID,
	).Scan(&displayOrder)
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
	createdQuestionIDs := make([]string, 0, len(p.Questions))
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
		"success":      true,
		"message":      fmt.Sprintf("Stimulus + group + %d soal berhasil dibuat dalam satu transaksi", len(createdQuestionIDs)),
		"stimulusId":   stimulusID,
		"groupId":      groupID,
		"questionIds":  createdQuestionIDs,
		"questionCount": len(createdQuestionIDs),
	})
	return string(b), nil
}
