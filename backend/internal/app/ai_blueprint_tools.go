package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// AI Blueprint authoring tools — registered into CapabilityRegistry under
// the "blueprints" domain. Implements Phase 9.5 capabilities per ADR-0010
// using the same propose-then-confirm pattern established in Phase 9.
//
// Read capabilities (return data directly):
//   list_blueprint_templates     exams:read
//   get_blueprint_template       exams:read
//   get_exam_blueprint           exams:read
//   list_blueprint_slots         exams:read
//   analyze_questions_to_blueprint  exams:read   ← reverse-flow analysis,
//                                                  no DB writes
//
// Write capabilities (proposal-first):
//   create_blueprint_template    blueprints:write
//   add_blueprint_slot           blueprints:write
//   bulk_add_blueprint_slots     blueprints:write
//   clone_blueprint_to_exam      blueprints:write
//   generate_question_for_slot   exams:write
//   apply_blueprint_analysis     blueprints:write   ← companion to analyze,
//                                                     persists accepted slots
//
// Why analyze + apply is a 2-step proposal-first flow:
// the bot can suggest a kisi-kisi reverse-engineered from existing
// questions, but a teacher must explicitly pick which suggestions to
// accept before any DB write happens (ADR-0010, council mandate).

func (a *App) registerBlueprintCapabilities(reg *CapabilityRegistry) {
	// ───── Read ─────
	reg.Register(Capability{
		Name:        "list_blueprint_templates",
		Description: "List blueprint templates. Filter: curriculum, blueprintType, status.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"search":{"type":"string"},
			"curriculum":{"type":"string","enum":["k13","merdeka","akm_numerasi","akm_literasi"]},
			"blueprintType":{"type":"string","enum":["reguler","akm_literasi","akm_numerasi"]},
			"status":{"type":"string","enum":["draft","published","archived"]},
			"limit":{"type":"integer","default":20}
		}}`),
	}, a.capListBlueprintTemplates)

	reg.Register(Capability{
		Name:        "get_blueprint_template",
		Description: "Get blueprint template + slots.",
		Permission:  "blueprints:read",
		Risk:        "read",
		Domain:      "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"templateId":{"type":"string"}
		},"required":["templateId"]}`),
	}, a.capGetBlueprintTemplate)

	reg.Register(Capability{
		Name:        "get_exam_blueprint",
		Description: "Get exam blueprint + slot coverage %.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"}
		},"required":["examId"]}`),
	}, a.capGetExamBlueprintAI)

	reg.Register(Capability{
		Name:        "list_blueprint_slots",
		Description: "List slots of exam blueprint with filled status.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"}
		},"required":["examId"]}`),
	}, a.capListExamBlueprintSlotsAI)

	reg.Register(Capability{
		Name: "analyze_questions_to_blueprint",
		Description: "Reverse-engineer kisi-kisi from existing questions. Read-only — returns proposed_slots/links/distribution. Cap 50 questions. minConfidence default 0.5.",
		Permission: "exams:read",
		Risk:       "read",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"minConfidence":{"type":"number","default":0.5,"minimum":0,"maximum":1},
			"batchSize":{"type":"integer","default":50,"minimum":1,"maximum":50}
		},"required":["examId"]}`),
	}, a.capAnalyzeQuestionsToBlueprint)

	// ───── Write (proposal-first) ─────
	reg.Register(Capability{
		Name: "create_blueprint_template",
		Description: "Create blueprint template (reusable kisi-kisi). curriculumCode required.",
		Permission: "blueprints:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"title":{"type":"string"},
			"description":{"type":"string"},
			"curriculumCode":{"type":"string","enum":["k13","merdeka","akm_numerasi","akm_literasi"]},
			"subjectCode":{"type":"string"},
			"gradeOrPhase":{"type":"string"},
			"blueprintType":{"type":"string","enum":["reguler","akm_literasi","akm_numerasi"]},
			"strictCoverage":{"type":"boolean"}
		},"required":["title","curriculumCode"]}`),
	}, a.capCreateBlueprintTemplate)

	reg.Register(Capability{
		Name: "add_blueprint_slot",
		Description: "Add slot to template. Position auto. cognitiveLevel: C1-C6. difficulty: mudah/sedang/sulit.",
		Permission: "blueprints:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"templateId":{"type":"string"},
			"position":{"type":"integer"},
			"competencyCode":{"type":"string"},
			"competencyDescription":{"type":"string"},
			"materi":{"type":"string"},
			"indikator":{"type":"string"},
			"cognitiveLevel":{"type":"string","enum":["C1","C2","C3","C4","C5","C6"]},
			"difficulty":{"type":"string","enum":["mudah","sedang","sulit"]},
			"questionType":{"type":"string","enum":["multiple_choice","true_false","short_answer","essay"]},
			"points":{"type":"number"},
			"akmKonten":{"type":"string"},
			"akmKonteks":{"type":"string"},
			"akmProses":{"type":"string"},
			"akmLevel":{"type":"integer","minimum":1,"maximum":5}
		},"required":["templateId"]}`),
	}, a.capAddBlueprintSlot)

	reg.Register(Capability{
		Name: "bulk_add_blueprint_slots",
		Description: "Bulk add slots to template (max 50).",
		Permission: "blueprints:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"templateId":{"type":"string"},
			"slots":{"type":"array","maxItems":50,"items":{"type":"object","properties":{
				"position":{"type":"integer"},
				"competencyCode":{"type":"string"},
				"competencyDescription":{"type":"string"},
				"materi":{"type":"string"},
				"indikator":{"type":"string"},
				"cognitiveLevel":{"type":"string"},
				"difficulty":{"type":"string"},
				"questionType":{"type":"string"},
				"points":{"type":"number"},
				"akmKonten":{"type":"string"},
				"akmKonteks":{"type":"string"},
				"akmProses":{"type":"string"},
				"akmLevel":{"type":"integer"}
			}}}
		},"required":["templateId","slots"]}`),
	}, a.capBulkAddBlueprintSlots)

	reg.Register(Capability{
		Name: "clone_blueprint_to_exam",
		Description: "Clone blueprint template to exam (snapshot). replace=true overrides existing.",
		Permission: "blueprints:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"templateId":{"type":"string"},
			"replace":{"type":"boolean","default":false}
		},"required":["examId","templateId"]}`),
	}, a.capCloneBlueprintToExamAI)

	reg.Register(Capability{
		Name: "generate_question_for_slot",
		Description: "Generate question matching slot spec. Slot defines type/cognitive/difficulty/points; you fill content/options/answer.",
		Permission: "exams:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"slotId":{"type":"string"},
			"content":{"type":"string","description":"Pertanyaan / batang soal"},
			"explanation":{"type":"string"},
			"correctAnswer":{"type":"string","description":"Untuk short_answer"},
			"options":{"type":"array","items":{"type":"object","properties":{
				"content":{"type":"string"},
				"isCorrect":{"type":"boolean"}
			}}}
		},"required":["slotId","content"]}`),
	}, a.capGenerateQuestionForSlot)

	reg.Register(Capability{
		Name: "apply_blueprint_analysis",
		Description: "Apply analyze_questions_to_blueprint result. Pass acceptedSlots + optional acceptedLinkIndices + mergeDecisions.",
		Permission: "blueprints:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"title":{"type":"string"},
			"curriculumCode":{"type":"string","enum":["k13","merdeka","akm_numerasi","akm_literasi"]},
			"blueprintType":{"type":"string","enum":["reguler","akm_literasi","akm_numerasi"]},
			"replace":{"type":"boolean","default":false},
			"acceptedSlotIndices":{"type":"array","items":{"type":"integer"}},
			"acceptedLinkIndices":{"type":"array","items":{"type":"integer"}},
			"mergeDecisions":{"type":"array","items":{"type":"object","properties":{
				"keepIndex":{"type":"integer"},
				"dropIndices":{"type":"array","items":{"type":"integer"}}
			}}},
			"acceptedSlots":{"type":"array","items":{"type":"object","properties":{
				"questionId":{"type":"string"},
				"competencyCode":{"type":"string"},
				"competencyDescription":{"type":"string"},
				"materi":{"type":"string"},
				"indikator":{"type":"string"},
				"cognitiveLevel":{"type":"string"},
				"difficulty":{"type":"string"},
				"questionType":{"type":"string"},
				"points":{"type":"number"},
				"akmKonten":{"type":"string"},
				"akmKonteks":{"type":"string"},
				"akmProses":{"type":"string"},
				"akmLevel":{"type":"integer"}
			},"required":["questionId"]}}
		},"required":["examId","title","curriculumCode","acceptedSlots"]}`),
	}, a.capApplyBlueprintAnalysis)

	// ───── Kisi-kisi toggle (ADR-0012) ─────
	reg.Register(Capability{
		Name: "set_uses_kisi_kisi",
		Description: "Toggle kisi-kisi enforcement on exam. enabled=true requires slot binding for every question.",
		Permission: "exams:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"enabled":{"type":"boolean"}
		},"required":["examId","enabled"]}`),
	}, a.capSetUsesKisiKisi)

	reg.Register(Capability{
		Name: "convert_questions_to_kisi_kisi",
		Description: "One-shot convert: analyze + auto-accept proposals ≥ minConfidence (default 0.7) + apply.",
		Permission: "blueprints:write",
		Risk:       "write",
		Domain:     "blueprints",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"title":{"type":"string","description":"Judul kisi-kisi yang dihasilkan. Default: 'Kisi-Kisi (auto-generated)'"},
			"curriculumCode":{"type":"string","enum":["k13","merdeka","akm_numerasi","akm_literasi"]},
			"blueprintType":{"type":"string","enum":["reguler","akm_literasi","akm_numerasi"]},
			"minConfidence":{"type":"number","default":0.7,"minimum":0.5,"maximum":1},
			"replace":{"type":"boolean","default":false}
		},"required":["examId","curriculumCode"]}`),
	}, a.capConvertQuestionsToKisiKisi)
}

// ─────────────────────────────────────────────────────────────────────
// Read capabilities
// ─────────────────────────────────────────────────────────────────────

func (a *App) capListBlueprintTemplates(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Search        string `json:"search"`
		Curriculum    string `json:"curriculum"`
		BlueprintType string `json:"blueprintType"`
		Status        string `json:"status"`
		Limit         int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &p)
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	q := `
		SELECT t.id::text, t.title, c.code, t.blueprint_type, t.status,
		       t.total_slots, t.total_points
		  FROM blueprint_templates t
		  JOIN curricula c ON c.id = t.curriculum_id
		 WHERE t.tenant_id = $1`
	qArgs := []any{tenantID}
	idx := 2
	add := func(clause string, val any) {
		q += " AND " + strings.Replace(clause, "$$", "$"+itoa(idx), 1)
		qArgs = append(qArgs, val)
		idx++
	}
	if p.Search != "" {
		add("t.title ILIKE $$", "%"+p.Search+"%")
	}
	if p.Curriculum != "" {
		add("c.code = $$", p.Curriculum)
	}
	if p.BlueprintType != "" {
		add("t.blueprint_type = $$", p.BlueprintType)
	}
	if p.Status != "" {
		add("t.status = $$", p.Status)
	}
	q += " ORDER BY t.updated_at DESC LIMIT $" + itoa(idx)
	qArgs = append(qArgs, p.Limit)

	rows, err := a.db.QueryContext(ctx, q, qArgs...)
	if err != nil {
		return "[]", nil
	}
	defer rows.Close()
	type Row struct {
		ID            string  `json:"id"`
		Title         string  `json:"title"`
		Curriculum    string  `json:"curriculum"`
		BlueprintType string  `json:"blueprintType"`
		Status        string  `json:"status"`
		TotalSlots    int     `json:"totalSlots"`
		TotalPoints   float64 `json:"totalPoints"`
	}
	var out []Row
	for rows.Next() {
		var r Row
		if rows.Scan(&r.ID, &r.Title, &r.Curriculum, &r.BlueprintType, &r.Status,
			&r.TotalSlots, &r.TotalPoints) == nil {
			out = append(out, r)
		}
	}
	if out == nil {
		out = []Row{}
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func (a *App) capGetBlueprintTemplate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		TemplateID string `json:"templateId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.TemplateID) {
		return errValidationFailed("templateId", "Valid templateId required"), nil
	}
	if denied := a.checkAIBlueprintAccess(ctx, tenantID, userID, p.TemplateID, ActionRead); denied != "" {
		return denied, nil
	}

	type Row struct {
		ID                    string   `json:"id"`
		Title                 string   `json:"title"`
		Description           *string  `json:"description,omitempty"`
		Curriculum            string   `json:"curriculum"`
		CompetencyLabel       string   `json:"competencyLabel"`
		BlueprintType         string   `json:"blueprintType"`
		Status                string   `json:"status"`
		TotalSlots            int      `json:"totalSlots"`
		TotalPoints           float64  `json:"totalPoints"`
		Slots                 []map[string]any `json:"slots"`
	}
	var t Row
	err := a.db.QueryRowContext(ctx, `
		SELECT t.id::text, t.title, t.description, c.code, c.competency_label,
		       t.blueprint_type, t.status, t.total_slots, t.total_points
		  FROM blueprint_templates t
		  JOIN curricula c ON c.id = t.curriculum_id
		 WHERE t.id = $1 AND t.tenant_id = $2`,
		p.TemplateID, tenantID,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Curriculum, &t.CompetencyLabel,
		&t.BlueprintType, &t.Status, &t.TotalSlots, &t.TotalPoints)
	if err != nil {
		return errEntityNotFound("blueprint_template", "id", p.TemplateID), nil
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT id::text, position, competency_code, competency_description,
		       materi, cognitive_level, difficulty, question_type, points
		  FROM blueprint_template_slots
		 WHERE template_id = $1 ORDER BY position`,
		p.TemplateID)
	if err == nil {
		defer rows.Close()
		t.Slots = []map[string]any{}
		for rows.Next() {
			var (
				id, comp, compDesc, materi, cog, diff, qtype sql.NullString
				position                                     int
				points                                       float64
				slotID                                       string
			)
			if rows.Scan(&slotID, &position, &comp, &compDesc, &materi,
				&cog, &diff, &qtype, &points) == nil {
				_ = id
				m := map[string]any{
					"id":       slotID,
					"position": position,
					"points":   points,
				}
				if comp.Valid {
					m["competencyCode"] = comp.String
				}
				if compDesc.Valid {
					m["competencyDescription"] = compDesc.String
				}
				if materi.Valid {
					m["materi"] = materi.String
				}
				if cog.Valid {
					m["cognitiveLevel"] = cog.String
				}
				if diff.Valid {
					m["difficulty"] = diff.String
				}
				if qtype.Valid {
					m["questionType"] = qtype.String
				}
				t.Slots = append(t.Slots, m)
			}
		}
	}
	b, _ := json.Marshal(t)
	return string(b), nil
}

func (a *App) capGetExamBlueprintAI(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId required"), nil
	}
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionRead); denied != "" {
		return denied, nil
	}

	var (
		bpID, title, curriculum, blueprintType, status string
		totalSlots                                     int
		totalPoints                                    float64
		filledSlots                                    int
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT b.id::text, b.title, c.code, b.blueprint_type, b.status,
		       b.total_slots, b.total_points
		  FROM exam_blueprints b
		  JOIN curricula c ON c.id = b.curriculum_id
		 WHERE b.exam_id = $1 AND b.tenant_id = $2`,
		p.ExamID, tenantID,
	).Scan(&bpID, &title, &curriculum, &blueprintType, &status, &totalSlots, &totalPoints)
	if err != nil {
		return `{"hasBlueprint":false,"hint":"Exam ini belum punya blueprint. Bisa pakai clone_blueprint_to_exam dengan templateId yang sesuai, atau analyze_questions_to_blueprint jika sudah ada soal."}`, nil
	}

	_ = a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM exam_blueprint_slots s
		  JOIN exam_questions q ON q.blueprint_slot_id = s.id
		 WHERE s.exam_blueprint_id = $1`, bpID,
	).Scan(&filledSlots)
	coverage := 0.0
	if totalSlots > 0 {
		coverage = float64(filledSlots) / float64(totalSlots)
	}

	out := map[string]any{
		"hasBlueprint":  true,
		"id":            bpID,
		"title":         title,
		"curriculum":    curriculum,
		"blueprintType": blueprintType,
		"status":        status,
		"totalSlots":    totalSlots,
		"totalPoints":   totalPoints,
		"filledSlots":   filledSlots,
		"coverage":      coverage,
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func (a *App) capListExamBlueprintSlotsAI(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId required"), nil
	}
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionRead); denied != "" {
		return denied, nil
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT s.id::text, s.position, s.competency_code, s.competency_description,
		       s.materi, s.cognitive_level, s.difficulty, s.question_type, s.points,
		       (SELECT q.id::text FROM exam_questions q WHERE q.blueprint_slot_id = s.id) AS question_id
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE b.exam_id = $1 AND b.tenant_id = $2
		 ORDER BY s.position`,
		p.ExamID, tenantID)
	if err != nil {
		return "[]", nil
	}
	defer rows.Close()
	type Row struct {
		ID                    string  `json:"id"`
		Position              int     `json:"position"`
		CompetencyCode        string  `json:"competencyCode,omitempty"`
		CompetencyDescription string  `json:"competencyDescription,omitempty"`
		Materi                string  `json:"materi,omitempty"`
		CognitiveLevel        string  `json:"cognitiveLevel,omitempty"`
		Difficulty            string  `json:"difficulty,omitempty"`
		QuestionType          string  `json:"questionType,omitempty"`
		Points                float64 `json:"points"`
		Filled                bool    `json:"filled"`
		QuestionID            string  `json:"questionId,omitempty"`
	}
	var out []Row
	for rows.Next() {
		var r Row
		var comp, compDesc, materi, cog, diff, qtype, qid sql.NullString
		if rows.Scan(&r.ID, &r.Position, &comp, &compDesc, &materi, &cog, &diff,
			&qtype, &r.Points, &qid) == nil {
			if comp.Valid {
				r.CompetencyCode = comp.String
			}
			if compDesc.Valid {
				r.CompetencyDescription = compDesc.String
			}
			if materi.Valid {
				r.Materi = materi.String
			}
			if cog.Valid {
				r.CognitiveLevel = cog.String
			}
			if diff.Valid {
				r.Difficulty = diff.String
			}
			if qtype.Valid {
				r.QuestionType = qtype.String
			}
			if qid.Valid {
				r.QuestionID = qid.String
				r.Filled = true
			}
			out = append(out, r)
		}
	}
	if out == nil {
		out = []Row{}
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// capAnalyzeQuestionsToBlueprint is the FORWARD-READ side of the reverse
// flow. It returns proposed slots and proposed question→slot links
// based on heuristics over existing questions; no DB write happens.
// Confidence is computed from how many fields the question content
// gives us evidence for.
//
// Per ADR-0010 the response shape is:
//
//	{
//	  proposedSlots:       [...],
//	  proposedLinks:       [{questionIndex, slotIndex, confidence, reasoning}],
//	  distributionSummary: {byKD, byLevel, byDifficulty, byType},
//	  unlinkedQuestions:   [{index, content, reason, confidence}],
//	  questionLimit:       50,
//	  minConfidence:       0.5
//	}
//
// Questions whose inferred confidence is below minConfidence still
// produce a slot proposal (so the user can review the inference) but
// the slot is NOT linked back to the question — the question goes
// into unlinkedQuestions, and the user assigns manually post-apply.
//
// This stub-grade analyzer can be replaced with an LLM call later; the
// shape of the response is the contract apply_blueprint_analysis expects.
func (a *App) capAnalyzeQuestionsToBlueprint(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID        string  `json:"examId"`
		MinConfidence float64 `json:"minConfidence"`
		BatchSize     int     `json:"batchSize"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId required"), nil
	}
	// Per-ADR caps
	if p.MinConfidence <= 0 || p.MinConfidence > 1 {
		p.MinConfidence = 0.5
	}
	const questionLimit = 50
	if p.BatchSize <= 0 || p.BatchSize > questionLimit {
		p.BatchSize = questionLimit
	}
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionRead); denied != "" {
		return denied, nil
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT id::text, content, question_type, points
		  FROM exam_questions
		 WHERE exam_id = $1 AND tenant_id = $2
		 ORDER BY sort_order, created_at
		 LIMIT $3`,
		p.ExamID, tenantID, p.BatchSize)
	if err != nil {
		return errInternal("Could not load questions"), nil
	}
	defer rows.Close()

	type ProposedSlot struct {
		Position       int     `json:"position"`
		CompetencyCode string  `json:"competencyCode,omitempty"`
		Materi         string  `json:"materi,omitempty"`
		CognitiveLevel string  `json:"cognitiveLevel,omitempty"`
		Difficulty     string  `json:"difficulty,omitempty"`
		QuestionType   string  `json:"questionType"`
		Points         float64 `json:"points"`
	}
	type ProposedLink struct {
		QuestionIndex int     `json:"questionIndex"`
		SlotIndex     int     `json:"slotIndex"`
		QuestionID    string  `json:"questionId"`
		Confidence    float64 `json:"confidence"`
		Reasoning     string  `json:"reasoning"`
	}
	type Unlinked struct {
		Index      int     `json:"index"`
		QuestionID string  `json:"questionId"`
		Content    string  `json:"content"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}

	proposedSlots := []ProposedSlot{}
	proposedLinks := []ProposedLink{}
	unlinked := []Unlinked{}
	byKD := map[string]int{}
	byLevel := map[string]int{}
	byDifficulty := map[string]int{}
	byType := map[string]int{}

	idxQ := 0
	for rows.Next() {
		var (
			questionID, content, qtype string
			points                     float64
		)
		if rows.Scan(&questionID, &content, &qtype, &points) != nil {
			continue
		}

		lower := strings.ToLower(content)
		level, levelEvidence := inferCognitiveLevel(lower)
		diff, diffEvidence := inferDifficulty(content, qtype)

		conf := 0.3
		reasons := []string{}
		if level != "" {
			conf += 0.3
			reasons = append(reasons, levelEvidence)
		}
		if diff != "" {
			conf += 0.2
			reasons = append(reasons, diffEvidence)
		}
		if conf > 0.9 {
			conf = 0.9
		}
		reasoning := strings.Join(reasons, "; ")
		if reasoning == "" {
			reasoning = "Tidak ada sinyal kuat. User perlu set manual."
		}

		slot := ProposedSlot{
			Position:       idxQ,
			CognitiveLevel: level,
			Difficulty:     diff,
			QuestionType:   qtype,
			Points:         points,
		}
		proposedSlots = append(proposedSlots, slot)

		if conf >= p.MinConfidence {
			proposedLinks = append(proposedLinks, ProposedLink{
				QuestionIndex: idxQ,
				SlotIndex:     idxQ, // 1:1 in heuristic mode
				QuestionID:    questionID,
				Confidence:    conf,
				Reasoning:     reasoning,
			})
		} else {
			preview := content
			if len(preview) > 120 {
				preview = preview[:117] + "..."
			}
			unlinked = append(unlinked, Unlinked{
				Index:      idxQ,
				QuestionID: questionID,
				Content:    preview,
				Confidence: conf,
				Reason:     "low_confidence",
			})
		}

		// Distribution counts
		if level != "" {
			byLevel[level]++
		} else {
			byLevel["unknown"]++
		}
		if diff != "" {
			byDifficulty[diff]++
		} else {
			byDifficulty["unknown"]++
		}
		if qtype != "" {
			byType[qtype]++
		}
		byKD["unknown"]++ // heuristic analyzer cannot infer KD; user fills.

		idxQ++
	}

	out := map[string]any{
		"examId":        p.ExamID,
		"proposedSlots": proposedSlots,
		"proposedLinks": proposedLinks,
		"distributionSummary": map[string]any{
			"byKD":         byKD,
			"byLevel":      byLevel,
			"byDifficulty": byDifficulty,
			"byType":       byType,
		},
		"unlinkedQuestions": unlinked,
		"questionLimit":     questionLimit,
		"minConfidence":     p.MinConfidence,
		"batchSize":         p.BatchSize,
		"count":             len(proposedSlots),
		"analysisType":      "heuristic",
		"hint":              "Review proposedSlots + proposedLinks. Kirim acceptedSlotIndices/acceptedLinkIndices ke apply_blueprint_analysis. Soal di unlinkedQuestions perlu user assign manual setelah apply.",
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func inferCognitiveLevel(lower string) (string, string) {
	// C1 Recall, C2 Understand, C3 Apply, C4 Analyze, C5 Evaluate, C6 Create
	checks := []struct {
		Level    string
		Keywords []string
	}{
		{"C6", []string{"buatlah", "rancang", "ciptakan", "design", "create", "compose"}},
		{"C5", []string{"evaluasi", "kritik", "penilaian", "argumen", "evaluate", "justify"}},
		{"C4", []string{"analisis", "bandingkan", "bedakan", "klasifikasi", "analyze", "compare", "differentiate"}},
		{"C3", []string{"hitung", "tentukan", "terapkan", "selesaikan", "calculate", "apply", "solve"}},
		{"C2", []string{"jelaskan", "uraikan", "ringkaskan", "deskripsikan", "explain", "describe", "summarize"}},
		{"C1", []string{"sebutkan", "apa yang", "siapa", "definisi", "list", "name", "identify"}},
	}
	for _, c := range checks {
		for _, k := range c.Keywords {
			if strings.Contains(lower, k) {
				return c.Level, fmt.Sprintf("level=%s (kata kunci: %s)", c.Level, k)
			}
		}
	}
	return "", ""
}

func inferDifficulty(content, qtype string) (string, string) {
	// Simple proxy: length + multi-step indicators. AKM/essay tend harder.
	n := len(content)
	if qtype == "essay" {
		return "sulit", "essay → sulit"
	}
	hasMultiStep := strings.Count(strings.ToLower(content), "kemudian") +
		strings.Count(strings.ToLower(content), "lalu") +
		strings.Count(strings.ToLower(content), "berikutnya")
	if n > 250 || hasMultiStep >= 2 {
		return "sulit", "panjang/multi-step"
	}
	if n < 80 {
		return "mudah", "ringkas"
	}
	return "sedang", "panjang sedang"
}

// ─────────────────────────────────────────────────────────────────────
// Write capabilities — propose-confirm-execute
// ─────────────────────────────────────────────────────────────────────

func (a *App) capCreateBlueprintTemplate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Title          string `json:"title"`
		CurriculumCode string `json:"curriculumCode"`
		BlueprintType  string `json:"blueprintType"`
	}
	_ = json.Unmarshal(args, &p)
	p.Title = strings.TrimSpace(p.Title)
	p.CurriculumCode = strings.TrimSpace(p.CurriculumCode)
	if p.Title == "" {
		return errValidationFailed("title", "title is required"), nil
	}
	switch p.CurriculumCode {
	case "k13", "merdeka", "akm_numerasi", "akm_literasi":
	default:
		return errValidationFailed("curriculumCode", "Must be k13/merdeka/akm_numerasi/akm_literasi"), nil
	}

	// In-flight + committed dupe
	if a.pendingHasField(ctx, "create_blueprint_template", "title", p.Title) {
		return errDuplicateEntryWithRecovery(
			"blueprint_template", "title", p.Title,
			"Already proposed in current session. Pick a different title.",
		), nil
	}
	var exists bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM blueprint_templates WHERE tenant_id = $1 AND lower(title) = lower($2) AND status != 'archived')`,
		tenantID, p.Title,
	).Scan(&exists)
	if exists {
		return errDuplicateEntryWithRecovery(
			"blueprint_template", "title", p.Title,
			"Template dengan judul yang sama sudah ada. Pilih judul unik atau pakai yang existing.",
		), nil
	}

	confirm := fmt.Sprintf("Buat blueprint template: \"%s\" (curriculum=%s)", p.Title, p.CurriculumCode)
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_blueprint_template", args, confirm)
}

func (a *App) capAddBlueprintSlot(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		TemplateID     string `json:"templateId"`
		CompetencyCode string `json:"competencyCode"`
		Materi         string `json:"materi"`
		Indikator      string `json:"indikator"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.TemplateID) {
		return errValidationFailed("templateId", "Valid templateId required. Call list_blueprint_templates first."), nil
	}
	// Layered access: tenant admin / owner / editor on the template.
	if denied := a.checkAIBlueprintAccess(ctx, tenantID, userID, p.TemplateID, ActionWrite); denied != "" {
		return denied, nil
	}

	// Slot signature dupe guard (committed + in-flight).
	if dup := a.checkBlueprintSlotDuplicate(ctx, tenantID, p.TemplateID, p.CompetencyCode, p.Materi, p.Indikator); dup != "" {
		return dup, nil
	}

	tag := p.CompetencyCode
	if tag == "" && p.Materi != "" {
		tag = p.Materi
	}
	if tag == "" {
		tag = "(slot)"
	}
	confirm := fmt.Sprintf("Tambah slot ke blueprint: %s", tag)
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "add_blueprint_slot", args, confirm)
}

func (a *App) capBulkAddBlueprintSlots(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		TemplateID string `json:"templateId"`
		Slots      []struct {
			Position       *int   `json:"position"`
			CompetencyCode string `json:"competencyCode"`
			Materi         string `json:"materi"`
			Indikator      string `json:"indikator"`
		} `json:"slots"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.TemplateID) {
		return errValidationFailed("templateId", "Valid templateId required"), nil
	}
	if len(p.Slots) == 0 {
		return errValidationFailed("slots", "At least one slot is required"), nil
	}
	if len(p.Slots) > 50 {
		return errValidationFailed("slots", "Maximum 50 slots per batch"), nil
	}

	// Layered access: must have write on the template.
	if denied := a.checkAIBlueprintAccess(ctx, tenantID, userID, p.TemplateID, ActionWrite); denied != "" {
		return denied, nil
	}

	// In-batch position collision (only when explicitly set)
	seenPos := map[int]int{}
	for i, s := range p.Slots {
		if s.Position == nil {
			continue
		}
		if first, ok := seenPos[*s.Position]; ok {
			return errValidationFailed(
				fmt.Sprintf("slots[%d].position", i),
				fmt.Sprintf("Duplicate position %d (also at index %d). Remove explicit position to auto-assign.", *s.Position, first),
			), nil
		}
		seenPos[*s.Position] = i
	}

	// In-batch signature duplicates + committed/pending dedup.
	seenSig := map[string]int{}
	for i, s := range p.Slots {
		sig := slotSignature(s.CompetencyCode, s.Materi, s.Indikator)
		if sig == "||" {
			continue
		}
		if first, ok := seenSig[sig]; ok {
			return errValidationFailed(
				fmt.Sprintf("slots[%d]", i),
				fmt.Sprintf("Duplicate slot signature with index %d (competencyCode/materi/indikator identical). Make each slot unique.", first),
			), nil
		}
		seenSig[sig] = i
		if dup := a.checkBlueprintSlotDuplicate(ctx, tenantID, p.TemplateID, s.CompetencyCode, s.Materi, s.Indikator); dup != "" {
			return dup, nil
		}
	}

	confirm := fmt.Sprintf("Tambah %d slot ke blueprint template", len(p.Slots))
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "bulk_add_blueprint_slots", args, confirm)
}

func (a *App) capCloneBlueprintToExamAI(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID     string `json:"examId"`
		TemplateID string `json:"templateId"`
		Replace    bool   `json:"replace"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId required"), nil
	}
	if !isUUID(p.TemplateID) {
		return errValidationFailed("templateId", "Valid templateId required"), nil
	}

	// Layered access: must be able to write the exam (clone is a write
	// to exam_blueprints owned by the exam) AND read the source template.
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}
	if denied := a.checkAIBlueprintAccess(ctx, tenantID, userID, p.TemplateID, ActionRead); denied != "" {
		return denied, nil
	}

	// Confirm exam exists, is draft, and (if has blueprint) replace=true
	var examStatus string
	if err := a.db.QueryRowContext(ctx,
		`SELECT status FROM exams WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	).Scan(&examStatus); err != nil {
		return errEntityNotFound("exam", "id", p.ExamID), nil
	}
	if examStatus != "draft" {
		return errInvalidState("Exam tidak dalam status draft, tidak bisa di-apply blueprint"), nil
	}
	var hasBP bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM exam_blueprints WHERE exam_id = $1)`,
		p.ExamID,
	).Scan(&hasBP)
	if hasBP && !p.Replace {
		return errInvalidState("Exam sudah punya blueprint. Pass replace=true untuk overwrite (akan menghapus blueprint lama)."), nil
	}

	// Verify template
	var tplTitle string
	if err := a.db.QueryRowContext(ctx,
		`SELECT title FROM blueprint_templates WHERE id = $1 AND tenant_id = $2`,
		p.TemplateID, tenantID,
	).Scan(&tplTitle); err != nil {
		return errEntityNotFound("blueprint_template", "id", p.TemplateID), nil
	}

	confirm := fmt.Sprintf("Apply blueprint \"%s\" ke exam (snapshot)", tplTitle)
	if hasBP && p.Replace {
		confirm += " — REPLACE blueprint existing"
	}
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "clone_blueprint_to_exam", args, confirm)
}

func (a *App) capGenerateQuestionForSlot(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		SlotID  string           `json:"slotId"`
		Content string           `json:"content"`
		Options []questionOption `json:"options"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.SlotID) {
		return errValidationFailed("slotId", "Valid slotId required. Call list_blueprint_slots first."), nil
	}
	p.Content = strings.TrimSpace(p.Content)
	if p.Content == "" {
		return errValidationFailed("content", "content (pertanyaan) wajib"), nil
	}

	// Resolve slot → exam, then enforce write access on the parent exam.
	examID := a.resolveSlotParentExam(ctx, tenantID, p.SlotID)
	if examID == "" {
		return errEntityNotFound("blueprint_slot", "id", p.SlotID), nil
	}
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, examID, ActionWrite); denied != "" {
		return denied, nil
	}

	// Slot detail (question_type + filled state). Already tenant-scoped
	// via resolveSlotParentExam, so this can be a plain lookup.
	var (
		qtype  string
		filled bool
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT COALESCE(s.question_type, 'multiple_choice'),
		       EXISTS(SELECT 1 FROM exam_questions q WHERE q.blueprint_slot_id = s.id)
		  FROM exam_blueprint_slots s
		 WHERE s.id = $1`,
		p.SlotID,
	).Scan(&qtype, &filled)
	if err != nil {
		return errEntityNotFound("blueprint_slot", "id", p.SlotID), nil
	}
	if filled {
		return errInvalidState("Slot ini sudah punya soal. Hapus soal existing dulu atau pilih slot lain yang masih kosong."), nil
	}

	// Validate question payload against the slot's questionType
	if errs := validateQuestionPayload(qtype, p.Content, "", p.Options); len(errs) > 0 {
		var firstField, firstMsg string
		for k, v := range errs {
			firstField, firstMsg = k, v
			break
		}
		return errValidationFailed(firstField, firstMsg), nil
	}

	// Dedup against committed questions in the same exam
	if dup := a.checkQuestionDuplicate(ctx, tenantID, examID, p.Content); dup != "" {
		return dup, nil
	}

	preview := p.Content
	if len(preview) > 60 {
		preview = preview[:57] + "..."
	}
	confirm := fmt.Sprintf("Buat soal (%s) terhubung ke slot: \"%s\"", qtype, preview)
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "generate_question_for_slot", args, confirm)
}

func (a *App) capApplyBlueprintAnalysis(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID         string `json:"examId"`
		Title          string `json:"title"`
		CurriculumCode string `json:"curriculumCode"`
		BlueprintType  string `json:"blueprintType"`
		Replace        bool   `json:"replace"`
		AcceptedSlots  []struct {
			QuestionID string `json:"questionId"`
		} `json:"acceptedSlots"`
		AcceptedSlotIndices []int `json:"acceptedSlotIndices"`
		AcceptedLinkIndices []int `json:"acceptedLinkIndices"`
		MergeDecisions      []struct {
			KeepIndex   int   `json:"keepIndex"`
			DropIndices []int `json:"dropIndices"`
		} `json:"mergeDecisions"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId required"), nil
	}
	if strings.TrimSpace(p.Title) == "" {
		return errValidationFailed("title", "title is required"), nil
	}
	switch p.CurriculumCode {
	case "k13", "merdeka", "akm_numerasi", "akm_literasi":
	default:
		return errValidationFailed("curriculumCode", "Must be k13/merdeka/akm_numerasi/akm_literasi"), nil
	}
	if len(p.AcceptedSlots) == 0 {
		return errValidationFailed("acceptedSlots", "At least one accepted slot is required"), nil
	}

	// Layered access: must be able to write the parent exam.
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}

	// Validate every accepted slot has a real questionId in this exam
	for i, s := range p.AcceptedSlots {
		if !isUUID(s.QuestionID) {
			return errValidationFailed(
				fmt.Sprintf("acceptedSlots[%d].questionId", i),
				"Valid questionId required",
			), nil
		}
		var ok bool
		_ = a.db.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM exam_questions WHERE id = $1 AND exam_id = $2 AND tenant_id = $3)`,
			s.QuestionID, p.ExamID, tenantID,
		).Scan(&ok)
		if !ok {
			return errValidationFailed(
				fmt.Sprintf("acceptedSlots[%d].questionId", i),
				"Question does not belong to this exam",
			), nil
		}
	}

	// Optional index-based filters per ADR-0010 (all index lists are
	// applied at execute time; here we just validate they are within
	// bounds so the bot fails fast).
	for i, idx := range p.AcceptedSlotIndices {
		if idx < 0 || idx >= len(p.AcceptedSlots) {
			return errValidationFailed(
				fmt.Sprintf("acceptedSlotIndices[%d]", i),
				fmt.Sprintf("Index %d out of range (acceptedSlots length %d)", idx, len(p.AcceptedSlots)),
			), nil
		}
	}
	for i, idx := range p.AcceptedLinkIndices {
		if idx < 0 || idx >= len(p.AcceptedSlots) {
			return errValidationFailed(
				fmt.Sprintf("acceptedLinkIndices[%d]", i),
				fmt.Sprintf("Index %d out of range (acceptedSlots length %d)", idx, len(p.AcceptedSlots)),
			), nil
		}
	}
	for i, m := range p.MergeDecisions {
		if m.KeepIndex < 0 || m.KeepIndex >= len(p.AcceptedSlots) {
			return errValidationFailed(
				fmt.Sprintf("mergeDecisions[%d].keepIndex", i),
				fmt.Sprintf("keepIndex %d out of range", m.KeepIndex),
			), nil
		}
		for j, drop := range m.DropIndices {
			if drop < 0 || drop >= len(p.AcceptedSlots) {
				return errValidationFailed(
					fmt.Sprintf("mergeDecisions[%d].dropIndices[%d]", i, j),
					fmt.Sprintf("dropIndex %d out of range", drop),
				), nil
			}
			if drop == m.KeepIndex {
				return errValidationFailed(
					fmt.Sprintf("mergeDecisions[%d].dropIndices[%d]", i, j),
					"dropIndex cannot equal keepIndex",
				), nil
			}
		}
	}

	// Check exam status + replace semantics
	var examStatus string
	if err := a.db.QueryRowContext(ctx,
		`SELECT status FROM exams WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	).Scan(&examStatus); err != nil {
		return errEntityNotFound("exam", "id", p.ExamID), nil
	}
	if examStatus != "draft" {
		return errInvalidState("Exam tidak dalam status draft"), nil
	}
	var hasBP bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM exam_blueprints WHERE exam_id = $1)`,
		p.ExamID,
	).Scan(&hasBP)
	if hasBP && !p.Replace {
		return errInvalidState("Exam sudah punya blueprint. Pass replace=true."), nil
	}

	confirm := fmt.Sprintf("Apply blueprint dari analisis: \"%s\" — %d slot, terhubung ke %d soal existing",
		p.Title, len(p.AcceptedSlots), len(p.AcceptedSlots))
	if hasBP && p.Replace {
		confirm += " (REPLACE blueprint existing)"
	}
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "apply_blueprint_analysis", args, confirm)
}

// ─────────────────────────────────────────────────────────────────────
// Executors — called from executeConfirmedAction switch
// ─────────────────────────────────────────────────────────────────────

func (a *App) execCreateBlueprintTemplate(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Title          string `json:"title"`
		Description    string `json:"description"`
		CurriculumCode string `json:"curriculumCode"`
		SubjectCode    string `json:"subjectCode"`
		GradeOrPhase   string `json:"gradeOrPhase"`
		BlueprintType  string `json:"blueprintType"`
		StrictCoverage *bool  `json:"strictCoverage"`
	}
	_ = json.Unmarshal(args, &p)
	if p.BlueprintType == "" {
		p.BlueprintType = "reguler"
	}
	strict := false
	if strings.HasPrefix(p.BlueprintType, "akm_") {
		strict = true
	}
	if p.StrictCoverage != nil {
		strict = *p.StrictCoverage
	}

	var curriculumID string
	if err := a.db.QueryRowContext(ctx,
		`SELECT id::text FROM curricula WHERE code = $1`, p.CurriculumCode,
	).Scan(&curriculumID); err != nil {
		return "", fmt.Errorf("unknown curriculumCode: %s", p.CurriculumCode)
	}

	var id string
	err := a.db.QueryRowContext(ctx, `
		INSERT INTO blueprint_templates (
		    tenant_id, owner_user_id, title, description,
		    curriculum_id, subject_code, grade_or_phase, blueprint_type,
		    strict_coverage, status, version
		) VALUES ($1, $2, $3, NULLIF($4,''),
		          $5, NULLIF($6,''), NULLIF($7,''), $8,
		          $9, 'draft', 1)
		RETURNING id::text`,
		tenantID, userID, p.Title, p.Description,
		curriculumID, p.SubjectCode, p.GradeOrPhase, p.BlueprintType, strict,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "blueprint_templates.create", "blueprint_template", id)
	return fmt.Sprintf(`{"success":true,"message":"Blueprint template \"%s\" berhasil dibuat","templateId":"%s"}`, p.Title, id), nil
}

func (a *App) execAddBlueprintSlot(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		TemplateID     string `json:"templateId"`
		CompetencyCode string `json:"competencyCode"`
		Materi         string `json:"materi"`
		Indikator      string `json:"indikator"`
	}
	_ = json.Unmarshal(args, &p)

	// Execute-layer access check + dupe re-check (window between
	// propose and confirm could have invited a colliding row).
	if denied := a.checkAIBlueprintAccess(ctx, tenantID, userID, p.TemplateID, ActionWrite); denied != "" {
		return denied, nil
	}
	if dup := a.checkBlueprintSlotDuplicate(ctx, tenantID, p.TemplateID, p.CompetencyCode, p.Materi, p.Indikator); dup != "" {
		return dup, nil
	}

	// Reuse buildSlotInsertSQL but it lives in blueprint_slots.go which is
	// the http handler file. To stay decoupled and respect the existing
	// architectural seam, the AI executor calls a small wrapper that mirrors
	// the same INSERT, supporting position auto-assign.
	id, position, err := a.aiInsertTemplateSlot(ctx, p.TemplateID, args)
	if err != nil {
		return "", err
	}
	// Refresh totals
	_, _ = a.db.ExecContext(ctx, `
		UPDATE blueprint_templates SET
		    total_slots  = (SELECT COUNT(*) FROM blueprint_template_slots WHERE template_id = $1),
		    total_points = (SELECT COALESCE(SUM(points), 0) FROM blueprint_template_slots WHERE template_id = $1),
		    updated_at = now()
		 WHERE id = $1`, p.TemplateID)
	a.auditAI(ctx, tenantID, userID, "blueprint_templates.slot_added", "blueprint_template_slot", id)
	return fmt.Sprintf(`{"success":true,"message":"Slot ditambahkan di posisi %d","slotId":"%s","position":%d}`, position, id, position), nil
}

func (a *App) execBulkAddBlueprintSlots(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		TemplateID string            `json:"templateId"`
		Slots      []json.RawMessage `json:"slots"`
	}
	_ = json.Unmarshal(args, &p)

	// Execute-layer access check
	if denied := a.checkAIBlueprintAccess(ctx, tenantID, userID, p.TemplateID, ActionWrite); denied != "" {
		return denied, nil
	}

	// Re-check signature dupes against committed rows in case another
	// session inserted matching slots between propose and confirm.
	for i, raw := range p.Slots {
		var s struct {
			CompetencyCode string `json:"competencyCode"`
			Materi         string `json:"materi"`
			Indikator      string `json:"indikator"`
		}
		_ = json.Unmarshal(raw, &s)
		if dup := a.checkBlueprintSlotDuplicate(ctx, tenantID, p.TemplateID, s.CompetencyCode, s.Materi, s.Indikator); dup != "" {
			_ = i
			return dup, nil
		}
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var nextPos int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) + 1 FROM blueprint_template_slots WHERE template_id = $1`,
		p.TemplateID,
	).Scan(&nextPos); err != nil {
		return "", err
	}

	ids := make([]string, 0, len(p.Slots))
	for _, raw := range p.Slots {
		var sm map[string]any
		_ = json.Unmarshal(raw, &sm)
		// Inject auto position if not set
		if _, ok := sm["position"]; !ok {
			sm["position"] = nextPos
			nextPos++
		} else if pf, ok := sm["position"].(float64); ok {
			if int(pf)+1 > nextPos {
				nextPos = int(pf) + 1
			}
		}
		merged, _ := json.Marshal(sm)
		id, _, err := aiInsertTemplateSlotTx(ctx, tx, p.TemplateID, merged)
		if err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	// Refresh totals
	_, _ = a.db.ExecContext(ctx, `
		UPDATE blueprint_templates SET
		    total_slots  = (SELECT COUNT(*) FROM blueprint_template_slots WHERE template_id = $1),
		    total_points = (SELECT COALESCE(SUM(points), 0) FROM blueprint_template_slots WHERE template_id = $1),
		    updated_at = now()
		 WHERE id = $1`, p.TemplateID)
	for _, id := range ids {
		a.auditAI(ctx, tenantID, userID, "blueprint_templates.slot_added", "blueprint_template_slot", id)
	}
	a.auditAI(ctx, tenantID, userID, "blueprint_templates.slots_bulk_added", "blueprint_template", p.TemplateID)
	return fmt.Sprintf(`{"success":true,"message":"%d slot berhasil ditambahkan","count":%d,"slotIds":%s}`,
		len(ids), len(ids), mustJSON(ids)), nil
}

func (a *App) execCloneBlueprintToExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID     string `json:"examId"`
		TemplateID string `json:"templateId"`
		Replace    bool   `json:"replace"`
	}
	_ = json.Unmarshal(args, &p)

	// Execute-layer access checks
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}
	if denied := a.checkAIBlueprintAccess(ctx, tenantID, userID, p.TemplateID, ActionRead); denied != "" {
		return denied, nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if p.Replace {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM exam_blueprints WHERE exam_id = $1`, p.ExamID,
		); err != nil {
			return "", err
		}
	}

	var (
		title, description, blueprintType                                                          string
		curriculumID                                                                               string
		subjectCode, gradeOrPhase                                                                  sql.NullString
		strictCoverage                                                                             bool
		version                                                                                    int
	)
	err = tx.QueryRowContext(ctx, `
		SELECT title, COALESCE(description, ''), curriculum_id::text,
		       subject_code, grade_or_phase,
		       blueprint_type, strict_coverage, version
		  FROM blueprint_templates
		 WHERE id = $1 AND tenant_id = $2`,
		p.TemplateID, tenantID,
	).Scan(&title, &description, &curriculumID, &subjectCode, &gradeOrPhase,
		&blueprintType, &strictCoverage, &version)
	if err != nil {
		return "", err
	}

	var newBPID string
	subjStr := ""
	if subjectCode.Valid {
		subjStr = subjectCode.String
	}
	gradeStr := ""
	if gradeOrPhase.Valid {
		gradeStr = gradeOrPhase.String
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO exam_blueprints (
		    tenant_id, exam_id, source_template_id, source_template_version,
		    created_via, title, description, curriculum_id,
		    subject_code, grade_or_phase, blueprint_type,
		    strict_coverage, status
		) VALUES ($1, $2, $3, $4, 'template_clone', $5, NULLIF($6,''),
		          $7, NULLIF($8,''), NULLIF($9,''), $10, $11, 'draft')
		RETURNING id::text`,
		tenantID, p.ExamID, p.TemplateID, version,
		title, description, curriculumID,
		subjStr, gradeStr, blueprintType, strictCoverage,
	).Scan(&newBPID)
	if err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO exam_blueprint_slots (
		    exam_blueprint_id, position,
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
		  FROM blueprint_template_slots
		 WHERE template_id = $2
		 ORDER BY position`,
		newBPID, p.TemplateID,
	); err != nil {
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE exam_blueprints SET
		    total_slots  = (SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1),
		    total_points = (SELECT COALESCE(SUM(points), 0) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1)
		 WHERE id = $1`, newBPID,
	); err != nil {
		return "", err
	}

	// Promote the kisi-kisi toggle (ADR-0012). Cloning a blueprint always
	// flips uses_kisi_kisi=true; AKM detection now reads from
	// blueprint_type at the consumer site, so there's no exam-level AKM
	// column to set.
	if _, err := tx.ExecContext(ctx,
		`UPDATE exams SET uses_kisi_kisi = true, updated_at = now()
		  WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	); err != nil {
		return "", err
	}

	// AKM auto-grouping (best-effort). Pre-create exam_question_groups
	// for each unique stimulus referenced by the just-cloned slots.
	isAKM := blueprintType == "akm_literasi" || blueprintType == "akm_numerasi"
	if isAKM {
		if err := a.autoCreateAkmGroups(ctx, tx, tenantID, p.ExamID, newBPID); err != nil {
			a.logger.Warn("akm auto-grouping failed (non-fatal)", "error", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "exam_blueprints.clone", "exam_blueprint", newBPID)
	return fmt.Sprintf(`{"success":true,"message":"Blueprint berhasil di-clone ke exam","blueprintId":"%s","createdVia":"template_clone","sourceTemplateVersion":%d,"usesKisiKisi":true,"blueprintType":%q}`,
		newBPID, version, blueprintType), nil
}

func (a *App) execGenerateQuestionForSlot(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		SlotID        string           `json:"slotId"`
		Content       string           `json:"content"`
		Explanation   string           `json:"explanation"`
		CorrectAnswer string           `json:"correctAnswer"`
		Options       []questionOption `json:"options"`
	}
	_ = json.Unmarshal(args, &p)

	// Resolve slot → exam first so we can run the layered access check
	// before any DB writes.
	examID := a.resolveSlotParentExam(ctx, tenantID, p.SlotID)
	if examID == "" {
		return errEntityNotFound("blueprint_slot", "id", p.SlotID), nil
	}
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, examID, ActionWrite); denied != "" {
		return denied, nil
	}

	// Load slot details (question_type + points). Tenant scoping via
	// resolveSlotParentExam already done.
	var (
		qtype  string
		points float64
	)
	if err := a.db.QueryRowContext(ctx, `
		SELECT COALESCE(s.question_type, 'multiple_choice'), s.points
		  FROM exam_blueprint_slots s
		 WHERE s.id = $1`,
		p.SlotID,
	).Scan(&qtype, &points); err != nil {
		return "", err
	}

	// Build the args payload for insertQuestionWithOptions, injecting slot
	// metadata as the source of truth.
	payload := map[string]any{
		"examId":          examID,
		"questionType":    qtype,
		"content":         p.Content,
		"explanation":     p.Explanation,
		"correctAnswer":   p.CorrectAnswer,
		"points":          points,
		"options":         p.Options,
	}
	merged, _ := json.Marshal(payload)

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	id, err := a.insertQuestionWithOptionsTx(ctx, tx, tenantID, userID, merged)
	if err != nil {
		return "", err
	}
	// Link the new question to the slot
	if _, err := tx.ExecContext(ctx,
		`UPDATE exam_questions SET blueprint_slot_id = $1 WHERE id = $2`,
		p.SlotID, id,
	); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "questions.create", "exam_question", id)
	a.auditAI(ctx, tenantID, userID, "exam_blueprints.slot_filled", "exam_blueprint_slot", p.SlotID)
	return fmt.Sprintf(`{"success":true,"message":"Soal berhasil dibuat dan terhubung ke slot","questionId":"%s","slotId":"%s"}`, id, p.SlotID), nil
}

func (a *App) execApplyBlueprintAnalysis(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID         string `json:"examId"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		CurriculumCode string `json:"curriculumCode"`
		BlueprintType  string `json:"blueprintType"`
		Replace        bool   `json:"replace"`
		AcceptedSlots  []struct {
			QuestionID            string   `json:"questionId"`
			CompetencyCode        string   `json:"competencyCode"`
			CompetencyDescription string   `json:"competencyDescription"`
			Materi                string   `json:"materi"`
			Indikator             string   `json:"indikator"`
			CognitiveLevel        string   `json:"cognitiveLevel"`
			Difficulty            string   `json:"difficulty"`
			QuestionType          string   `json:"questionType"`
			Points                *float64 `json:"points"`
			AkmKonten             string   `json:"akmKonten"`
			AkmKonteks            string   `json:"akmKonteks"`
			AkmProses             string   `json:"akmProses"`
			AkmLevel              *int     `json:"akmLevel"`
		} `json:"acceptedSlots"`
	}
	_ = json.Unmarshal(args, &p)
	if p.BlueprintType == "" {
		if strings.HasPrefix(p.CurriculumCode, "akm_") {
			p.BlueprintType = p.CurriculumCode
		} else {
			p.BlueprintType = "reguler"
		}
	}

	// Execute-layer access check
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if p.Replace {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM exam_blueprints WHERE exam_id = $1`, p.ExamID,
		); err != nil {
			return "", err
		}
	}

	var curriculumID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id::text FROM curricula WHERE code = $1`, p.CurriculumCode,
	).Scan(&curriculumID); err != nil {
		return "", err
	}

	strict := false
	if strings.HasPrefix(p.BlueprintType, "akm_") {
		strict = true
	}

	var bpID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO exam_blueprints (
		    tenant_id, exam_id, source_template_id, source_template_version,
		    created_via, title, description, curriculum_id,
		    blueprint_type, strict_coverage, status
		) VALUES ($1, $2, NULL, NULL, 'reverse_analysis', $3, NULLIF($4,''),
		          $5, $6, $7, 'draft')
		RETURNING id::text`,
		tenantID, p.ExamID, p.Title, p.Description,
		curriculumID, p.BlueprintType, strict,
	).Scan(&bpID)
	if err != nil {
		return "", err
	}

	for i, s := range p.AcceptedSlots {
		points := 1.0
		if s.Points != nil {
			points = *s.Points
		}
		var slotID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO exam_blueprint_slots (
			    exam_blueprint_id, position,
			    competency_code, competency_description, materi, indikator,
			    cognitive_level, difficulty, question_type, points,
			    akm_konten, akm_konteks, akm_proses, akm_level
			) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''),
			          NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), $10,
			          NULLIF($11,''), NULLIF($12,''), NULLIF($13,''), $14)
			RETURNING id::text`,
			bpID, i, s.CompetencyCode, s.CompetencyDescription, s.Materi, s.Indikator,
			s.CognitiveLevel, s.Difficulty, s.QuestionType, points,
			s.AkmKonten, s.AkmKonteks, s.AkmProses, s.AkmLevel,
		).Scan(&slotID)
		if err != nil {
			return "", err
		}
		// Link the existing question to its new slot. Clear any prior link
		// to avoid dangling references.
		if _, err := tx.ExecContext(ctx,
			`UPDATE exam_questions SET blueprint_slot_id = $1 WHERE id = $2 AND tenant_id = $3`,
			slotID, s.QuestionID, tenantID,
		); err != nil {
			return "", err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE exam_blueprints SET
		    total_slots  = (SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1),
		    total_points = (SELECT COALESCE(SUM(points), 0) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1)
		 WHERE id = $1`, bpID,
	); err != nil {
		return "", err
	}

	// Reverse-flow apply also flips uses_kisi_kisi=true (ADR-0012). AKM
	// detection follows from blueprint_type, no separate exam-level
	// signal needed.
	if _, err := tx.ExecContext(ctx,
		`UPDATE exams SET uses_kisi_kisi = true, updated_at = now()
		  WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "exam_blueprints.apply_analysis", "exam_blueprint", bpID)
	return fmt.Sprintf(`{"success":true,"message":"Blueprint berhasil di-apply dari analisis (%d slot)","blueprintId":"%s","createdVia":"reverse_analysis"}`,
		len(p.AcceptedSlots), bpID), nil
}

// ─────────────────────────────────────────────────────────────────────
// Internal slot-insert helpers (kept here so AI executors don't reach
// into HTTP handler internals).
// ─────────────────────────────────────────────────────────────────────

func (a *App) aiInsertTemplateSlot(ctx context.Context, templateID string, args json.RawMessage) (string, int, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()
	id, pos, err := aiInsertTemplateSlotTx(ctx, tx, templateID, args)
	if err != nil {
		return "", 0, err
	}
	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return id, pos, nil
}

func aiInsertTemplateSlotTx(ctx context.Context, tx *sql.Tx, templateID string, args json.RawMessage) (string, int, error) {
	var p struct {
		Position              *int     `json:"position"`
		CompetencyCode        string   `json:"competencyCode"`
		CompetencyDescription string   `json:"competencyDescription"`
		Materi                string   `json:"materi"`
		Indikator             string   `json:"indikator"`
		CognitiveLevel        string   `json:"cognitiveLevel"`
		Difficulty            string   `json:"difficulty"`
		QuestionType          string   `json:"questionType"`
		Points                *float64 `json:"points"`
		AkmKonten             string   `json:"akmKonten"`
		AkmKonteks            string   `json:"akmKonteks"`
		AkmProses             string   `json:"akmProses"`
		AkmLevel              *int     `json:"akmLevel"`
	}
	_ = json.Unmarshal(args, &p)

	pos := 0
	if p.Position != nil {
		pos = *p.Position
	} else {
		_ = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(position), -1) + 1 FROM blueprint_template_slots WHERE template_id = $1`,
			templateID,
		).Scan(&pos)
	}
	points := 1.0
	if p.Points != nil {
		points = *p.Points
	}

	var id string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO blueprint_template_slots (
		    template_id, position, points,
		    competency_code, competency_description, materi, indikator,
		    cognitive_level, difficulty, question_type,
		    akm_konten, akm_konteks, akm_proses, akm_level
		) VALUES (
		    $1, $2, $3,
		    NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), NULLIF($7,''),
		    NULLIF($8,''), NULLIF($9,''), NULLIF($10,''),
		    NULLIF($11,''), NULLIF($12,''), NULLIF($13,''), $14
		) RETURNING id::text`,
		templateID, pos, points,
		p.CompetencyCode, p.CompetencyDescription, p.Materi, p.Indikator,
		p.CognitiveLevel, p.Difficulty, p.QuestionType,
		p.AkmKonten, p.AkmKonteks, p.AkmProses, p.AkmLevel,
	).Scan(&id)
	if err != nil {
		return "", 0, err
	}
	return id, pos, nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ────────────────────────────────────────────────────────────────────
// Kisi-kisi toggle capabilities (ADR-0012)
// ────────────────────────────────────────────────────────────────────

func (a *App) capSetUsesKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID  string `json:"examId"`
		Enabled bool   `json:"enabled"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId required"), nil
	}
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}
	var status string
	var current bool
	if err := a.db.QueryRowContext(ctx,
		`SELECT status, uses_kisi_kisi FROM exams WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	).Scan(&status, &current); err != nil {
		return errEntityNotFound("exam", "id", p.ExamID), nil
	}
	if p.Enabled == current {
		return fmt.Sprintf(`{"success":true,"message":"uses_kisi_kisi sudah %v, tidak ada perubahan","usesKisiKisi":%v}`, current, current), nil
	}
	auth := a.loadAIAuth(ctx, tenantID, userID)
	if status != "draft" && !isAdminOverrideRole(auth) {
		return errInvalidState("Toggle hanya bisa diubah saat exam draft"), nil
	}
	label := "matikan"
	if p.Enabled {
		label = "aktifkan"
	}
	confirm := fmt.Sprintf("%s kisi-kisi pada exam ini", label)
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "set_uses_kisi_kisi", args, confirm)
}

func (a *App) execSetUsesKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID  string `json:"examId"`
		Enabled bool   `json:"enabled"`
	}
	_ = json.Unmarshal(args, &p)
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}
	var status string
	var current bool
	if err := a.db.QueryRowContext(ctx,
		`SELECT status, uses_kisi_kisi FROM exams WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	).Scan(&status, &current); err != nil {
		return "", err
	}
	auth := a.loadAIAuth(ctx, tenantID, userID)
	if status != "draft" && !isAdminOverrideRole(auth) {
		return errInvalidState("Toggle hanya bisa diubah saat exam draft"), nil
	}
	if _, err := a.db.ExecContext(ctx,
		`UPDATE exams SET uses_kisi_kisi = $1, updated_at = now()
		  WHERE id = $2 AND tenant_id = $3`,
		p.Enabled, p.ExamID, tenantID,
	); err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "exams.update", "exam", p.ExamID)
	return fmt.Sprintf(`{"success":true,"message":"Kisi-kisi enforcement di-set ke %v","examId":%q,"usesKisiKisi":%v,"previousValue":%v}`,
		p.Enabled, p.ExamID, p.Enabled, current), nil
}

// capConvertQuestionsToKisiKisi is the high-level wrapper for the
// reverse-flow shortcut on the slot-first canvas. The frontend's
// "Generate kisi-kisi from existing questions" button calls this.
//
// It is propose-first: validates inputs, runs the heuristic analyzer
// (read-only) to produce a proposal preview, and creates a single
// pending action wrapping the analysis + apply call. On confirm the
// executor runs apply_blueprint_analysis on the auto-accepted subset.
func (a *App) capConvertQuestionsToKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID         string  `json:"examId"`
		Title          string  `json:"title"`
		CurriculumCode string  `json:"curriculumCode"`
		BlueprintType  string  `json:"blueprintType"`
		MinConfidence  float64 `json:"minConfidence"`
		Replace        bool    `json:"replace"`
	}
	_ = json.Unmarshal(args, &p)
	if !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId required"), nil
	}
	switch p.CurriculumCode {
	case "k13", "merdeka", "akm_numerasi", "akm_literasi":
	default:
		return errValidationFailed("curriculumCode", "Must be k13/merdeka/akm_numerasi/akm_literasi"), nil
	}
	if p.MinConfidence <= 0 || p.MinConfidence > 1 {
		p.MinConfidence = 0.7
	}
	if p.Title == "" {
		p.Title = "Kisi-Kisi (auto-generated)"
	}
	if p.BlueprintType == "" {
		if strings.HasPrefix(p.CurriculumCode, "akm_") {
			p.BlueprintType = p.CurriculumCode
		} else {
			p.BlueprintType = "reguler"
		}
	}

	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}

	// Confirm exam is draft + replace semantics
	var examStatus string
	if err := a.db.QueryRowContext(ctx,
		`SELECT status FROM exams WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	).Scan(&examStatus); err != nil {
		return errEntityNotFound("exam", "id", p.ExamID), nil
	}
	if examStatus != "draft" {
		return errInvalidState("Exam tidak dalam status draft"), nil
	}
	var hasBP bool
	_ = a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM exam_blueprints WHERE exam_id = $1)`,
		p.ExamID,
	).Scan(&hasBP)
	if hasBP && !p.Replace {
		return errInvalidState("Exam sudah punya blueprint. Pass replace=true."), nil
	}

	// Run the read-only analyzer to count what we'd create. We don't
	// persist the proposal payload; the executor re-runs analysis to
	// produce a fresh, current view of the exam at confirm time.
	analyzeArgs, _ := json.Marshal(map[string]any{
		"examId":        p.ExamID,
		"minConfidence": p.MinConfidence,
		"batchSize":     50,
	})
	analysisJSON, _ := a.capAnalyzeQuestionsToBlueprint(ctx, tenantID, userID, analyzeArgs)
	var analysis struct {
		ProposedSlots     []map[string]any `json:"proposedSlots"`
		ProposedLinks     []map[string]any `json:"proposedLinks"`
		UnlinkedQuestions []map[string]any `json:"unlinkedQuestions"`
	}
	_ = json.Unmarshal([]byte(analysisJSON), &analysis)

	confirm := fmt.Sprintf(
		"Generate kisi-kisi dari soal existing: ~%d slot terdeteksi, ~%d soal akan otomatis terhubung (confidence ≥ %.2f), %d soal perlu manual",
		len(analysis.ProposedSlots),
		len(analysis.ProposedLinks),
		p.MinConfidence,
		len(analysis.UnlinkedQuestions),
	)
	if hasBP && p.Replace {
		confirm += " — REPLACE blueprint existing"
	}
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "convert_questions_to_kisi_kisi", args, confirm)
}

func (a *App) execConvertQuestionsToKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID         string  `json:"examId"`
		Title          string  `json:"title"`
		Description    string  `json:"description"`
		CurriculumCode string  `json:"curriculumCode"`
		BlueprintType  string  `json:"blueprintType"`
		MinConfidence  float64 `json:"minConfidence"`
		Replace        bool    `json:"replace"`
	}
	_ = json.Unmarshal(args, &p)
	if p.MinConfidence <= 0 || p.MinConfidence > 1 {
		p.MinConfidence = 0.7
	}
	if p.Title == "" {
		p.Title = "Kisi-Kisi (auto-generated)"
	}
	if p.BlueprintType == "" {
		if strings.HasPrefix(p.CurriculumCode, "akm_") {
			p.BlueprintType = p.CurriculumCode
		} else {
			p.BlueprintType = "reguler"
		}
	}

	if denied := a.checkAIExamAccess(ctx, tenantID, userID, p.ExamID, ActionWrite); denied != "" {
		return denied, nil
	}

	// Re-run the analyzer at confirm time so we use the latest exam state.
	analyzeArgs, _ := json.Marshal(map[string]any{
		"examId":        p.ExamID,
		"minConfidence": p.MinConfidence,
		"batchSize":     50,
	})
	analysisJSON, _ := a.capAnalyzeQuestionsToBlueprint(ctx, tenantID, userID, analyzeArgs)
	var analysis struct {
		ProposedSlots []struct {
			CompetencyCode string  `json:"competencyCode"`
			Materi         string  `json:"materi"`
			CognitiveLevel string  `json:"cognitiveLevel"`
			Difficulty     string  `json:"difficulty"`
			QuestionType   string  `json:"questionType"`
			Points         float64 `json:"points"`
		} `json:"proposedSlots"`
		ProposedLinks []struct {
			QuestionIndex int     `json:"questionIndex"`
			SlotIndex     int     `json:"slotIndex"`
			QuestionID    string  `json:"questionId"`
			Confidence    float64 `json:"confidence"`
		} `json:"proposedLinks"`
		UnlinkedQuestions []map[string]any `json:"unlinkedQuestions"`
	}
	_ = json.Unmarshal([]byte(analysisJSON), &analysis)

	// Auto-accept proposals at or above the confidence threshold. Each
	// link maps 1:1 to a slot in heuristic mode (analyzer guarantees
	// slotIndex == questionIndex when present).
	type acceptedSlot struct {
		QuestionID     string  `json:"questionId"`
		CompetencyCode string  `json:"competencyCode"`
		Materi         string  `json:"materi"`
		CognitiveLevel string  `json:"cognitiveLevel"`
		Difficulty     string  `json:"difficulty"`
		QuestionType   string  `json:"questionType"`
		Points         float64 `json:"points"`
	}
	slotForIndex := map[int]acceptedSlot{}
	for i, s := range analysis.ProposedSlots {
		slotForIndex[i] = acceptedSlot{
			CompetencyCode: s.CompetencyCode,
			Materi:         s.Materi,
			CognitiveLevel: s.CognitiveLevel,
			Difficulty:     s.Difficulty,
			QuestionType:   s.QuestionType,
			Points:         s.Points,
		}
	}
	accepted := make([]acceptedSlot, 0, len(analysis.ProposedLinks))
	linked := 0
	for _, link := range analysis.ProposedLinks {
		if link.Confidence < p.MinConfidence {
			continue
		}
		spec, ok := slotForIndex[link.SlotIndex]
		if !ok {
			continue
		}
		spec.QuestionID = link.QuestionID
		accepted = append(accepted, spec)
		linked++
	}
	if len(accepted) == 0 {
		return errInvalidState("Tidak ada proposal di atas confidence threshold; kisi-kisi tidak dibentuk. Turunkan minConfidence atau gunakan analyze_questions_to_blueprint untuk review manual."), nil
	}

	// Delegate to apply_blueprint_analysis executor by serialising args.
	applyArgs, _ := json.Marshal(map[string]any{
		"examId":         p.ExamID,
		"title":          p.Title,
		"description":    p.Description,
		"curriculumCode": p.CurriculumCode,
		"blueprintType":  p.BlueprintType,
		"replace":        p.Replace,
		"acceptedSlots":  accepted,
	})
	result, err := a.execApplyBlueprintAnalysis(ctx, tenantID, userID, applyArgs)
	if err != nil {
		return "", err
	}
	_ = result
	return fmt.Sprintf(
		`{"success":true,"message":"Kisi-kisi otomatis dari soal existing dibuat","createdSlots":%d,"linkedQuestions":%d,"unlinkedQuestions":%d}`,
		len(accepted), linked, len(analysis.UnlinkedQuestions),
	), nil
}
