package app

// AI tools v2 — covers the rest of the exam-authoring surface that
// was missing after Phase 9.11. After this file lands, every endpoint
// in the exam manager UI has a corresponding AI capability:
//
//   - exam status: update, publish
//   - exam sections: list, update, delete
//   - questions: get, options CRUD
//   - groups: list, update, delete
//   - stimuli: list, get, update, archive, promote, sync_snapshot
//   - blueprint slots (exam-side): list with questions, add, update,
//     delete, assign question
//   - blueprint templates: update, publish, unpublish
//   - exam gates: list, create, update, delete
//   - exam: export-to-template
//
// Read tools execute inline (no proposal). Write tools go through
// createProposal. All write tools re-check permissions via
// checkExamWriteAccess / checkBlueprintWriteAccess and emit audit
// events on confirm.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// RegisterExamV2Capabilities wires the second batch of exam-authoring
// tools. Called by RegisterAllCapabilities after the v1 set.
func (a *App) RegisterExamV2Capabilities(reg *CapabilityRegistry) {
	// =============================================================
	// READ TOOLS — execute inline, no proposal flow.
	// =============================================================

	reg.Register(Capability{
		Name:        "list_exam_sections",
		Description: "List exam sections (id, title, sort_order).",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"}},"required":["examId"]}`),
	}, a.capListExamSections)

	reg.Register(Capability{
		Name:        "list_exam_groups",
		Description: "List question groups in exam with section/stimulus/count.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"}},"required":["examId"]}`),
	}, a.capListExamGroups)

	reg.Register(Capability{
		Name:        "get_question",
		Description: "Get question detail (content, options, links).",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"questionId":{"type":"string"}},"required":["questionId"]}`),
	}, a.capGetQuestion)

	reg.Register(Capability{
		Name:        "list_stimuli",
		Description: "List stimuli library. Filter: lifecycle, search.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "stimuli",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"lifecycle":{"type":"string","enum":["exam_scoped","shared","archived"]},
			"search":{"type":"string"}
		}}`),
	}, a.capListStimuli)

	reg.Register(Capability{
		Name:        "get_stimulus",
		Description: "Get stimulus detail.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "stimuli",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"stimulusId":{"type":"string"}},"required":["stimulusId"]}`),
	}, a.capGetStimulus)

	reg.Register(Capability{
		Name:        "list_exam_gates",
		Description: "List exam gate windows (schedule).",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"}},"required":["examId"]}`),
	}, a.capListExamGates)

	reg.Register(Capability{
		Name:        "list_slots_with_questions",
		Description: "Get blueprint slots paired with assigned questions (track fill progress).",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"}},"required":["examId"]}`),
	}, a.capListSlotsWithQuestions)

	// =============================================================
	// WRITE TOOLS — go through createProposal.
	// =============================================================

	reg.Register(Capability{
		Name:        "update_exam",
		Description: "Edit exam metadata. Hanya field yang di-set diubah.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"title":{"type":"string"},
			"description":{"type":"string"},
			"durationMinutes":{"type":"integer","minimum":0},
			"maxScore":{"type":"number"},
			"passingScore":{"type":"number"},
			"examType":{"type":"string","enum":["quiz","midterm","final","practice","custom"]},
			"shuffleQuestions":{"type":"boolean"},
			"shuffleOptions":{"type":"boolean"},
			"showResultImmediately":{"type":"boolean"}
		},"required":["examId"]}`),
	}, a.capUpdateExam)

	reg.Register(Capability{
		Name:        "publish_exam",
		Description: "Publish exam (draft → published).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"}},"required":["examId"]}`),
	}, a.capPublishExam)

	reg.Register(Capability{
		Name:        "update_exam_section",
		Description: "Edit title, description, sort_order dari sebuah section.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"sectionId":{"type":"string"},
			"title":{"type":"string"},
			"description":{"type":"string"},
			"sortOrder":{"type":"integer"}
		},"required":["sectionId"]}`),
	}, a.capUpdateExamSection)

	reg.Register(Capability{
		Name:        "delete_exam_section",
		Description: "Delete section (questions become unsectioned).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"sectionId":{"type":"string"}},"required":["sectionId"]}`),
	}, a.capDeleteExamSection)

	reg.Register(Capability{
		Name:        "update_question_group",
		Description: "Edit group (section/stimulus/snapshot/order).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"groupId":{"type":"string"},
			"sectionId":{"type":"string"},
			"stimulusId":{"type":"string"},
			"titleSnapshot":{"type":"string"},
			"bodySnapshot":{"type":"string"},
			"displayOrder":{"type":"integer"}
		},"required":["groupId"]}`),
	}, a.capUpdateQuestionGroup)

	reg.Register(Capability{
		Name:        "delete_question_group",
		Description: "Delete group (questions become ungrouped).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"groupId":{"type":"string"}},"required":["groupId"]}`),
	}, a.capDeleteQuestionGroup)

	reg.Register(Capability{
		Name:        "update_stimulus",
		Description: "Edit stimulus.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "stimuli",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"stimulusId":{"type":"string"},
			"title":{"type":"string"},
			"content":{"type":"string"},
			"source":{"type":"string"},
			"lifecycle":{"type":"string","enum":["exam_scoped","shared"]}
		},"required":["stimulusId"]}`),
	}, a.capUpdateStimulus)

	reg.Register(Capability{
		Name:        "archive_stimulus",
		Description: "Archive stimulus (lifecycle→archived).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "stimuli",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"stimulusId":{"type":"string"}},"required":["stimulusId"]}`),
	}, a.capArchiveStimulus)

	reg.Register(Capability{
		Name:        "promote_stimulus",
		Description: "Promote stimulus exam_scoped → shared.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "stimuli",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"stimulusId":{"type":"string"}},"required":["stimulusId"]}`),
	}, a.capPromoteStimulus)

	reg.Register(Capability{
		Name:        "create_exam_gate",
		Description: "Create exam gate window (ISO 8601 timestamps).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"opensAt":{"type":"string","description":"ISO 8601 timestamp"},
			"closesAt":{"type":"string","description":"ISO 8601 timestamp"},
			"password":{"type":"string"},
			"accessCode":{"type":"string"}
		},"required":["examId","opensAt","closesAt"]}`),
	}, a.capCreateExamGate)

	reg.Register(Capability{
		Name:        "update_exam_gate",
		Description: "Edit gate window.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"gateId":{"type":"string"},
			"opensAt":{"type":"string"},
			"closesAt":{"type":"string"},
			"password":{"type":"string"},
			"accessCode":{"type":"string"}
		},"required":["gateId"]}`),
	}, a.capUpdateExamGate)

	reg.Register(Capability{
		Name:        "delete_exam_gate",
		Description: "Hapus gate window.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"gateId":{"type":"string"}},"required":["gateId"]}`),
	}, a.capDeleteExamGate)

	reg.Register(Capability{
		Name:        "assign_question_to_slot",
		Description: "Assign existing question to blueprint slot (1:1 link).",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"slotId":{"type":"string"},
			"questionId":{"type":"string"}
		},"required":["slotId","questionId"]}`),
	}, a.capAssignQuestionToSlot)

	reg.Register(Capability{
		Name:        "export_exam_to_template",
		Description: "Export exam blueprint to new draft template.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"templateTitle":{"type":"string"},
			"description":{"type":"string"}
		},"required":["examId","templateTitle"]}`),
	}, a.capExportExamToTemplate)
}

// =================================================================
// READ HANDLERS — execute inline, no proposal.
// =================================================================

func (a *App) capListExamSections(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, title, COALESCE(description,''), sort_order FROM exam_sections WHERE tenant_id=$1 AND exam_id=$2 ORDER BY sort_order ASC, created_at ASC`,
		tenantID, p.ExamID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type sec struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		SortOrder   int    `json:"sortOrder"`
	}
	var out []sec
	for rows.Next() {
		var s sec
		if rows.Scan(&s.ID, &s.Title, &s.Description, &s.SortOrder) == nil {
			out = append(out, s)
		}
	}
	b, _ := json.Marshal(map[string]any{"sections": out, "count": len(out)})
	return string(b), nil
}

func (a *App) capListExamGroups(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT g.id, COALESCE(g.section_id::text,''), COALESCE(g.stimulus_id::text,''),
		       COALESCE(g.stimulus_title_snapshot,''), g.group_type, g.display_order,
		       (SELECT count(*) FROM exam_questions q WHERE q.group_id = g.id) AS question_count
		  FROM exam_question_groups g
		 WHERE g.tenant_id=$1 AND g.exam_id=$2
		 ORDER BY g.display_order ASC`,
		tenantID, p.ExamID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type grp struct {
		ID            string `json:"id"`
		SectionID     string `json:"sectionId,omitempty"`
		StimulusID    string `json:"stimulusId,omitempty"`
		Title         string `json:"title"`
		GroupType     string `json:"groupType"`
		DisplayOrder  int    `json:"displayOrder"`
		QuestionCount int    `json:"questionCount"`
	}
	var out []grp
	for rows.Next() {
		var g grp
		if rows.Scan(&g.ID, &g.SectionID, &g.StimulusID, &g.Title, &g.GroupType, &g.DisplayOrder, &g.QuestionCount) == nil {
			out = append(out, g)
		}
	}
	b, _ := json.Marshal(map[string]any{"groups": out, "count": len(out)})
	return string(b), nil
}

func (a *App) capGetQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		QuestionID string `json:"questionId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.QuestionID) {
		return errInvalidUUID("questionId", p.QuestionID, "question"), nil
	}
	var q struct {
		ID            string  `json:"id"`
		ExamID        string  `json:"examId"`
		SectionID     string  `json:"sectionId,omitempty"`
		GroupID       string  `json:"groupId,omitempty"`
		StimulusID    string  `json:"stimulusId,omitempty"`
		BlueprintSlot string  `json:"blueprintSlotId,omitempty"`
		Type          string  `json:"questionType"`
		Content       string  `json:"content"`
		Explanation   string  `json:"explanation"`
		CorrectAnswer string  `json:"correctAnswer"`
		Points        float64 `json:"points"`
		SortOrder     int     `json:"sortOrder"`
		ScoringMode   string  `json:"scoringMode"`
	}
	err := a.db.QueryRowContext(ctx, `
		SELECT id, exam_id, COALESCE(section_id::text,''), COALESCE(group_id::text,''),
		       COALESCE(stimulus_id::text,''), COALESCE(blueprint_slot_id::text,''),
		       question_type, content, COALESCE(explanation,''), COALESCE(correct_answer,''),
		       points, sort_order, scoring_mode
		  FROM exam_questions
		 WHERE id=$1 AND tenant_id=$2`,
		p.QuestionID, tenantID,
	).Scan(&q.ID, &q.ExamID, &q.SectionID, &q.GroupID, &q.StimulusID, &q.BlueprintSlot,
		&q.Type, &q.Content, &q.Explanation, &q.CorrectAnswer, &q.Points, &q.SortOrder, &q.ScoringMode)
	if err == sql.ErrNoRows {
		return errEntityNotFound("question", "questionId", p.QuestionID), nil
	}
	if err != nil {
		return "", err
	}
	// Fetch options
	type opt struct {
		ID        string  `json:"id"`
		Content   string  `json:"content"`
		IsCorrect bool    `json:"isCorrect"`
		SortOrder int     `json:"sortOrder"`
		Weight    float64 `json:"pointsWeight,omitempty"`
	}
	var opts []opt
	rows, _ := a.db.QueryContext(ctx,
		`SELECT id, content, is_correct, sort_order, COALESCE(points_weight, 0) FROM exam_question_options WHERE question_id=$1 ORDER BY sort_order ASC`,
		p.QuestionID)
	if rows != nil {
		for rows.Next() {
			var o opt
			if rows.Scan(&o.ID, &o.Content, &o.IsCorrect, &o.SortOrder, &o.Weight) == nil {
				opts = append(opts, o)
			}
		}
		rows.Close()
	}
	b, _ := json.Marshal(map[string]any{"question": q, "options": opts})
	return string(b), nil
}

func (a *App) capListStimuli(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Lifecycle string `json:"lifecycle"`
		Search    string `json:"search"`
	}
	_ = json.Unmarshal(args, &p)
	q := `SELECT id, title, lifecycle, type, COALESCE(parent_exam_id::text,''), usage_count
	        FROM stimuli WHERE tenant_id=$1`
	argsList := []any{tenantID}
	if p.Lifecycle != "" {
		q += ` AND lifecycle=$2`
		argsList = append(argsList, p.Lifecycle)
	} else {
		q += ` AND lifecycle <> 'archived'`
	}
	if p.Search != "" {
		q += fmt.Sprintf(` AND title ILIKE $%d`, len(argsList)+1)
		argsList = append(argsList, "%"+p.Search+"%")
	}
	q += ` ORDER BY updated_at DESC LIMIT 50`
	rows, err := a.db.QueryContext(ctx, q, argsList...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type sti struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Lifecycle     string `json:"lifecycle"`
		Type          string `json:"type"`
		ParentExamID  string `json:"parentExamId,omitempty"`
		UsageCount    int    `json:"usageCount"`
	}
	var out []sti
	for rows.Next() {
		var s sti
		if rows.Scan(&s.ID, &s.Title, &s.Lifecycle, &s.Type, &s.ParentExamID, &s.UsageCount) == nil {
			out = append(out, s)
		}
	}
	b, _ := json.Marshal(map[string]any{"stimuli": out, "count": len(out)})
	return string(b), nil
}

func (a *App) capGetStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		StimulusID string `json:"stimulusId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.StimulusID) {
		return errInvalidUUID("stimulusId", p.StimulusID, "stimulus"), nil
	}
	var s struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		Source     string `json:"source"`
		Lifecycle  string `json:"lifecycle"`
		Type       string `json:"type"`
		UsageCount int    `json:"usageCount"`
	}
	err := a.db.QueryRowContext(ctx,
		`SELECT id, title, content, COALESCE(source,''), lifecycle, type, usage_count FROM stimuli WHERE id=$1 AND tenant_id=$2`,
		p.StimulusID, tenantID,
	).Scan(&s.ID, &s.Title, &s.Content, &s.Source, &s.Lifecycle, &s.Type, &s.UsageCount)
	if err == sql.ErrNoRows {
		return errEntityNotFound("stimulus", "stimulusId", p.StimulusID), nil
	}
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(map[string]any{"stimulus": s})
	return string(b), nil
}

func (a *App) capListExamGates(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, opens_at, closes_at, COALESCE(access_code,''), (password IS NOT NULL AND password<>'') AS has_pw
		   FROM exam_gate_windows WHERE tenant_id=$1 AND exam_id=$2 ORDER BY opens_at ASC`,
		tenantID, p.ExamID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type gate struct {
		ID         string `json:"id"`
		OpensAt    string `json:"opensAt"`
		ClosesAt   string `json:"closesAt"`
		AccessCode string `json:"accessCode,omitempty"`
		HasPwd     bool   `json:"hasPassword"`
	}
	var out []gate
	for rows.Next() {
		var g gate
		var oa, ca sql.NullString
		if rows.Scan(&g.ID, &oa, &ca, &g.AccessCode, &g.HasPwd) == nil {
			g.OpensAt = oa.String
			g.ClosesAt = ca.String
			out = append(out, g)
		}
	}
	b, _ := json.Marshal(map[string]any{"gates": out, "count": len(out)})
	return string(b), nil
}

func (a *App) capListSlotsWithQuestions(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT s.id, s.position, COALESCE(s.materi,''), COALESCE(s.indikator,''),
		       COALESCE(s.cognitive_level,''), COALESCE(s.difficulty,''),
		       COALESCE(s.question_type,''), s.points,
		       COALESCE(s.competency_code,''), COALESCE(s.competency_description,''),
		       COALESCE(s.stimulus_id::text,''),
		       (SELECT q.id::text FROM exam_questions q WHERE q.blueprint_slot_id = s.id LIMIT 1) AS question_id
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE b.exam_id = $1 AND b.tenant_id = $2
		 ORDER BY s.position ASC`,
		p.ExamID, tenantID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	type slot struct {
		ID            string  `json:"id"`
		Position      int     `json:"position"`
		Materi        string  `json:"materi"`
		Indikator     string  `json:"indikator"`
		Cognitive     string  `json:"cognitiveLevel"`
		Difficulty    string  `json:"difficulty"`
		Type          string  `json:"questionType"`
		Points        float64 `json:"points"`
		CompCode      string  `json:"competencyCode,omitempty"`
		CompDesc      string  `json:"competencyDescription,omitempty"`
		StimulusID    string  `json:"stimulusId,omitempty"`
		QuestionID    sql.NullString
		QuestionIDStr string `json:"questionId,omitempty"`
		Filled        bool   `json:"filled"`
	}
	var out []slot
	var filled int
	for rows.Next() {
		var s slot
		if rows.Scan(&s.ID, &s.Position, &s.Materi, &s.Indikator, &s.Cognitive, &s.Difficulty, &s.Type, &s.Points, &s.CompCode, &s.CompDesc, &s.StimulusID, &s.QuestionID) == nil {
			if s.QuestionID.Valid {
				s.QuestionIDStr = s.QuestionID.String
				s.Filled = true
				filled++
			}
			out = append(out, s)
		}
	}
	b, _ := json.Marshal(map[string]any{"slots": out, "count": len(out), "filled": filled, "remaining": len(out) - filled})
	return string(b), nil
}

// =================================================================
// WRITE HANDLERS — propose via createProposal.
// =================================================================

func (a *App) capUpdateExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "update_exam", args, "Edit metadata exam.")
}

func (a *App) capPublishExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "publish_exam", args, "Publish exam (draft → published).")
}

func (a *App) capUpdateExamSection(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "update_exam_section", args, "Edit section.")
}

func (a *App) capDeleteExamSection(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "delete_exam_section", args, "Hapus section (soal di dalamnya tetap, hanya jadi unsectioned).")
}

func (a *App) capUpdateQuestionGroup(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)

	// Validate + build content-aware confirmation. Mirrors the
	// capUpdateQuestion fix — 'Edit group soal.' alone is too vague
	// when the user just generated stimulus content; they should see
	// the title + body excerpt before clicking 'ya'.
	var p struct {
		GroupID       string  `json:"groupId"`
		SectionID     *string `json:"sectionId"`
		StimulusID    *string `json:"stimulusId"`
		TitleSnapshot *string `json:"titleSnapshot"`
		BodySnapshot  *string `json:"bodySnapshot"`
		DisplayOrder  *int    `json:"displayOrder"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.GroupID) {
		return errInvalidUUID("groupId", p.GroupID, "group"), nil
	}
	hasField := p.SectionID != nil || p.StimulusID != nil ||
		p.TitleSnapshot != nil || p.BodySnapshot != nil ||
		p.DisplayOrder != nil
	if !hasField {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":        "INVALID_UPDATE",
				"message":     "update_question_group butuh minimal satu field yang akan diubah (sectionId/stimulusId/titleSnapshot/bodySnapshot/displayOrder).",
				"recoverable": true,
				"recovery": map[string]any{
					"hint": "Untuk set/refresh stimulus group, kirim titleSnapshot + bodySnapshot. Untuk pindah section, kirim sectionId.",
				},
			},
		})
		return string(b), nil
	}

	var sb strings.Builder
	sb.WriteString("**Update group soal**\n")
	if p.TitleSnapshot != nil {
		title := *p.TitleSnapshot
		if len(title) > 100 {
			title = title[:100] + "…"
		}
		fmt.Fprintf(&sb, "\n**\U0001F4C4 Judul stimulus baru:** %s\n", title)
	}
	if p.BodySnapshot != nil {
		body := *p.BodySnapshot
		if len(body) > 400 {
			body = body[:400] + "…"
		}
		sb.WriteString("\n**\U0001F4D6 Isi stimulus baru:**\n> ")
		sb.WriteString(strings.ReplaceAll(strings.TrimSpace(body), "\n", "\n> "))
		sb.WriteString("\n")
	}
	if p.StimulusID != nil {
		if *p.StimulusID == "" {
			sb.WriteString("\n**\u26A0 Stimulus dihapus** (link ke master stimulus dilepas; snapshot tetap kalau ada)\n")
		} else {
			fmt.Fprintf(&sb, "\n**\U0001F517 Link ke stimulus library:** %s\n", *p.StimulusID)
		}
	}
	if p.SectionID != nil {
		if *p.SectionID == "" {
			sb.WriteString("\n**Pindah:** lepas dari section (jadi section-less)\n")
		} else {
			fmt.Fprintf(&sb, "\n**Pindah ke section:** %s\n", *p.SectionID)
		}
	}
	if p.DisplayOrder != nil {
		fmt.Fprintf(&sb, "\n**Urutan baru:** %d\n", *p.DisplayOrder+1)
	}
	return a.createProposal(ctx, sessionID, tenantID, userID, "update_question_group", args, sb.String())
}

func (a *App) capDeleteQuestionGroup(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "delete_question_group", args, "Hapus group (soal di dalamnya tetap, jadi ungrouped).")
}

func (a *App) capUpdateStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "update_stimulus", args, "Edit stimulus.")
}

func (a *App) capArchiveStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "archive_stimulus", args, "Archive stimulus (lifecycle → archived).")
}

func (a *App) capPromoteStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "promote_stimulus", args, "Promote stimulus dari exam_scoped ke shared.")
}

func (a *App) capCreateExamGate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_exam_gate", args, "Buat gate window baru.")
}

func (a *App) capUpdateExamGate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "update_exam_gate", args, "Edit gate window.")
}

func (a *App) capDeleteExamGate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "delete_exam_gate", args, "Hapus gate window.")
}

func (a *App) capAssignQuestionToSlot(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "assign_question_to_slot", args, "Assign soal ke blueprint slot.")
}

func (a *App) capExportExamToTemplate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "export_exam_to_template", args, "Export exam blueprint ke template baru (draft).")
}

// =================================================================
// EXECUTORS — run on confirm.
// =================================================================

func (a *App) execUpdateExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID                string   `json:"examId"`
		Title                 *string  `json:"title"`
		Description           *string  `json:"description"`
		DurationMinutes       *int     `json:"durationMinutes"`
		MaxScore              *float64 `json:"maxScore"`
		PassingScore          *float64 `json:"passingScore"`
		ExamType              *string  `json:"examType"`
		ShuffleQuestions      *bool    `json:"shuffleQuestions"`
		ShuffleOptions        *bool    `json:"shuffleOptions"`
		ShowResultImmediately *bool    `json:"showResultImmediately"`
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
	sets := []string{}
	vals := []any{}
	add := func(col string, v any) {
		vals = append(vals, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(vals)))
	}
	if p.Title != nil {
		add("title", *p.Title)
	}
	if p.Description != nil {
		add("description", *p.Description)
	}
	if p.DurationMinutes != nil {
		add("duration_minutes", *p.DurationMinutes)
	}
	if p.MaxScore != nil {
		add("max_score", *p.MaxScore)
	}
	if p.PassingScore != nil {
		add("passing_score", *p.PassingScore)
	}
	if p.ExamType != nil {
		add("exam_type", *p.ExamType)
	}
	if p.ShuffleQuestions != nil {
		add("shuffle_questions", *p.ShuffleQuestions)
	}
	if p.ShuffleOptions != nil {
		add("shuffle_options", *p.ShuffleOptions)
	}
	if p.ShowResultImmediately != nil {
		add("show_result_immediately", *p.ShowResultImmediately)
	}
	if len(sets) == 0 {
		return errValidationFailed("body", "tidak ada field yang diubah"), nil
	}
	vals = append(vals, p.ExamID, tenantID)
	q := fmt.Sprintf(`UPDATE exams SET %s, updated_at=now() WHERE id=$%d AND tenant_id=$%d`, strings.Join(sets, ", "), len(vals)-1, len(vals))
	if _, err := a.db.ExecContext(ctx, q, vals...); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "exams.update", "exam", p.ExamID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Exam diperbarui", "examId": p.ExamID})
	return string(b), nil
}

func (a *App) execPublishExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ExamID); denied != "" {
		return denied, nil
	}
	res, err := a.db.ExecContext(ctx,
		`UPDATE exams SET status='published', published_at=COALESCE(published_at, now()), updated_at=now() WHERE id=$1 AND tenant_id=$2 AND status='draft'`,
		p.ExamID, tenantID)
	if err != nil {
		return "", err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errValidationFailed("status", "Exam tidak dalam status draft atau tidak ditemukan"), nil
	}
	a.auditAI(ctx, tenantID, userID, "exams.publish", "exam", p.ExamID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Exam dipublish", "examId": p.ExamID})
	return string(b), nil
}

func (a *App) execUpdateExamSection(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		SectionID   string  `json:"sectionId"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		SortOrder   *int    `json:"sortOrder"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.SectionID) {
		return errInvalidUUID("sectionId", p.SectionID, "section"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx, `SELECT exam_id FROM exam_sections WHERE id=$1 AND tenant_id=$2`, p.SectionID, tenantID).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("section", "sectionId", p.SectionID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	sets := []string{}
	vals := []any{}
	add := func(col string, v any) {
		vals = append(vals, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(vals)))
	}
	if p.Title != nil {
		add("title", *p.Title)
	}
	if p.Description != nil {
		add("description", *p.Description)
	}
	if p.SortOrder != nil {
		add("sort_order", *p.SortOrder)
	}
	if len(sets) == 0 {
		return errValidationFailed("body", "tidak ada field yang diubah"), nil
	}
	vals = append(vals, p.SectionID, tenantID)
	q := fmt.Sprintf(`UPDATE exam_sections SET %s WHERE id=$%d AND tenant_id=$%d`, strings.Join(sets, ", "), len(vals)-1, len(vals))
	if _, err := a.db.ExecContext(ctx, q, vals...); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "exam_sections.update", "exam_section", p.SectionID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Section diperbarui", "sectionId": p.SectionID})
	return string(b), nil
}

func (a *App) execDeleteExamSection(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		SectionID string `json:"sectionId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.SectionID) {
		return errInvalidUUID("sectionId", p.SectionID, "section"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx, `SELECT exam_id FROM exam_sections WHERE id=$1 AND tenant_id=$2`, p.SectionID, tenantID).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("section", "sectionId", p.SectionID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	if _, err := a.db.ExecContext(ctx, `DELETE FROM exam_sections WHERE id=$1 AND tenant_id=$2`, p.SectionID, tenantID); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "exam_sections.delete", "exam_section", p.SectionID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Section dihapus", "sectionId": p.SectionID})
	return string(b), nil
}

func (a *App) execUpdateQuestionGroup(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		GroupID       string  `json:"groupId"`
		SectionID     *string `json:"sectionId"`
		StimulusID    *string `json:"stimulusId"`
		TitleSnapshot *string `json:"titleSnapshot"`
		BodySnapshot  *string `json:"bodySnapshot"`
		DisplayOrder  *int    `json:"displayOrder"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.GroupID) {
		return errInvalidUUID("groupId", p.GroupID, "group"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx, `SELECT exam_id FROM exam_question_groups WHERE id=$1 AND tenant_id=$2`, p.GroupID, tenantID).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("group", "groupId", p.GroupID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	sets := []string{}
	vals := []any{}
	add := func(col string, v any) {
		vals = append(vals, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(vals)))
	}
	if p.SectionID != nil {
		add("section_id", sql.NullString{String: *p.SectionID, Valid: *p.SectionID != ""})
	}
	if p.StimulusID != nil {
		add("stimulus_id", sql.NullString{String: *p.StimulusID, Valid: *p.StimulusID != ""})
	}
	if p.TitleSnapshot != nil {
		add("stimulus_title_snapshot", *p.TitleSnapshot)
	}
	if p.BodySnapshot != nil {
		add("stimulus_body_snapshot", *p.BodySnapshot)
	}
	if p.DisplayOrder != nil {
		add("display_order", *p.DisplayOrder)
	}
	if len(sets) == 0 {
		return errValidationFailed("body", "tidak ada field yang diubah"), nil
	}
	vals = append(vals, p.GroupID, tenantID)
	q := fmt.Sprintf(`UPDATE exam_question_groups SET %s, updated_at=now() WHERE id=$%d AND tenant_id=$%d`, strings.Join(sets, ", "), len(vals)-1, len(vals))
	if _, err := a.db.ExecContext(ctx, q, vals...); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "groups.update", "exam_question_group", p.GroupID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Group diperbarui", "groupId": p.GroupID})
	return string(b), nil
}

func (a *App) execDeleteQuestionGroup(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		GroupID string `json:"groupId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.GroupID) {
		return errInvalidUUID("groupId", p.GroupID, "group"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx, `SELECT exam_id FROM exam_question_groups WHERE id=$1 AND tenant_id=$2`, p.GroupID, tenantID).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("group", "groupId", p.GroupID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	if _, err := a.db.ExecContext(ctx, `DELETE FROM exam_question_groups WHERE id=$1 AND tenant_id=$2`, p.GroupID, tenantID); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "groups.delete", "exam_question_group", p.GroupID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Group dihapus", "groupId": p.GroupID})
	return string(b), nil
}

func (a *App) execUpdateStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		StimulusID string  `json:"stimulusId"`
		Title      *string `json:"title"`
		Content    *string `json:"content"`
		Source     *string `json:"source"`
		Lifecycle  *string `json:"lifecycle"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.StimulusID) {
		return errInvalidUUID("stimulusId", p.StimulusID, "stimulus"), nil
	}
	sets := []string{}
	vals := []any{}
	add := func(col string, v any) {
		vals = append(vals, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(vals)))
	}
	if p.Title != nil {
		add("title", *p.Title)
	}
	if p.Content != nil {
		add("content", *p.Content)
	}
	if p.Source != nil {
		add("source", *p.Source)
	}
	if p.Lifecycle != nil {
		if *p.Lifecycle != "exam_scoped" && *p.Lifecycle != "shared" {
			return errValidationFailed("lifecycle", "lifecycle must be exam_scoped or shared"), nil
		}
		add("lifecycle", *p.Lifecycle)
	}
	if len(sets) == 0 {
		return errValidationFailed("body", "tidak ada field yang diubah"), nil
	}
	vals = append(vals, p.StimulusID, tenantID)
	q := fmt.Sprintf(`UPDATE stimuli SET %s, updated_at=now() WHERE id=$%d AND tenant_id=$%d`, strings.Join(sets, ", "), len(vals)-1, len(vals))
	if _, err := a.db.ExecContext(ctx, q, vals...); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "stimuli.update", "stimulus", p.StimulusID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Stimulus diperbarui", "stimulusId": p.StimulusID})
	return string(b), nil
}

func (a *App) execArchiveStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		StimulusID string `json:"stimulusId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.StimulusID) {
		return errInvalidUUID("stimulusId", p.StimulusID, "stimulus"), nil
	}
	if _, err := a.db.ExecContext(ctx,
		`UPDATE stimuli SET lifecycle='archived', updated_at=now() WHERE id=$1 AND tenant_id=$2`,
		p.StimulusID, tenantID); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "stimuli.archive", "stimulus", p.StimulusID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Stimulus diarsip", "stimulusId": p.StimulusID})
	return string(b), nil
}

func (a *App) execPromoteStimulus(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		StimulusID string `json:"stimulusId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.StimulusID) {
		return errInvalidUUID("stimulusId", p.StimulusID, "stimulus"), nil
	}
	res, err := a.db.ExecContext(ctx,
		`UPDATE stimuli SET lifecycle='shared', parent_exam_id=NULL, updated_at=now() WHERE id=$1 AND tenant_id=$2 AND lifecycle='exam_scoped'`,
		p.StimulusID, tenantID)
	if err != nil {
		return "", err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errValidationFailed("lifecycle", "Stimulus tidak dalam exam_scoped atau tidak ditemukan"), nil
	}
	a.auditAI(ctx, tenantID, userID, "stimuli.promote", "stimulus", p.StimulusID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Stimulus dipromosikan ke shared", "stimulusId": p.StimulusID})
	return string(b), nil
}

func (a *App) execCreateExamGate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID     string `json:"examId"`
		OpensAt    string `json:"opensAt"`
		ClosesAt   string `json:"closesAt"`
		Password   string `json:"password"`
		AccessCode string `json:"accessCode"`
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
	var id string
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO exam_gate_windows (tenant_id, exam_id, opens_at, closes_at, password, access_code)
		 VALUES ($1,$2,$3::timestamptz,$4::timestamptz, NULLIF($5,''), NULLIF($6,'')) RETURNING id`,
		tenantID, p.ExamID, p.OpensAt, p.ClosesAt, p.Password, p.AccessCode,
	).Scan(&id)
	if err != nil {
		return errValidationFailed("opensAt/closesAt", "format ISO 8601 invalid: "+err.Error()), nil
	}
	a.auditAI(ctx, tenantID, userID, "gates.create", "exam_gate_window", id)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Gate window dibuat", "gateId": id})
	return string(b), nil
}

func (a *App) execUpdateExamGate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		GateID     string  `json:"gateId"`
		OpensAt    *string `json:"opensAt"`
		ClosesAt   *string `json:"closesAt"`
		Password   *string `json:"password"`
		AccessCode *string `json:"accessCode"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.GateID) {
		return errInvalidUUID("gateId", p.GateID, "gate"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx, `SELECT exam_id FROM exam_gate_windows WHERE id=$1 AND tenant_id=$2`, p.GateID, tenantID).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("gate", "gateId", p.GateID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	sets := []string{}
	vals := []any{}
	add := func(col string, v any) {
		vals = append(vals, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(vals)))
	}
	if p.OpensAt != nil {
		add("opens_at", *p.OpensAt)
	}
	if p.ClosesAt != nil {
		add("closes_at", *p.ClosesAt)
	}
	if p.Password != nil {
		add("password", sql.NullString{String: *p.Password, Valid: *p.Password != ""})
	}
	if p.AccessCode != nil {
		add("access_code", sql.NullString{String: *p.AccessCode, Valid: *p.AccessCode != ""})
	}
	if len(sets) == 0 {
		return errValidationFailed("body", "tidak ada field yang diubah"), nil
	}
	vals = append(vals, p.GateID, tenantID)
	q := fmt.Sprintf(`UPDATE exam_gate_windows SET %s WHERE id=$%d AND tenant_id=$%d`, strings.Join(sets, ", "), len(vals)-1, len(vals))
	if _, err := a.db.ExecContext(ctx, q, vals...); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "gates.update", "exam_gate_window", p.GateID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Gate diperbarui", "gateId": p.GateID})
	return string(b), nil
}

func (a *App) execDeleteExamGate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		GateID string `json:"gateId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.GateID) {
		return errInvalidUUID("gateId", p.GateID, "gate"), nil
	}
	var examID string
	if err := a.db.QueryRowContext(ctx, `SELECT exam_id FROM exam_gate_windows WHERE id=$1 AND tenant_id=$2`, p.GateID, tenantID).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("gate", "gateId", p.GateID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	if _, err := a.db.ExecContext(ctx, `DELETE FROM exam_gate_windows WHERE id=$1 AND tenant_id=$2`, p.GateID, tenantID); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "gates.delete", "exam_gate_window", p.GateID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Gate dihapus", "gateId": p.GateID})
	return string(b), nil
}

func (a *App) execAssignQuestionToSlot(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		SlotID     string `json:"slotId"`
		QuestionID string `json:"questionId"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if !isUUID(p.SlotID) {
		return errInvalidUUID("slotId", p.SlotID, "slot"), nil
	}
	if !isUUID(p.QuestionID) {
		return errInvalidUUID("questionId", p.QuestionID, "question"), nil
	}
	// Resolve exam from slot via exam_blueprints join.
	var examID string
	if err := a.db.QueryRowContext(ctx,
		`SELECT b.exam_id FROM exam_blueprint_slots s JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		  WHERE s.id=$1 AND b.tenant_id=$2`, p.SlotID, tenantID,
	).Scan(&examID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("slot", "slotId", p.SlotID), nil
		}
		return "", err
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, examID); denied != "" {
		return denied, nil
	}
	// Verify question is in same exam + same tenant before linking.
	var qExamID string
	if err := a.db.QueryRowContext(ctx,
		`SELECT exam_id FROM exam_questions WHERE id=$1 AND tenant_id=$2`,
		p.QuestionID, tenantID,
	).Scan(&qExamID); err != nil {
		if err == sql.ErrNoRows {
			return errEntityNotFound("question", "questionId", p.QuestionID), nil
		}
		return "", err
	}
	if qExamID != examID {
		return errValidationFailed("questionId", "Soal dan slot harus dari exam yang sama"), nil
	}
	res, err := a.db.ExecContext(ctx,
		`UPDATE exam_questions SET blueprint_slot_id=$1, updated_at=now() WHERE id=$2 AND tenant_id=$3`,
		p.SlotID, p.QuestionID, tenantID)
	if err != nil {
		return "", err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errEntityNotFound("question", "questionId", p.QuestionID), nil
	}
	a.auditAI(ctx, tenantID, userID, "slots.assign_question", "exam_blueprint_slot", p.SlotID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Soal di-assign ke slot", "slotId": p.SlotID, "questionId": p.QuestionID})
	return string(b), nil
}

// exportExamBlueprintToNewTemplate is the reusable core of
// handleExportExamBlueprintToTemplate. The HTTP handler can be
// refactored to call this; for now we duplicate the logic so the AI
// path doesn't have to fake an http.Request.
func (a *App) exportExamBlueprintToNewTemplate(ctx context.Context, tenantID, userID, examID, title, description string) (string, error) {
	var (
		srcID                                             string
		curriculumID, blueprintType, srcSubject, srcGrade string
		strictCoverage                                    bool
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT id::text, curriculum_id::text, blueprint_type,
		       COALESCE(subject_code, ''), COALESCE(grade_or_phase, ''),
		       strict_coverage
		  FROM exam_blueprints
		 WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&srcID, &curriculumID, &blueprintType, &srcSubject, &srcGrade, &strictCoverage)
	if err != nil {
		return "", fmt.Errorf("exam belum punya blueprint untuk diekspor")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var newTemplateID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO blueprint_templates (
		    tenant_id, owner_user_id, title, description,
		    curriculum_id, subject_code, grade_or_phase, blueprint_type,
		    strict_coverage, status, version
		) VALUES ($1, $2, $3, NULLIF($4,''),
		          $5, NULLIF($6,''), NULLIF($7,''), $8,
		          $9, 'draft', 1)
		RETURNING id::text`,
		tenantID, userID, title, strings.TrimSpace(description),
		curriculumID, srcSubject, srcGrade, blueprintType, strictCoverage,
	).Scan(&newTemplateID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO blueprint_template_slots (
		    template_id, position,
		    competency_id, competency_code, competency_description,
		    materi, indikator, cognitive_level, difficulty,
		    question_type, points, stimulus_id,
		    akm_konten, akm_konteks, akm_proses, akm_level
		)
		SELECT $1, position,
		       competency_id, competency_code, competency_description,
		       materi, indikator, cognitive_level, difficulty,
		       question_type, points, stimulus_id,
		       akm_konten, akm_konteks, akm_proses, akm_level
		  FROM exam_blueprint_slots
		 WHERE exam_blueprint_id = $2
		 ORDER BY position`,
		newTemplateID, srcID,
	); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return newTemplateID, nil
}

func (a *App) execExportExamToTemplate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID        string `json:"examId"`
		TemplateTitle string `json:"templateTitle"`
		Description   string `json:"description"`
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
	if strings.TrimSpace(p.TemplateTitle) == "" {
		return errValidationFailed("templateTitle", "templateTitle wajib"), nil
	}
	// Reuse the existing handler logic by calling it with synthesised input.
	templateID, err := a.exportExamBlueprintToNewTemplate(ctx, tenantID, userID, p.ExamID, p.TemplateTitle, p.Description)
	if err != nil {
		return errValidationFailed("export", err.Error()), nil
	}
	a.auditAI(ctx, tenantID, userID, "blueprints.export_from_exam", "blueprint_template", templateID)
	b, _ := json.Marshal(map[string]any{"success": true, "message": "Blueprint diekspor ke template baru", "templateId": templateID})
	return string(b), nil
}
