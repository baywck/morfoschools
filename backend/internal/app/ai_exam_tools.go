package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// AI exam authoring tools — registered into the CapabilityRegistry under
// the "exams" domain. Mirrors the pattern in ai_cap_registry.go but kept
// separate so Phase 9 changes are localized.
//
// Capabilities:
//   list_exams              read   exams:read
//   search_exams            read   exams:read
//   get_exam                read   exams:read
//   create_exam             write  exams:write    (proposes; user confirms)
//   create_question         write  exams:write    (proposes; user confirms)
//   batch_create_questions  write  exams:write    (proposes a batch)
//   list_questions          read   exams:read

func (a *App) registerExamCapabilities(reg *CapabilityRegistry) {
	reg.Register(Capability{
		Name:        "list_exams",
		Description: "List exams in tenant. Filter: status, subjectId.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"status":{"type":"string"},"subjectId":{"type":"string"},"search":{"type":"string"},"limit":{"type":"integer","default":20}}}`),
	}, a.capListExams)

	reg.Register(Capability{
		Name:        "get_exam",
		Description: "Get exam detail + question count + total points.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"}},"required":["examId"]}`),
	}, a.capGetExam)

	reg.Register(Capability{
		Name:        "list_questions",
		Description: "List questions of an exam.",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"}},"required":["examId"]}`),
	}, a.capListQuestions)

	reg.Register(Capability{
		Name:        "find_similar_questions",
		Description: "Cari soal yang mirip dengan content kandidat di exam tertentu via trigram similarity. WAJIB dipanggil sebelum create_question/batch_create_questions kalau exam punya banyak soal existing, untuk menghindari paraphrase duplicate. threshold default 0.6 (paraphrase), 0.85 (probable dup), 0.95 (near-exact).",
		Permission:  "exams:read",
		Risk:        "read",
		Domain:      "exams",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"examId":{"type":"string"},"content":{"type":"string"},"threshold":{"type":"number"}},"required":["examId","content"]}`),
	}, a.capFindSimilarQuestions)

	reg.Register(Capability{
		Name:        "create_exam",
		Description: "Create new exam. title required.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"title":{"type":"string"},
			"description":{"type":"string"},
			"subjectId":{"type":"string"},
			"examType":{"type":"string","enum":["quiz","midterm","final","tryout","daily"]},
			"durationMinutes":{"type":"integer"},
			"maxScore":{"type":"number"},
			"passingScore":{"type":"number"},
			"shuffleQuestions":{"type":"boolean"},
			"shuffleOptions":{"type":"boolean"}
		},"required":["title"]}`),
	}, a.capCreateExam)

	reg.Register(Capability{
		Name:        "create_question",
		Description: "Create one high-quality contextual question. Prefer multiple_choice with inline stimulus/context, plausible homogeneous options, one key, and explanation. If exam uses kisi-kisi, include competencyCode/competencyDescription/materi/indikator/cognitiveLevel/difficulty so backend can auto-link kisi-kisi.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"sectionId":{"type":"string"},
			"groupId":{"type":"string","description":"opsional, masukkan soal ke group existing"},
			"stimulusId":{"type":"string","description":"opsional, link soal ke stimulus library"},
			"blueprintSlotId":{"type":"string","description":"opsional, link soal ke blueprint slot"},
			"competencyCode":{"type":"string"},
			"competencyDescription":{"type":"string"},
			"materi":{"type":"string"},
			"indikator":{"type":"string"},
			"cognitiveLevel":{"type":"string","enum":["C1","C2","C3","C4","C5","C6"]},
			"difficulty":{"type":"string","enum":["mudah","sedang","sulit"]},
			"questionType":{"type":"string","enum":["multiple_choice","true_false","short_answer","essay"]},
			"content":{"type":"string","description":"Pertanyaan / soal"},
			"explanation":{"type":"string"},
			"correctAnswer":{"type":"string","description":"Untuk short_answer: jawaban referensi"},
			"points":{"type":"number","default":1},
			"scoringMode":{"type":"string","enum":["correct_all","correct_one","percentage"]},
			"wrongPenaltyPct":{"type":"number","description":"0..1, dipakai di percentage mode"},
			"options":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"isCorrect":{"type":"boolean"},"pointsWeight":{"type":"number"}},"required":["content","isCorrect"]}}
		},"required":["examId","questionType","content"]}`),
	}, a.capCreateQuestion)

	reg.Register(Capability{
		Name:        "batch_create_questions",
		Description: "Create multiple high-quality contextual questions in one exam. Prefer multiple_choice with inline stimulus/context, plausible homogeneous options, key, explanation. If exam uses kisi-kisi, include competencyCode/competencyDescription/materi/indikator/cognitiveLevel/difficulty per question so backend can auto-link kisi-kisi.",
		Permission:  "exams:write",
		Risk:        "write",
		Domain:      "exams",
		Parameters: json.RawMessage(`{"type":"object","properties":{
			"examId":{"type":"string"},
			"sectionId":{"type":"string","description":"default sectionId untuk semua soal kalau tidak di-set per-item"},
			"groupId":{"type":"string","description":"default groupId untuk semua soal kalau tidak di-set per-item"},
			"stimulusId":{"type":"string","description":"default stimulusId untuk semua soal"},
			"questions":{"type":"array","items":{"type":"object","properties":{
				"questionType":{"type":"string","enum":["multiple_choice","true_false","short_answer","essay"]},
				"content":{"type":"string"},
				"explanation":{"type":"string"},
				"correctAnswer":{"type":"string"},
				"points":{"type":"number"},
				"scoringMode":{"type":"string"},
				"sectionId":{"type":"string","description":"override per-item"},
				"groupId":{"type":"string","description":"override per-item"},
				"stimulusId":{"type":"string","description":"override per-item"},
				"blueprintSlotId":{"type":"string"},
				"competencyCode":{"type":"string"},
				"competencyDescription":{"type":"string"},
				"materi":{"type":"string"},
				"indikator":{"type":"string"},
				"cognitiveLevel":{"type":"string","enum":["C1","C2","C3","C4","C5","C6"]},
				"difficulty":{"type":"string","enum":["mudah","sedang","sulit"]},
				"options":{"type":"array","items":{"type":"object"}}
			},"required":["questionType","content"]}}
		},"required":["examId","questions"]}`),
	}, a.capBatchCreateQuestions)
}

// --- Read handlers ---

func (a *App) capListExams(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Status    string `json:"status"`
		SubjectID string `json:"subjectId"`
		Search    string `json:"search"`
		Limit     int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &p)
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	q := `
		SELECT e.id, e.title, e.status, e.exam_type,
		       COALESCE(s.name, '') AS subject_name,
		       COALESCE((SELECT COUNT(*) FROM exam_questions q WHERE q.exam_id = e.id), 0) AS question_count,
		       COALESCE((SELECT SUM(points) FROM exam_questions q WHERE q.exam_id = e.id), 0) AS total_points
		  FROM exams e
		  LEFT JOIN subjects s ON s.id = e.subject_id
		 WHERE e.tenant_id = $1`
	args2 := []any{tenantID}
	idx := 2
	if p.Status != "" {
		q += fmt.Sprintf(" AND e.status = $%d", idx)
		args2 = append(args2, p.Status)
		idx++
	}
	if p.SubjectID != "" {
		q += fmt.Sprintf(" AND e.subject_id = $%d", idx)
		args2 = append(args2, p.SubjectID)
		idx++
	}
	if p.Search != "" {
		q += fmt.Sprintf(" AND e.title ILIKE $%d", idx)
		args2 = append(args2, "%"+p.Search+"%")
		idx++
	}
	q += fmt.Sprintf(" ORDER BY e.created_at DESC LIMIT $%d", idx)
	args2 = append(args2, p.Limit)

	rows, err := a.db.QueryContext(ctx, q, args2...)
	if err != nil {
		return `{"error":"list failed"}`, nil
	}
	defer rows.Close()

	type R struct {
		ID            string  `json:"id"`
		Title         string  `json:"title"`
		Status        string  `json:"status"`
		ExamType      string  `json:"examType"`
		SubjectName   string  `json:"subjectName"`
		QuestionCount int     `json:"questionCount"`
		TotalPoints   float64 `json:"totalPoints"`
	}
	out := make([]R, 0)
	for rows.Next() {
		var r R
		if err := rows.Scan(&r.ID, &r.Title, &r.Status, &r.ExamType, &r.SubjectName, &r.QuestionCount, &r.TotalPoints); err == nil {
			out = append(out, r)
		}
	}
	b, _ := json.Marshal(map[string]any{"data": out, "count": len(out)})
	return string(b), nil
}

func (a *App) capGetExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if p.ExamID == "" {
		return errValidationFailed("examId", "examId is required"), nil
	}
	if !isUUID(p.ExamID) {
		return errInvalidUUID("examId", p.ExamID, "exam"), nil
	}

	type R struct {
		ID            string  `json:"id"`
		Title         string  `json:"title"`
		Status        string  `json:"status"`
		QuestionCount int     `json:"questionCount"`
		TotalPoints   float64 `json:"totalPoints"`
	}
	var r R
	err := a.db.QueryRowContext(ctx, `
		SELECT id, title, status,
		       COALESCE((SELECT COUNT(*) FROM exam_questions WHERE exam_id = $1), 0),
		       COALESCE((SELECT SUM(points) FROM exam_questions WHERE exam_id = $1), 0)
		  FROM exams WHERE id = $1 AND tenant_id = $2`,
		p.ExamID, tenantID,
	).Scan(&r.ID, &r.Title, &r.Status, &r.QuestionCount, &r.TotalPoints)
	if err == sql.ErrNoRows {
		return errEntityNotFound("exam", "examId", p.ExamID), nil
	}
	if err != nil {
		return `{"error":"lookup failed"}`, nil
	}
	b, _ := json.Marshal(r)
	return string(b), nil
}

func (a *App) capListQuestions(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID string `json:"examId"`
	}
	_ = json.Unmarshal(args, &p)
	if p.ExamID == "" || !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId is required"), nil
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT id, question_type, content, points, sort_order, scoring_mode, correct_count
		  FROM exam_questions
		 WHERE exam_id = $1 AND tenant_id = $2
		 ORDER BY sort_order, id`,
		p.ExamID, tenantID,
	)
	if err != nil {
		return `{"error":"list failed"}`, nil
	}
	defer rows.Close()

	type Q struct {
		ID           string  `json:"id"`
		QuestionType string  `json:"questionType"`
		Content      string  `json:"content"`
		Points       float64 `json:"points"`
		SortOrder    int     `json:"sortOrder"`
		ScoringMode  string  `json:"scoringMode"`
		CorrectCount int     `json:"correctCount"`
	}
	out := make([]Q, 0)
	for rows.Next() {
		var q Q
		if err := rows.Scan(&q.ID, &q.QuestionType, &q.Content, &q.Points, &q.SortOrder, &q.ScoringMode, &q.CorrectCount); err == nil {
			out = append(out, q)
		}
	}
	b, _ := json.Marshal(map[string]any{
		"examId": p.ExamID,
		"data":   out,
		"count":  len(out),
	})
	return string(b), nil
}

// capFindSimilarQuestions runs a trigram similarity scan over the
// exam's existing questions and returns the top-K most similar ones
// above the threshold. AI agents call this BEFORE proposing new
// questions on exams that already have many soal, so paraphrase
// duplicates are surfaced before the user even sees the proposal.
//
// threshold default 0.6 surfaces 'paraphrase' candidates; the model
// can then decide to skip, regenerate, or proceed if the topic
// genuinely warrants similar phrasing (e.g., variant problems on the
// same operation).
func (a *App) capFindSimilarQuestions(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID    string  `json:"examId"`
		Content   string  `json:"content"`
		Threshold float64 `json:"threshold"`
	}
	_ = json.Unmarshal(args, &p)
	if p.ExamID == "" || !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId is required"), nil
	}
	if strings.TrimSpace(p.Content) == "" {
		return errValidationFailed("content", "content is required"), nil
	}
	if p.Threshold <= 0 || p.Threshold > 1 {
		p.Threshold = 0.6
	}

	// Tenant-scoped read access check.
	auth := AuthContext{UserID: userID}
	access, err := a.resolveExamAccess(ctx, tenantID, &auth, p.ExamID)
	if err != nil || !access.CanRead {
		return `{"error":"forbidden"}`, nil
	}

	normalized := normalizeQuestionContent(p.Content)
	hits, err := findSimilarQuestionsTopK(ctx, a.db, p.ExamID, normalized, p.Threshold, 5)
	if err != nil {
		return `{"error":"similarity_failed"}`, nil
	}

	if hits == nil {
		hits = []SimilarQuestionHit{}
	}

	// Truncate content per hit to keep token cost low; AI only needs
	// stems to detect overlap, not full body.
	for i := range hits {
		if len(hits[i].Content) > 160 {
			hits[i].Content = hits[i].Content[:160] + "…"
		}
	}

	b, _ := json.Marshal(map[string]any{
		"matches":        hits,
		"threshold":      p.Threshold,
		"interpretation": "sim>=0.95 near-exact, 0.85-0.95 probable dup, 0.6-0.85 paraphrase candidate",
	})
	return string(b), nil
}

// --- Write handlers (propose to ai_pending_actions) ---

func (a *App) capCreateExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		SubjectID       string  `json:"subjectId"`
		ExamType        string  `json:"examType"`
		DurationMinutes *int    `json:"durationMinutes"`
		MaxScore        float64 `json:"maxScore"`
		PassingScore    float64 `json:"passingScore"`
	}
	_ = json.Unmarshal(args, &p)
	if strings.TrimSpace(p.Title) == "" {
		return errValidationFailed("title", "Title is required"), nil
	}
	if p.SubjectID != "" && !isUUID(p.SubjectID) {
		return errInvalidUUID("subjectId", p.SubjectID, "subject"), nil
	}

	// Subject access check at propose time. Mirror of the HTTP path's
	// requireExamSubjectAccess so a teacher cannot ask the bot to create
	// an exam under a subject they aren't assigned to.
	if p.SubjectID != "" && !a.checkTeacherSubjectAccess(ctx, tenantID, userID, p.SubjectID) {
		return errPermissionDenied("create exam for this subject"), nil
	}

	// Pre-propose dedup: an active exam with the same title in this tenant
	// is almost certainly a mistake (the bot re-proposing under retry).
	if dup := a.checkExamDuplicate(ctx, tenantID, p.Title); dup != "" {
		return dup, nil
	}

	confirm := fmt.Sprintf("Buat exam baru: \"%s\"", p.Title)
	if p.ExamType != "" {
		confirm += fmt.Sprintf(" (%s)", p.ExamType)
	}

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_exam", args, confirm)
}

func (a *App) capCreateQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID       string           `json:"examId"`
		SectionID    string           `json:"sectionId"`
		QuestionType string           `json:"questionType"`
		Content      string           `json:"content"`
		Explanation  string           `json:"explanation"`
		GroupID      string           `json:"groupId"`
		Options      []questionOption `json:"options"`
		ScoringMode  string           `json:"scoringMode"`
	}
	_ = json.Unmarshal(args, &p)

	if p.ExamID == "" || !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId is required. Call list_exams first to get the UUID."), nil
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ExamID); denied != "" {
		return denied, nil
	}
	if errs := validateQuestionPayload(p.QuestionType, p.Content, p.ScoringMode, p.Options); len(errs) > 0 {
		// Surface the first error as the primary one — bot iterates per field
		var firstField, firstMsg string
		for k, v := range errs {
			firstField, firstMsg = k, v
			break
		}
		return errValidationFailed(firstField, firstMsg), nil
	}

	for _, issue := range validateGeneratedQuestionDraft(defaultExamAuthoringPolicy(tenantID, p.ExamID), generatedQuestionDraft{
		QuestionType: p.QuestionType,
		Content:      p.Content,
		Options:      p.Options,
		Grouped:      p.GroupID != "",
	}) {
		if issue.Field == "content" && issue.Severity == "warning" && p.GroupID == "" {
			return errValidationFailed(issue.Field, issue.Message+". Buat ulang dengan bacaan/stimulus 2-4 kalimat sebelum pertanyaan."), nil
		}
	}

	if dup := a.checkQuestionDuplicate(ctx, tenantID, p.ExamID, p.Content); dup != "" {
		return dup, nil
	}
	// Fuzzy duplicate detection (Phase 9.13). Catches paraphrases that
	// exact md5 hash misses. 0.85 threshold = probable dup.
	if normalized := normalizeQuestionContent(p.Content); normalized != "" {
		if _, existing, sim, ok := findSimilarQuestion(ctx, a.db, p.ExamID, normalized, 0.85); ok {
			excerpt := existing
			if len(excerpt) > 100 {
				excerpt = excerpt[:100] + "…"
			}
			b, _ := json.Marshal(map[string]any{
				"error": map[string]any{
					"code":        "DUPLICATE_FUZZY",
					"message":     fmt.Sprintf("Soal mirip dengan yang sudah ada (similarity=%.2f). Existing: %q", sim, excerpt),
					"recoverable": true,
					"recovery": map[string]any{
						"hint": "Ganti pendekatan: topik berbeda, level kognitif berbeda, stimulus berbeda, atau konfirmasi ke user kalau memang mau soal serupa (variant problem). Pakai find_similar_questions untuk lihat top match.",
					},
				},
			})
			return string(b), nil
		}
	}

	preview := p.Content
	if len(preview) > 140 {
		preview = preview[:140] + "…"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Tambah soal [%s]**\n\n", p.QuestionType))
	sb.WriteString("**❓ Pertanyaan:** ")
	sb.WriteString(preview)
	sb.WriteString("\n")
	if p.QuestionType == "multiple_choice" && len(p.Options) > 0 {
		sb.WriteString("**Pilihan:**\n")
		for j, opt := range p.Options {
			letter := string(rune('A' + j))
			mark := ""
			if opt.IsCorrect {
				mark = " ✅"
			}
			oc := opt.Content
			if len(oc) > 90 {
				oc = oc[:90] + "…"
			}
			sb.WriteString(fmt.Sprintf("%s. %s%s\n", letter, oc, mark))
		}
	}
	warnings := validateGeneratedQuestionDraft(defaultExamAuthoringPolicy(tenantID, p.ExamID), generatedQuestionDraft{
		QuestionType: p.QuestionType,
		Content:      p.Content,
		Explanation:  p.Explanation,
		Options:      p.Options,
		Grouped:      p.GroupID != "",
	})
	appendAuthoringWarnings(&sb, warnings)
	confirm := sb.String()

	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "create_question", args, confirm)
}

func (a *App) capBatchCreateQuestions(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID    string `json:"examId"`
		SectionID string `json:"sectionId"`
		Questions []struct {
			QuestionType string           `json:"questionType"`
			Content      string           `json:"content"`
			Explanation  string           `json:"explanation"`
			Points       *float64         `json:"points"`
			ScoringMode  string           `json:"scoringMode"`
			GroupID      string           `json:"groupId"`
			Options      []questionOption `json:"options"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(args, &p)

	if p.ExamID == "" || !isUUID(p.ExamID) {
		return errValidationFailed("examId", "Valid examId is required"), nil
	}
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ExamID); denied != "" {
		return denied, nil
	}
	if len(p.Questions) == 0 {
		return errValidationFailed("questions", "At least one question is required"), nil
	}
	if len(p.Questions) > 100 {
		return errValidationFailed("questions", "Maximum 100 questions per batch"), nil
	}

	// Validate all + dedup against committed AND in-flight (within this batch).
	type FailedItem struct {
		Index   int    `json:"index"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Content string `json:"content,omitempty"`
	}
	var failures []FailedItem
	seenHashes := map[string]int{}

	for i, q := range p.Questions {
		if errs := validateQuestionPayload(q.QuestionType, q.Content, q.ScoringMode, q.Options); len(errs) > 0 {
			for k, v := range errs {
				failures = append(failures, FailedItem{
					Index: i, Code: "VALIDATION_FAILED", Message: k + ": " + v, Content: q.Content,
				})
				break
			}
			continue
		}
		h := hashContent(q.Content)
		for _, issue := range validateGeneratedQuestionDraft(defaultExamAuthoringPolicy(tenantID, p.ExamID), generatedQuestionDraft{
			QuestionType: q.QuestionType,
			Content:      q.Content,
			Explanation:  q.Explanation,
			Options:      q.Options,
			Grouped:      q.GroupID != "",
		}) {
			if issue.Field == "content" && issue.Severity == "warning" && q.GroupID == "" {
				failures = append(failures, FailedItem{
					Index: i, Code: "QUALITY_CONTRACT_FAILED",
					Message: issue.Message + ". Buat ulang dengan bacaan/stimulus 2-4 kalimat sebelum pertanyaan.",
					Content: q.Content,
				})
				break
			}
		}
		if h != "" {
			if firstIdx, ok := seenHashes[h]; ok {
				failures = append(failures, FailedItem{
					Index: i, Code: "DUPLICATE_IN_BATCH",
					Message: fmt.Sprintf("Duplicate of question at index %d in this batch", firstIdx),
					Content: q.Content,
				})
				continue
			}
			seenHashes[h] = i
		}
		if dupErr := a.checkQuestionDuplicate(ctx, tenantID, p.ExamID, q.Content); dupErr != "" {
			failures = append(failures, FailedItem{
				Index: i, Code: "DUPLICATE_ENTRY",
				Message: "Question with same text already exists in this exam",
				Content: q.Content,
			})
			continue
		}
		// Fuzzy similarity check (Phase 9.13). pg_trgm catches paraphrase
		// duplicates that exact md5 hash misses (e.g. "Apa ibu kota X" vs
		// "Sebutkan ibu kota X"). Threshold 0.85 = probable duplicate.
		if normalized := normalizeQuestionContent(q.Content); normalized != "" {
			if _, existingContent, sim, ok := findSimilarQuestion(ctx, a.db, p.ExamID, normalized, 0.85); ok {
				excerpt := existingContent
				if len(excerpt) > 100 {
					excerpt = excerpt[:100] + "…"
				}
				failures = append(failures, FailedItem{
					Index: i, Code: "DUPLICATE_FUZZY",
					Message: fmt.Sprintf("Mirip dengan soal existing (similarity=%.2f): %q. Ganti pendekatan/topik atau gunakan stimulus yang berbeda.", sim, excerpt),
					Content: q.Content,
				})
			}
		}
	}

	// All-or-nothing batch propose: if anything fails, return without
	// creating proposals so the bot can fix the offending items first.
	if len(failures) > 0 {
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":        "BATCH_VALIDATION_FAILED",
				"message":     fmt.Sprintf("%d of %d questions invalid", len(failures), len(p.Questions)),
				"recoverable": true,
				"recovery": map[string]any{
					"hint": "Untuk setiap entry di 'failures', perbaiki content/options sesuai message. Untuk DUPLICATE_ENTRY/DUPLICATE_FUZZY, panggil find_similar_questions atau list_questions untuk lihat soal existing dan ganti pendekatan (topik berbeda, level kognitif berbeda, stimulus berbeda). Lalu retry batch_create_questions HANYA untuk soal yang lolos validasi.",
				},
				"failures": failures,
			},
		})
		return string(b), nil
	}

	// Create one proposal that wraps the entire batch. The executor splits
	// it into N rows in one transaction.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Tambah %d soal ke exam**\n\n", len(p.Questions)))
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
		warnings := validateGeneratedQuestionDraft(defaultExamAuthoringPolicy(tenantID, p.ExamID), generatedQuestionDraft{
			QuestionType: q.QuestionType,
			Content:      q.Content,
			Explanation:  q.Explanation,
			Options:      q.Options,
			Grouped:      q.GroupID != "",
		})
		appendAuthoringWarnings(&sb, warnings)
	}
	preview := sb.String()
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	return a.createProposal(ctx, sessionID, tenantID, userID, "batch_create_questions", args, preview)
}

// --- Executors (called from executeConfirmedAction switch) ---

func (a *App) execCreateExam(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		Title                 string   `json:"title"`
		Description           string   `json:"description"`
		SubjectID             string   `json:"subjectId"`
		ExamType              string   `json:"examType"`
		DurationMinutes       *int     `json:"durationMinutes"`
		MaxScore              *float64 `json:"maxScore"`
		PassingScore          *float64 `json:"passingScore"`
		ShuffleQuestions      bool     `json:"shuffleQuestions"`
		ShuffleOptions        bool     `json:"shuffleOptions"`
		ShowResultImmediately bool     `json:"showResultImmediately"`
	}
	_ = json.Unmarshal(args, &p)

	// Execute-layer guards: subject access + duplicate re-check.
	// The propose-time check ran when the user submitted; between propose
	// and confirm a colliding row may have committed elsewhere, so re-check
	// here per .ai/standards/ai-tool-guards.md.
	if p.SubjectID != "" && !a.checkTeacherSubjectAccess(ctx, tenantID, userID, p.SubjectID) {
		return errPermissionDenied("create exam for this subject"), nil
	}
	if dup := a.checkExamDuplicate(ctx, tenantID, p.Title); dup != "" {
		return dup, nil
	}

	if p.ExamType == "" {
		p.ExamType = "quiz"
	}
	maxScore := 100.0
	if p.MaxScore != nil {
		maxScore = *p.MaxScore
	}
	passingScore := 70.0
	if p.PassingScore != nil {
		passingScore = *p.PassingScore
	}

	var id string
	err := a.db.QueryRowContext(ctx, `
		INSERT INTO exams (
		    tenant_id, title, description, subject_id, exam_type,
		    duration_minutes, max_score, passing_score,
		    shuffle_questions, shuffle_options, show_result_immediately,
		    created_by, owner_user_id, status
		) VALUES ($1, $2, NULLIF($3,''), NULLIF($4,'')::uuid, $5,
		          $6, $7, $8, $9, $10, $11, $12, $12, 'draft')
		RETURNING id`,
		tenantID, p.Title, p.Description, p.SubjectID, p.ExamType,
		p.DurationMinutes, maxScore, passingScore,
		p.ShuffleQuestions, p.ShuffleOptions, p.ShowResultImmediately,
		userID,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "exams.create", "exam", id)
	return fmt.Sprintf(`{"success":true,"message":"Exam \"%s\" berhasil dibuat","examId":"%s"}`, p.Title, id), nil
}

func (a *App) execCreateQuestion(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	// Execute-layer guards before insert.
	var pre struct {
		ExamID  string `json:"examId"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(args, &pre)
	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, pre.ExamID); denied != "" {
		return denied, nil
	}
	if dup := a.checkQuestionDuplicate(ctx, tenantID, pre.ExamID, pre.Content); dup != "" {
		return dup, nil
	}

	id, err := a.insertQuestionWithOptions(ctx, tenantID, userID, args)
	if err != nil {
		return "", err
	}
	a.auditAI(ctx, tenantID, userID, "questions.create", "exam_question", id)
	return fmt.Sprintf(`{"success":true,"message":"Soal berhasil dibuat","questionId":"%s"}`, id), nil
}

func (a *App) execBatchCreateQuestions(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID     string            `json:"examId"`
		SectionID  string            `json:"sectionId"`
		GroupID    string            `json:"groupId"`
		StimulusID string            `json:"stimulusId"`
		Questions  []json.RawMessage `json:"questions"`
	}
	_ = json.Unmarshal(args, &p)

	if denied := a.checkExamWriteAccess(ctx, tenantID, userID, p.ExamID); denied != "" {
		return denied, nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	policy, err := loadExamAuthoringPolicy(ctx, tx, tenantID, p.ExamID)
	if err != nil {
		return "", err
	}
	bpID := ""
	bpPos := 0
	if policy.UsesKisiKisi {
		bpID, err = ensureExamBlueprintTx(ctx, tx, tenantID, p.ExamID, "merdeka")
		if err != nil {
			return "", err
		}
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM exam_blueprint_slots WHERE exam_blueprint_id = $1`, bpID).Scan(&bpPos); err != nil {
			return "", err
		}
	}

	created := 0
	linkedKisi := 0
	createdIDs := make([]string, 0, len(p.Questions))
	for _, qRaw := range p.Questions {
		var qm map[string]any
		_ = json.Unmarshal(qRaw, &qm)
		qm["examId"] = p.ExamID
		// Per-item override wins; batch-level value is the default.
		if _, ok := qm["sectionId"]; !ok && p.SectionID != "" {
			qm["sectionId"] = p.SectionID
		}
		if _, ok := qm["groupId"]; !ok && p.GroupID != "" {
			qm["groupId"] = p.GroupID
		}
		if _, ok := qm["stimulusId"]; !ok && p.StimulusID != "" {
			qm["stimulusId"] = p.StimulusID
		}
		merged, _ := json.Marshal(qm)

		id, err := a.insertQuestionWithOptionsTx(ctx, tx, tenantID, userID, merged)
		if err != nil {
			return "", err
		}
		createdIDs = append(createdIDs, id)
		created++

		if result, err := ensureQuestionKisiKisiLink(ctx, tx, tenantID, policy, bpID, id, qm, bpPos); err != nil {
			return "", err
		} else if result.Created {
			bpPos++
			linkedKisi++
		}
	}
	if policy.UsesKisiKisi && bpID != "" && linkedKisi > 0 {
		if err := updateExamBlueprintTotals(ctx, tx, bpID); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}

	// Best-effort audit, one event per created row. Failures don't unwind
	// the batch since the rows are already committed.
	for _, id := range createdIDs {
		a.auditAI(ctx, tenantID, userID, "questions.create", "exam_question", id)
	}
	a.auditAI(ctx, tenantID, userID, "questions.batch_create", "exam", p.ExamID)

	return fmt.Sprintf(`{"success":true,"message":"%d soal berhasil dibuat%s","count":%d,"questionIds":%s,"linkedKisiKisi":%d}`,
		created,
		func() string {
			if linkedKisi > 0 {
				return fmt.Sprintf("; %d kisi-kisi otomatis dibuat", linkedKisi)
			}
			return ""
		}(),
		created,
		mustJSONLocal(createdIDs),
		linkedKisi,
	), nil
}

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
	content := get("content")
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
	_ = content
	code := get("competencyCode")
	if code == "" {
		code = fmt.Sprintf("KOMP-%03d", pos+1)
	}
	desc := get("competencyDescription")
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

func mustJSONLocal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// checkExamWriteAccess is the execute-layer access predicate for AI tools.
// Returns "" when allowed, or a serialized ToolError JSON when denied.
func (a *App) checkExamWriteAccess(ctx context.Context, tenantID, userID, examID string) string {
	if examID == "" {
		return errValidationFailed("examId", "examId is required")
	}
	var subjectID sql.NullString
	err := a.db.QueryRowContext(ctx,
		`SELECT subject_id::text FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&subjectID)
	if err == sql.ErrNoRows {
		return errEntityNotFound("exam", "examId", examID)
	}
	if err != nil {
		return errValidationFailed("examId", "Could not load exam")
	}
	if subjectID.Valid && subjectID.String != "" {
		if !a.checkTeacherSubjectAccess(ctx, tenantID, userID, subjectID.String) {
			return errPermissionDenied("author for the subject of this exam")
		}
	}
	return ""
}

// auditAI emits an audit event from the AI execute path. The HTTP-bound
// a.audit needs *http.Request for IP/UA/request-id, which AI executors
// don't have, so we write directly with empty values for those fields.
func (a *App) auditAI(ctx context.Context, tenantID, actorID, action, resourceType, resourceID string) {
	if a.db == nil {
		return
	}
	t := tenantID
	_, _ = a.db.ExecContext(ctx,
		`INSERT INTO audit_events (tenant_id, actor_id, actor_type, action, resource_type, resource_id, request_id)
		 VALUES ($1, $2, 'ai_agent', $3, $4, $5, $6)`,
		t, actorID, action, resourceType, resourceID, RequestID(ctx),
	)
}

// insertQuestionWithOptions wraps insert + options in a fresh transaction.
// Used for single create_question executions.
func (a *App) insertQuestionWithOptions(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	id, err := a.insertQuestionWithOptionsTx(ctx, tx, tenantID, userID, args)
	if err != nil {
		return "", err
	}
	var qm map[string]any
	_ = json.Unmarshal(args, &qm)
	var examID string
	if v, ok := qm["examId"].(string); ok {
		examID = v
	}
	policy, err := loadExamAuthoringPolicy(ctx, tx, tenantID, examID)
	if err != nil {
		return "", err
	}
	if policy.UsesKisiKisi {
		bpID, err := ensureExamBlueprintTx(ctx, tx, tenantID, examID, "merdeka")
		if err != nil {
			return "", err
		}
		bpPos := 0
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM exam_blueprint_slots WHERE exam_blueprint_id = $1`, bpID).Scan(&bpPos)
		result, err := ensureQuestionKisiKisiLink(ctx, tx, tenantID, policy, bpID, id, qm, bpPos)
		if err != nil {
			return "", err
		}
		if result.Created {
			if err := updateExamBlueprintTotals(ctx, tx, bpID); err != nil {
				return "", err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// insertQuestionWithOptionsTx is the inner helper used by both single and
// batch executors. Caller owns the transaction lifecycle.
func (a *App) insertQuestionWithOptionsTx(ctx context.Context, tx *sql.Tx, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID          string           `json:"examId"`
		SectionID       string           `json:"sectionId"`
		GroupID         string           `json:"groupId"`
		StimulusID      string           `json:"stimulusId"`
		BlueprintSlotID string           `json:"blueprintSlotId"`
		QuestionType    string           `json:"questionType"`
		Content         string           `json:"content"`
		Explanation     string           `json:"explanation"`
		CorrectAnswer   string           `json:"correctAnswer"`
		Rubric          json.RawMessage  `json:"rubric"`
		Points          *float64         `json:"points"`
		SortOrder       *int             `json:"sortOrder"`
		ScoringMode     string           `json:"scoringMode"`
		WrongPenaltyPct *float64         `json:"wrongPenaltyPct"`
		ShuffleOpts     *bool            `json:"shuffleOptionsOverride"`
		Options         []questionOption `json:"options"`
	}
	_ = json.Unmarshal(args, &p)

	if p.ScoringMode == "" {
		p.ScoringMode = "correct_all"
	}
	// Section is mandatory (migration 000017). When the LLM omits it,
	// fall back to the exam's first section so the AI can author soal
	// with just examId + content. Mirrors the auto-section behaviour
	// the HTTP handler already provides on direct API calls.
	if p.SectionID == "" {
		_ = tx.QueryRowContext(ctx,
			`SELECT id::text FROM exam_sections
			  WHERE exam_id = $1 AND tenant_id = $2
			  ORDER BY sort_order ASC, created_at ASC LIMIT 1`,
			p.ExamID, tenantID,
		).Scan(&p.SectionID)
	}
	points := 1.0
	if p.Points != nil {
		points = *p.Points
	}
	sortOrder := 0
	if p.SortOrder != nil {
		sortOrder = *p.SortOrder
	} else {
		// Use the section-unified helper so new questions don't collide
		// with existing groups' display_order in the same section, and
		// in-group questions scope to their group instead of the exam.
		sortOrder = resolveQuestionPosition(ctx, tx, p.ExamID, p.SectionID, p.GroupID)
	}
	contentHash := hashContent(p.Content)

	// If groupId provided but sectionId/examId are missing, resolve
	// them from the group itself. NOTE: deliberately do NOT inherit
	// stimulus_id from the group — DB constraint exam_questions_stimulus
	// _xor_group_chk requires (stimulus_id IS NULL) OR (group_id IS NULL).
	// When the soal lives in a group, the stimulus link is implicit
	// via the group; the question's own stimulus_id must stay NULL.
	if p.GroupID != "" {
		var (
			gExamID, gSectionID string
		)
		err := tx.QueryRowContext(ctx,
			`SELECT exam_id::text, COALESCE(section_id::text, '')
			   FROM exam_question_groups WHERE id = $1 AND tenant_id = $2`,
			p.GroupID, tenantID,
		).Scan(&gExamID, &gSectionID)
		if err != nil {
			if err == sql.ErrNoRows {
				return "", fmt.Errorf("group %s not found", p.GroupID)
			}
			return "", err
		}
		if p.ExamID == "" {
			p.ExamID = gExamID
		}
		if p.SectionID == "" {
			p.SectionID = gSectionID
		}
		// Force-clear stimulusId on the question when it's in a group.
		p.StimulusID = ""
		// Re-resolve position now that we know the group definitively
		// (the earlier resolution may have used GroupID="" because the
		// caller passed groupId AFTER computing position). Only override
		// when caller didn't pin a sortOrder.
		if p.SortOrder == nil {
			sortOrder = nextGroupPosition(ctx, tx, p.GroupID)
		}
	}

	// Guard against LLM reusing an already-linked blueprint slot. The DB has
	// idx_exam_questions_one_per_slot to enforce one question per slot; if the
	// model passes a stale/occupied blueprintSlotId, create the question without
	// that slot and let the caller/auto-kisi path attach a fresh slot instead of
	// failing the whole proposal.
	if p.BlueprintSlotID != "" {
		var occupied bool
		_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM exam_questions WHERE blueprint_slot_id=$1 AND tenant_id=$2)`, p.BlueprintSlotID, tenantID).Scan(&occupied)
		if occupied {
			p.BlueprintSlotID = ""
		}
	}

	var id string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO exam_questions (
		    tenant_id, exam_id, section_id, group_id, stimulus_id, blueprint_slot_id,
		    question_type, content, explanation,
		    correct_answer, rubric, points, sort_order, scoring_mode,
		    wrong_penalty_pct, shuffle_options_override, content_hash, content_normalized, created_by
		) VALUES (
		    $1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid, NULLIF($6,'')::uuid,
		    $7, $8, NULLIF($9,''),
		    NULLIF($10,''), NULLIF($11,'')::jsonb, $12, $13, $14,
		    $15, $16, NULLIF($17,''), NULLIF($18,''), $19
		) RETURNING id`,
		tenantID, p.ExamID, p.SectionID, p.GroupID, p.StimulusID, p.BlueprintSlotID,
		p.QuestionType, p.Content, p.Explanation,
		p.CorrectAnswer, string(p.Rubric), points, sortOrder, p.ScoringMode,
		p.WrongPenaltyPct, p.ShuffleOpts, contentHash, normalizeQuestionContent(p.Content), userID,
	).Scan(&id)
	if err != nil {
		return "", err
	}

	for i, opt := range p.Options {
		order := opt.SortOrder
		if order == 0 {
			order = i
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO exam_question_options
			    (tenant_id, question_id, content, is_correct, sort_order, points_weight)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			tenantID, id, opt.Content, opt.IsCorrect, order, opt.PointsWeight,
		)
		if err != nil {
			return "", err
		}
	}
	markExamAIContextStale(ctx, tx, tenantID, p.ExamID)
	return id, nil
}
