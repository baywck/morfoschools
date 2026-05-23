package app

// AI tools for exam-authoring actions that were missing from the
// initial Phase 9 capability set: update_question, delete_question,
// create_question_group, create_stimulus, create_exam_section,
// move_question. Without these the agent had to fall back to telling
// the user to "copy-paste manually" which broke the active-page
// context flow.
//
// Each tool follows the established pattern:
//   1. Capability declared via reg.Register with permission gate
//   2. cap* handler validates inputs + delegates to exec*
//   3. exec* runs through executeConfirmedAction so confirm flow holds
//   4. Permission re-check inside cap* via checkExamWriteAccess /
//      checkBlueprintWriteAccess where applicable
//
// Tools that propose mutating state still emit a structured
// ai_pending_actions row and require user confirmation before
// running. Read-only tools execute inline.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// RegisterExamExtraCapabilities wires the missing exam-authoring
// tools into the registry. Call site lives in ai_cap_registry.go's
// RegisterAllCapabilities.
func (a *App) RegisterExamExtraCapabilities(reg *CapabilityRegistry) {
	// --- Update question ---
	reg.Register(Capability{
		Name: "update_question",
		Description: "Edit content / explanation / options / metadata of an existing question. " +
			"WAJIB: panggil list_questions atau get_exam dulu untuk dapat questionId yang valid. " +
			"Hanya field yang di-set yang akan diubah; field kosong dilewati.",
		Permission: "exams:write",
		Risk:       "write",
		Domain:     "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"questionId":{"type":"string"},
			"content":{"type":"string"},
			"explanation":{"type":"string"},
			"correctAnswer":{"type":"string"},
			"points":{"type":"number"},
			"questionType":{"type":"string","enum":["multiple_choice","true_false","short_answer","essay"]},
			"options":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"isCorrect":{"type":"boolean"},"pointsWeight":{"type":"number"}},"required":["content","isCorrect"]}},
			"competencyCode":{"type":"string"},
			"materi":{"type":"string"},
			"indikator":{"type":"string"},
			"cognitiveLevel":{"type":"string","enum":["C1","C2","C3","C4","C5","C6"]},
			"difficulty":{"type":"string","enum":["mudah","sedang","sulit"]}
		},"required":["questionId"]}`),
	}, a.capUpdateQuestion)

	// --- Delete question ---
	reg.Register(Capability{
		Name: "delete_question",
		Description: "Hapus soal beserta options dan link ke slot kisi-kisi. " +
			"Operasi ini tidak bisa di-undo. Pastikan questionId benar (panggil list_questions dulu).",
		Permission: "exams:write",
		Risk:       "destructive",
		Domain:     "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"questionId":{"type":"string"}
		},"required":["questionId"]}`),
	}, a.capDeleteQuestion)

	// --- Create exam section ---
	reg.Register(Capability{
		Name:        "create_exam_section",
		Description: "Add new section to exam.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"title":{"type":"string"},
			"description":{"type":"string"},
			"sortOrder":{"type":"integer"}
		},"required":["examId","title"]}`),
	}, a.capCreateExamSection)

	// --- Create question group ---
	reg.Register(Capability{
		Name:        "create_question_group",
		Description: "Create question group in section. CRITICAL: kalau user mau passage + soal yang merujuk passage, JANGAN pakai tool ini sendiri — pakai 'create_stimulus_block' yang atomic (stimulus + group + soal sekaligus). Tool ini hanya untuk group kosong (akan diisi soal manual nanti).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"sectionId":{"type":"string"},
			"stimulusId":{"type":"string","description":"opsional, link ke stimulus library"},
			"groupType":{"type":"string","enum":["standalone","stimulus"],"default":"standalone"},
			"titleSnapshot":{"type":"string","description":"manual title kalau stimulusId kosong"},
			"bodySnapshot":{"type":"string","description":"manual body kalau stimulusId kosong"}
		},"required":["examId","sectionId"]}`),
	}, a.capCreateQuestionGroup)

	// --- Create stimulus ---
	reg.Register(Capability{
		Name: "create_stimulus",
		Description: "Buat stimulus baru di library (passage, kasus, dialog, dst). " +
			"Lifecycle 'shared' = bisa dipakai banyak exam. 'exam_scoped' = lokal ke satu exam (perlu parentExamId).",
		Permission: "exams:write",
		Risk:       "write",
		Domain:     "stimuli",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"title":{"type":"string"},
			"content":{"type":"string"},
			"source":{"type":"string"},
			"lifecycle":{"type":"string","enum":["shared","exam_scoped"],"default":"shared"},
			"parentExamId":{"type":"string"}
		},"required":["title","content"]}`),
	}, a.capCreateStimulus)

	// --- Move question ---
	reg.Register(Capability{
		Name:        "move_question",
		Description: "Move question to different section/group/order in same exam. Set hanya field yang ingin diubah.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"questionId":{"type":"string"},
			"sectionId":{"type":"string"},
			"groupId":{"type":"string"},
			"sortOrder":{"type":"integer"}
		},"required":["questionId"]}`),
	}, a.capMoveQuestion)
}

// ============================================================================
// Capability handlers — all proxy to executeConfirmedAction via the proposal
// flow (so the user gets a confirm dialog before mutation).
// ============================================================================

func (a *App) capUpdateQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)

	// Validate: at least one editable field must be present. Without
	// this the model can fire an empty update_question with only
	// questionId and we surface a no-op proposal that does nothing
	// when confirmed (and creates user confusion / loop).
	var p struct {
		QuestionID     string           `json:"questionId"`
		Content        *string          `json:"content"`
		Explanation    *string          `json:"explanation"`
		CorrectAnswer  *string          `json:"correctAnswer"`
		Points         *float64         `json:"points"`
		QuestionType   *string          `json:"questionType"`
		CompetencyCode *string          `json:"competencyCode"`
		Materi         *string          `json:"materi"`
		Indikator      *string          `json:"indikator"`
		CognitiveLevel *string          `json:"cognitiveLevel"`
		Difficulty     *string          `json:"difficulty"`
		Options        []questionOption `json:"options"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.QuestionID) {
		return errInvalidUUID("questionId", p.QuestionID, "question"), nil
	}
	hasField := p.Content != nil || p.Explanation != nil || p.CorrectAnswer != nil ||
		p.Points != nil || p.QuestionType != nil || p.CompetencyCode != nil ||
		p.Materi != nil || p.Indikator != nil || p.CognitiveLevel != nil ||
		p.Difficulty != nil || p.Options != nil
	if !hasField {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":        "INVALID_UPDATE",
				"message":     "update_question butuh minimal satu field yang akan diubah (content/explanation/correctAnswer/points/questionType/options/kisi-kisi).",
				"recoverable": true,
				"recovery": map[string]any{
					"hint": "Isi field yang ingin diubah dengan nilai baru. Untuk soal HOTS, kirim 'content' baru. Untuk ganti tipe, kirim 'questionType' + 'options' jika perlu.",
				},
			},
		})
		return string(b), nil
	}

	// Build a content-aware confirmation so the user sees what's about
	// to change BEFORE clicking 'ya'. Mirrors the rich preview pattern
	// from create_question / create_stimulus_block.
	var sb strings.Builder
	sb.WriteString("**Update soal**\n")
	if p.Content != nil {
		excerpt := *p.Content
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		sb.WriteString("\n**Konten baru:**\n> " + excerpt + "\n")
	}
	if p.QuestionType != nil {
		fmt.Fprintf(&sb, "\n**Tipe baru:** %s\n", *p.QuestionType)
	}
	if p.Points != nil {
		fmt.Fprintf(&sb, "\n**Poin baru:** %.0f\n", *p.Points)
	}
	if p.CorrectAnswer != nil {
		fmt.Fprintf(&sb, "\n**Jawaban baru:** %s\n", *p.CorrectAnswer)
	}
	if p.Explanation != nil {
		ex := *p.Explanation
		if len(ex) > 160 {
			ex = ex[:160] + "…"
		}
		sb.WriteString("\n**Penjelasan baru:** " + ex + "\n")
	}
	if p.Options != nil {
		fmt.Fprintf(&sb, "\n**Opsi baru (%d):**\n", len(p.Options))
		for i, o := range p.Options {
			letter := string(rune('A' + i))
			mark := ""
			if o.IsCorrect {
				mark = " ✅"
			}
			oc := o.Content
			if len(oc) > 80 {
				oc = oc[:80] + "…"
			}
			fmt.Fprintf(&sb, "  %s) %s%s\n", letter, oc, mark)
		}
	}
	if p.CompetencyCode != nil || p.Materi != nil || p.Indikator != nil ||
		p.CognitiveLevel != nil || p.Difficulty != nil {
		sb.WriteString("\n**Update kisi-kisi:**\n")
		if p.CompetencyCode != nil {
			fmt.Fprintf(&sb, "  KD: %s\n", *p.CompetencyCode)
		}
		if p.Materi != nil {
			fmt.Fprintf(&sb, "  Materi: %s\n", *p.Materi)
		}
		if p.Indikator != nil {
			fmt.Fprintf(&sb, "  Indikator: %s\n", *p.Indikator)
		}
		if p.CognitiveLevel != nil {
			fmt.Fprintf(&sb, "  Cognitive: %s\n", *p.CognitiveLevel)
		}
		if p.Difficulty != nil {
			fmt.Fprintf(&sb, "  Difficulty: %s\n", *p.Difficulty)
		}
	}
	return a.createProposal(ctx, sessionID, tenantID, userID, "update_question", args, sb.String())
}

func (a *App) capDeleteQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "delete_question", args, confirmDeleteQuestion(args))
}

func (a *App) capCreateExamSection(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_exam_section", args, confirmCreateExamSection(args))
}

func (a *App) capCreateQuestionGroup(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_question_group", args, confirmCreateQuestionGroup(args))
}

func (a *App) capCreateStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_stimulus", args, confirmCreateStimulus(args))
}

func (a *App) capMoveQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "move_question", args, confirmMoveQuestion(args))
}

// ============================================================================
// Executors — called from executeConfirmedAction after user confirms.
// ============================================================================

func (a *App) execUpdateQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		QuestionID     string           `json:"questionId"`
		Content        *string          `json:"content"`
		Explanation    *string          `json:"explanation"`
		CorrectAnswer  *string          `json:"correctAnswer"`
		Points         *float64         `json:"points"`
		QuestionType   *string          `json:"questionType"`
		CompetencyCode *string          `json:"competencyCode"`
		Materi         *string          `json:"materi"`
		Indikator      *string          `json:"indikator"`
		CognitiveLevel *string          `json:"cognitiveLevel"`
		Difficulty     *string          `json:"difficulty"`
		Options        []questionOption `json:"options"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.QuestionID) {
		return errInvalidUUID("questionId", p.QuestionID, "question"), nil
	}

	// Resolve parent exam to gate write access (questions inherit from exam).
	var examID string
	if err := a.db.QueryRowContext(ctx,
		`SELECT exam_id::text FROM exam_questions WHERE id = $1 AND tenant_id = $2`,
		p.QuestionID, tenantID,
	).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("question", "questionId", p.QuestionID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Build dynamic UPDATE so we only touch fields the model supplied.
	parts := []string{}
	vals := []any{}
	idx := 1
	add := func(col string, v any) {
		parts = append(parts, fmt.Sprintf("%s = $%d", col, idx))
		vals = append(vals, v)
		idx++
	}
	if p.Content != nil {
		add("content", *p.Content)
		add("content_hash", hashContent(*p.Content))
		add("content_normalized", normalizeQuestionContent(*p.Content))
	}
	if p.Explanation != nil {
		add("explanation", *p.Explanation)
	}
	if p.CorrectAnswer != nil {
		add("correct_answer", *p.CorrectAnswer)
	}
	if p.Points != nil {
		add("points", *p.Points)
	}
	if p.QuestionType != nil {
		add("question_type", *p.QuestionType)
	}
	if len(parts) > 0 {
		parts = append(parts, "updated_at = now()")
		q := fmt.Sprintf("UPDATE exam_questions SET %s WHERE id = $%d AND tenant_id = $%d",
			strings.Join(parts, ", "), idx, idx+1)
		vals = append(vals, p.QuestionID, tenantID)
		if _, err := tx.ExecContext(ctx, q, vals...); err != nil {
			return "", err
		}
	}

	// Replace options when supplied. Replace = delete-then-insert; the
	// model is expected to send the full new option set, not a partial.
	if p.Options != nil {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM exam_question_options WHERE question_id = $1`, p.QuestionID,
		); err != nil {
			return "", err
		}
		for i, o := range p.Options {
			weight := 0.0
			if o.PointsWeight != nil {
				weight = *o.PointsWeight
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO exam_question_options
				    (tenant_id, question_id, content, is_correct, sort_order, points_weight)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				tenantID, p.QuestionID, o.Content, o.IsCorrect, i, weight,
			); err != nil {
				return "", err
			}
		}
	}

	// Slot writeback (kisi-kisi metadata) — same logic as the inline
	// PATCH route in questions.go. Only fires when the question is
	// already bound to a slot.
	if p.CompetencyCode != nil || p.Materi != nil || p.Indikator != nil ||
		p.CognitiveLevel != nil || p.Difficulty != nil {
		var slotID sql.NullString
		_ = tx.QueryRowContext(ctx,
			`SELECT blueprint_slot_id::text FROM exam_questions WHERE id = $1`,
			p.QuestionID,
		).Scan(&slotID)
		if slotID.Valid && slotID.String != "" {
			sp := slotPayload{
				CompetencyCode: p.CompetencyCode,
				Materi:         p.Materi,
				Indikator:      p.Indikator,
				CognitiveLevel: p.CognitiveLevel,
				Difficulty:     p.Difficulty,
			}
			if slotPayloadHasMeta(sp) {
				sq, sargs := buildSlotUpdateSQL("exam_blueprint_slots", slotID.String, sp)
				if sq != "" {
					if _, err := tx.ExecContext(ctx, sq, sargs...); err != nil {
						return "", err
					}
				}
			}
		}
	}

	markExamAIContextStale(ctx, tx, tenantID, examID)
	if err := tx.Commit(); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "questions.update", "exam_question", p.QuestionID)
	return fmt.Sprintf(`{"success":true,"message":"Soal berhasil diupdate","questionId":%q}`, p.QuestionID), nil
}

func (a *App) execDeleteQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		QuestionID string `json:"questionId"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.QuestionID) {
		return errInvalidUUID("questionId", p.QuestionID, "question"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx,
		`SELECT exam_id::text FROM exam_questions WHERE id = $1 AND tenant_id = $2`,
		p.QuestionID, tenantID,
	).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("question", "questionId", p.QuestionID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	if _, err := a.db.ExecContext(ctx,
		`DELETE FROM exam_questions WHERE id = $1 AND tenant_id = $2`,
		p.QuestionID, tenantID,
	); err != nil {
		return "", err
	}
	markExamAIContextStaleDB(ctx, a.db, tenantID, examID)
	a.auditAI(ctx, tenantID, userID, "questions.delete", "exam_question", p.QuestionID)
	return fmt.Sprintf(`{"success":true,"message":"Soal dihapus","questionId":%q}`, p.QuestionID), nil
}

func (a *App) execCreateExamSection(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID      string `json:"examId"`
		Title       string `json:"title"`
		Description string `json:"description"`
		SortOrder   *int   `json:"sortOrder"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	if strings.TrimSpace(p.Title) == "" {
		return errValidationFailed("title", "title is required"), nil
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ExamID); denied != "" {
		return denied, nil
	}
	sortOrder := 0
	if p.SortOrder != nil {
		sortOrder = *p.SortOrder
	} else {
		_ = a.db.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM exam_sections WHERE exam_id = $1`, p.ExamID,
		).Scan(&sortOrder)
	}
	var id string
	if err := a.db.QueryRowContext(ctx, `
		INSERT INTO exam_sections (tenant_id, exam_id, title, description, sort_order)
		VALUES ($1, $2, $3, NULLIF($4,''), $5)
		RETURNING id`,
		tenantID, p.ExamID, strings.TrimSpace(p.Title), strings.TrimSpace(p.Description), sortOrder,
	).Scan(&id); err != nil {
		return "", err
	}
	markExamAIContextStaleDB(ctx, a.db, tenantID, p.ExamID)
	a.auditAI(ctx, tenantID, userID, "exam_sections.create", "exam_section", id)
	return fmt.Sprintf(`{"success":true,"id":%q,"sortOrder":%d}`, id, sortOrder), nil
}

func (a *App) execCreateQuestionGroup(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID        string `json:"examId"`
		SectionID     string `json:"sectionId"`
		StimulusID    string `json:"stimulusId"`
		GroupType     string `json:"groupType"`
		TitleSnapshot string `json:"titleSnapshot"`
		BodySnapshot  string `json:"bodySnapshot"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	if !isUUID(p.SectionID) {
		return errInvalidUUID("sectionId", p.SectionID, "section"), nil
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ExamID); denied != "" {
		return denied, nil
	}
	if p.GroupType == "" {
		p.GroupType = "standalone"
	}
	if p.GroupType != "standalone" && p.GroupType != "stimulus" {
		return errValidationFailed("groupType", "groupType must be 'standalone' or 'stimulus'"), nil
	}
	// If stimulusId provided, snapshot from library.
	if p.StimulusID != "" {
		if !isUUID(p.StimulusID) {
			return errInvalidUUID("stimulusId", p.StimulusID, "stimulus"), nil
		}
		if err := a.db.QueryRowContext(ctx,
			`SELECT title, content FROM stimuli WHERE id = $1 AND tenant_id = $2`,
			p.StimulusID, tenantID,
		).Scan(&p.TitleSnapshot, &p.BodySnapshot); err != nil {
			if err == sql.ErrNoRows {
				return errEntityNotFound("stimulus", "stimulusId", p.StimulusID), nil
			}
			return "", err
		}
		p.GroupType = "stimulus"
	}
	var displayOrder int
	_ = a.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(display_order), -1) + 1 FROM exam_question_groups WHERE section_id = $1`,
		p.SectionID,
	).Scan(&displayOrder)
	var id string
	if err := a.db.QueryRowContext(ctx, `
		INSERT INTO exam_question_groups (
		    tenant_id, exam_id, section_id, stimulus_id,
		    stimulus_title_snapshot, stimulus_body_snapshot,
		    group_type, display_order
		) VALUES ($1, $2, $3, NULLIF($4,'')::uuid,
		          NULLIF($5,''), NULLIF($6,''),
		          $7, $8)
		RETURNING id`,
		tenantID, p.ExamID, p.SectionID, p.StimulusID,
		p.TitleSnapshot, p.BodySnapshot,
		p.GroupType, displayOrder,
	).Scan(&id); err != nil {
		return "", err
	}
	markExamAIContextStaleDB(ctx, a.db, tenantID, p.ExamID)
	a.auditAI(ctx, tenantID, userID, "groups.create", "exam_question_group", id)
	return fmt.Sprintf(`{"success":true,"id":%q,"displayOrder":%d,"groupType":%q}`, id, displayOrder, p.GroupType), nil
}

func (a *App) execCreateStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Title        string `json:"title"`
		Content      string `json:"content"`
		Source       string `json:"source"`
		Lifecycle    string `json:"lifecycle"`
		ParentExamID string `json:"parentExamId"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if strings.TrimSpace(p.Title) == "" {
		return errValidationFailed("title", "title is required"), nil
	}
	if strings.TrimSpace(p.Content) == "" {
		return errValidationFailed("content", "content is required"), nil
	}
	if p.Lifecycle == "" {
		p.Lifecycle = "shared"
	}
	if p.Lifecycle != "shared" && p.Lifecycle != "exam_scoped" {
		return errValidationFailed("lifecycle", "lifecycle must be 'shared' or 'exam_scoped'"), nil
	}
	if p.Lifecycle == "exam_scoped" && !isUUID(p.ParentExamID) {
		return errValidationFailed("parentExamId", "parentExamId required when lifecycle=exam_scoped"), nil
	}
	if p.ParentExamID != "" {
		if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ParentExamID); denied != "" {
			return denied, nil
		}
	}
	var id string
	if err := a.db.QueryRowContext(ctx, `
		INSERT INTO stimuli (
		    tenant_id, owner_user_id, type, title, content, source,
		    lifecycle, parent_exam_id
		) VALUES ($1, $2, 'text', $3, $4, NULLIF($5,''),
		          $6, NULLIF($7,'')::uuid)
		RETURNING id::text`,
		tenantID, userID, strings.TrimSpace(p.Title), p.Content, strings.TrimSpace(p.Source),
		p.Lifecycle, p.ParentExamID,
	).Scan(&id); err != nil {
		return "", err
	}
	if p.ParentExamID != "" {
		markExamAIContextStaleDB(ctx, a.db, tenantID, p.ParentExamID)
	}
	a.auditAI(ctx, tenantID, userID, "stimuli.create", "stimulus", id)
	return fmt.Sprintf(`{"success":true,"id":%q,"lifecycle":%q}`, id, p.Lifecycle), nil
}

func (a *App) execMoveQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		QuestionID string `json:"questionId"`
		SectionID  string `json:"sectionId"`
		GroupID    string `json:"groupId"`
		SortOrder  *int   `json:"sortOrder"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.QuestionID) {
		return errInvalidUUID("questionId", p.QuestionID, "question"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx,
		`SELECT exam_id::text FROM exam_questions WHERE id = $1 AND tenant_id = $2`,
		p.QuestionID, tenantID,
	).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("question", "questionId", p.QuestionID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	parts := []string{}
	vals := []any{}
	idx := 1
	add := func(col string, v any) {
		parts = append(parts, fmt.Sprintf("%s = $%d", col, idx))
		vals = append(vals, v)
		idx++
	}
	if p.SectionID != "" {
		if !isUUID(p.SectionID) {
			return errInvalidUUID("sectionId", p.SectionID, "section"), nil
		}
		add("section_id", p.SectionID)
	}
	if p.GroupID != "" {
		if !isUUID(p.GroupID) {
			return errInvalidUUID("groupId", p.GroupID, "group"), nil
		}
		add("group_id", p.GroupID)
	}
	if p.SortOrder != nil {
		add("sort_order", *p.SortOrder)
	}
	if len(parts) == 0 {
		return errValidationFailed("any",
			"set at least one of sectionId / groupId / sortOrder"), nil
	}
	parts = append(parts, "updated_at = now()")
	q := fmt.Sprintf("UPDATE exam_questions SET %s WHERE id = $%d AND tenant_id = $%d",
		strings.Join(parts, ", "), idx, idx+1)
	vals = append(vals, p.QuestionID, tenantID)
	if _, err := a.db.ExecContext(ctx, q, vals...); err != nil {
		return "", err
	}
	markExamAIContextStaleDB(ctx, a.db, tenantID, examID)
	a.auditAI(ctx, tenantID, userID, "questions.move", "exam_question", p.QuestionID)
	return fmt.Sprintf(`{"success":true,"questionId":%q}`, p.QuestionID), nil
}
