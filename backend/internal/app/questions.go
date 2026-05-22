package app

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Exam Questions — the core authoring surface. Supports four types:
//
//   multiple_choice : 1+ correct options, 2-10 total
//   true_false      : two options, exactly one correct
//   short_answer    : free-text, optional reference answer for manual grading
//   essay           : free-text, no auto-grading
//
// Scoring modes (multiple_choice only):
//
//   correct_all  default: must select exactly all correct, no extras
//   correct_one  any single correct selection scores full points
//   percentage   score = points * (correct_selected / total_correct)
//                with optional wrong_penalty_pct subtracting
//                points * wrong_penalty_pct per wrong selection (clamped >=0)
//                or per-option points_weight when explicitly set
//
// Per-question shuffle override: if shuffle_options_override IS NOT NULL it
// wins over the parent exam.shuffle_options.
//
// Content hash: md5(lower(trim(content))) is stored in content_hash so the AI
// dupe-guard can ask "did I already create or propose this question text in
// this exam?" without scanning rows.

func (a *App) registerExamQuestionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/exams/{id}/questions", a.handleListQuestions)
	mux.HandleFunc("POST /api/v1/exams/{id}/questions", a.handleCreateQuestion)
	mux.HandleFunc("POST /api/v1/exams/{id}/questions/from-slot", a.handleCreateQuestionFromSlot)
	mux.HandleFunc("GET /api/v1/questions/{questionId}", a.handleGetQuestion)
	mux.HandleFunc("PATCH /api/v1/questions/{questionId}", a.handleUpdateQuestion)
	mux.HandleFunc("DELETE /api/v1/questions/{questionId}", a.handleDeleteQuestion)
	mux.HandleFunc("POST /api/v1/questions/{questionId}/options", a.handleCreateOption)
	mux.HandleFunc("PATCH /api/v1/options/{optionId}", a.handleUpdateOption)
	mux.HandleFunc("DELETE /api/v1/options/{optionId}", a.handleDeleteOption)
}

// --- Types ---

type questionOption struct {
	ID           string   `json:"id"`
	Content      string   `json:"content"`
	IsCorrect    bool     `json:"isCorrect"`
	SortOrder    int      `json:"sortOrder"`
	PointsWeight *float64 `json:"pointsWeight,omitempty"`
}

type questionStimulusRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type questionGroupRef struct {
	ID             string  `json:"id"`
	StimulusTitle  *string `json:"stimulusTitle,omitempty"`
}

type questionRow struct {
	ID              string               `json:"id"`
	ExamID          string               `json:"examId"`
	SectionID       *string              `json:"sectionId"`
	GroupID         *string              `json:"groupId,omitempty"`
	QuestionType    string               `json:"questionType"`
	Content         string               `json:"content"`
	Explanation     *string              `json:"explanation"`
	CorrectAnswer   *string              `json:"correctAnswer,omitempty"`
	Rubric          json.RawMessage      `json:"rubric,omitempty"`
	Points          float64              `json:"points"`
	SortOrder       int                  `json:"sortOrder"`
	ScoringMode     string               `json:"scoringMode"`
	WrongPenaltyPct *float64             `json:"wrongPenaltyPct,omitempty"`
	ShuffleOpts     *bool                `json:"shuffleOptionsOverride,omitempty"`
	CorrectCount    int                  `json:"correctCount"`
	BlueprintSlotID *string              `json:"blueprintSlotId,omitempty"`
	StimulusID      *string              `json:"stimulusId,omitempty"`
	Slot            *questionSlotRef     `json:"slot,omitempty"`
	Stimulus        *questionStimulusRef `json:"stimulus,omitempty"`
	Group           *questionGroupRef    `json:"group,omitempty"`
	Options         []questionOption     `json:"options,omitempty"`
	CreatedAt       string               `json:"createdAt"`
}

// questionSlotRef is the embedded slot summary returned alongside a
// question whose blueprint_slot_id is set. Lets the frontend render the
// slot-first canvas without an N+1 lookup per question.
type questionSlotRef struct {
	ID                    string  `json:"id"`
	Position              int     `json:"position"`
	CompetencyCode        *string `json:"competencyCode,omitempty"`
	CompetencyDescription *string `json:"competencyDescription,omitempty"`
	Materi                *string `json:"materi,omitempty"`
	Indikator      *string `json:"indikator,omitempty"`
	CognitiveLevel *string `json:"cognitiveLevel,omitempty"`
	Difficulty     *string `json:"difficulty,omitempty"`
	QuestionType   *string `json:"questionType,omitempty"`
	Points         float64 `json:"points"`
	// Phase 9.8 — AKM dimensions surface inline alongside the rest of
	// the slot summary so the question accordion can render them when
	// blueprint_type is AKM.
	AkmKonten   *string `json:"akmKonten,omitempty"`
	AkmKonteks  *string `json:"akmKonteks,omitempty"`
	AkmProses   *string `json:"akmProses,omitempty"`
	AkmLevel    *int    `json:"akmLevel,omitempty"`
	// Set when the parent blueprint was cloned from a template. The
	// frontend uses this signal to lock metadata edits on the slot
	// (template-locked vs auto-blueprint).
	FromTemplate bool `json:"fromTemplate"`
}

// hashContent normalizes question text before hashing: lowercase, trim, and
// collapse internal whitespace. This makes the dedup robust against the most
// common LLM variations ("  hello world  " vs "hello  world\n").
func hashContent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// --- List ---

func (a *App) handleListQuestions(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	examID := r.PathValue("id")

	// Gate enforcement: callers without exams:write are takers, not authors.
	// They can only fetch questions while at least one gate window is open.
	// Authors (exams:write) and platform admins always bypass the gate.
	auth := AuthFromContext(r.Context())
	if !hasPermission(auth, "exams:write") && (auth == nil || !auth.IsPlatformAdmin) {
		var examStatus string
		err := a.db.QueryRowContext(r.Context(),
			`SELECT status FROM exams WHERE id = $1 AND tenant_id = $2`,
			examID, tenantID,
		).Scan(&examStatus)
		if err != nil {
			writeErrorJSON(w, http.StatusNotFound, "not_found", "Exam not found", r)
			return
		}
		if examStatus != "published" {
			writeErrorJSON(w, http.StatusForbidden, "exam_not_open", "This exam is not currently available", r)
			return
		}
		var gateCount int
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM exam_gate_windows WHERE exam_id = $1 AND tenant_id = $2`,
			examID, tenantID,
		).Scan(&gateCount)
		if gateCount > 0 {
			var isOpen bool
			_ = a.db.QueryRowContext(r.Context(), `
				SELECT EXISTS(
				    SELECT 1 FROM exam_gate_windows
				     WHERE exam_id = $1 AND tenant_id = $2
				       AND now() BETWEEN opens_at AND closes_at
				)`,
				examID, tenantID,
			).Scan(&isOpen)
			if !isOpen {
				writeErrorJSON(w, http.StatusForbidden, "exam_not_open",
					"This exam is not currently within an open window", r)
				return
			}
		}
	}

	// One round-trip per exam: questions with their options nested via
	// json_agg, ordered by section then sort_order. We also embed the
	// blueprint slot summary when the question is linked to a slot, so
	// the slot-first canvas (Phase 9.6) renders without an N+1 lookup.
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT q.id, q.exam_id, q.section_id, q.group_id::text, q.question_type, q.content,
		       q.explanation, q.correct_answer, q.rubric,
		       q.points, q.sort_order, q.scoring_mode, q.wrong_penalty_pct,
		       q.shuffle_options_override, q.correct_count, q.created_at,
		       q.blueprint_slot_id::text, q.stimulus_id::text,
		       s.position, s.competency_code, s.competency_description, s.materi, s.indikator,
		       s.cognitive_level, s.difficulty, s.question_type, s.points,
		       s.akm_konten, s.akm_konteks, s.akm_proses, s.akm_level,
		       b.source_template_id::text,
		       st.title,
		       g.stimulus_title_snapshot,
		       COALESCE((
		           SELECT json_agg(
		               json_build_object(
		                   'id', o.id,
		                   'content', o.content,
		                   'isCorrect', o.is_correct,
		                   'sortOrder', o.sort_order,
		                   'pointsWeight', o.points_weight
		               )
		               ORDER BY o.sort_order, o.id
		           )
		             FROM exam_question_options o
		            WHERE o.question_id = q.id
		       ), '[]'::json) AS options
		  FROM exam_questions q
		  LEFT JOIN exam_blueprint_slots s ON s.id = q.blueprint_slot_id
		  LEFT JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		  LEFT JOIN stimuli st ON st.id = q.stimulus_id
		  LEFT JOIN exam_question_groups g ON g.id = q.group_id
		 WHERE q.exam_id = $1 AND q.tenant_id = $2
		 ORDER BY COALESCE(q.section_id::text, ''), q.sort_order, q.id`,
		examID, tenantID,
	)
	if err != nil {
		a.logger.Error("list questions failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "questions_lookup_failed", "Could not load questions", r)
		return
	}
	defer rows.Close()

	out := make([]questionRow, 0)
	for rows.Next() {
		var q questionRow
		var optionsJSON []byte
		var rubricBytes sql.NullString
		var groupID, slotID, stimulusID sql.NullString
		var slotPos sql.NullInt64
		var slotCompCode, slotCompDesc, slotMateri, slotIndikator sql.NullString
		var slotCog, slotDiff, slotQType sql.NullString
		var slotPoints sql.NullFloat64
		var slotAkmKonten, slotAkmKonteks, slotAkmProses sql.NullString
		var slotAkmLevel sql.NullInt64
		var slotTemplateID sql.NullString
		var stimulusTitle sql.NullString
		var groupStimulusTitle sql.NullString
		if err := rows.Scan(
			&q.ID, &q.ExamID, &q.SectionID, &groupID, &q.QuestionType, &q.Content,
			&q.Explanation, &q.CorrectAnswer, &rubricBytes,
			&q.Points, &q.SortOrder, &q.ScoringMode, &q.WrongPenaltyPct,
			&q.ShuffleOpts, &q.CorrectCount, &q.CreatedAt,
			&slotID, &stimulusID,
			&slotPos, &slotCompCode, &slotCompDesc, &slotMateri, &slotIndikator,
			&slotCog, &slotDiff, &slotQType, &slotPoints,
			&slotAkmKonten, &slotAkmKonteks, &slotAkmProses, &slotAkmLevel,
			&slotTemplateID,
			&stimulusTitle,
			&groupStimulusTitle,
			&optionsJSON,
		); err != nil {
			a.logger.Error("scan question failed", "error", err)
			continue
		}
		if rubricBytes.Valid {
			q.Rubric = json.RawMessage(rubricBytes.String)
		}
		if groupID.Valid {
			gid := groupID.String
			q.GroupID = &gid
			ref := questionGroupRef{ID: gid}
			if groupStimulusTitle.Valid {
				t := groupStimulusTitle.String
				ref.StimulusTitle = &t
			}
			q.Group = &ref
		}
		if slotID.Valid {
			sid := slotID.String
			q.BlueprintSlotID = &sid
			slot := questionSlotRef{ID: sid}
			if slotPos.Valid {
				slot.Position = int(slotPos.Int64)
			}
			if slotCompCode.Valid {
				v := slotCompCode.String
				slot.CompetencyCode = &v
			}
			if slotCompDesc.Valid {
				v := slotCompDesc.String
				slot.CompetencyDescription = &v
			}
			if slotMateri.Valid {
				v := slotMateri.String
				slot.Materi = &v
			}
			if slotIndikator.Valid {
				v := slotIndikator.String
				slot.Indikator = &v
			}
			if slotCog.Valid {
				v := slotCog.String
				slot.CognitiveLevel = &v
			}
			if slotDiff.Valid {
				v := slotDiff.String
				slot.Difficulty = &v
			}
			if slotQType.Valid {
				v := slotQType.String
				slot.QuestionType = &v
			}
			if slotPoints.Valid {
				slot.Points = slotPoints.Float64
			}
			if slotAkmKonten.Valid {
				v := slotAkmKonten.String
				slot.AkmKonten = &v
			}
			if slotAkmKonteks.Valid {
				v := slotAkmKonteks.String
				slot.AkmKonteks = &v
			}
			if slotAkmProses.Valid {
				v := slotAkmProses.String
				slot.AkmProses = &v
			}
			if slotAkmLevel.Valid {
				v := int(slotAkmLevel.Int64)
				slot.AkmLevel = &v
			}
			slot.FromTemplate = slotTemplateID.Valid && slotTemplateID.String != ""
			q.Slot = &slot
		}
		if stimulusID.Valid {
			sid := stimulusID.String
			q.StimulusID = &sid
			ref := questionStimulusRef{ID: sid}
			if stimulusTitle.Valid {
				ref.Title = stimulusTitle.String
			}
			q.Stimulus = &ref
		}
		_ = json.Unmarshal(optionsJSON, &q.Options)
		out = append(out, q)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// --- Get single ---

func (a *App) handleGetQuestion(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:read") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	qid := r.PathValue("questionId")

	var q questionRow
	var rubricBytes sql.NullString
	err := a.db.QueryRowContext(r.Context(), `
		SELECT id, exam_id, section_id, question_type, content,
		       explanation, correct_answer, rubric,
		       points, sort_order, scoring_mode, wrong_penalty_pct,
		       shuffle_options_override, correct_count, created_at
		  FROM exam_questions
		 WHERE id = $1 AND tenant_id = $2`,
		qid, tenantID,
	).Scan(
		&q.ID, &q.ExamID, &q.SectionID, &q.QuestionType, &q.Content,
		&q.Explanation, &q.CorrectAnswer, &rubricBytes,
		&q.Points, &q.SortOrder, &q.ScoringMode, &q.WrongPenaltyPct,
		&q.ShuffleOpts, &q.CorrectCount, &q.CreatedAt,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Question not found", r)
		return
	}
	if rubricBytes.Valid {
		q.Rubric = json.RawMessage(rubricBytes.String)
	}

	// Load options separately
	q.Options = a.loadOptions(r, qid)
	writeJSON(w, http.StatusOK, q)
}

// loadOptions returns ordered options for a question. Used by Get and after
// mutations.
func (a *App) loadOptions(r *http.Request, questionID string) []questionOption {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, content, is_correct, sort_order, points_weight
		  FROM exam_question_options
		 WHERE question_id = $1
		 ORDER BY sort_order, id`,
		questionID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]questionOption, 0)
	for rows.Next() {
		var o questionOption
		if err := rows.Scan(&o.ID, &o.Content, &o.IsCorrect, &o.SortOrder, &o.PointsWeight); err == nil {
			out = append(out, o)
		}
	}
	return out
}

// --- Create question ---

func (a *App) handleCreateQuestion(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	examID := r.PathValue("id")

	// Layered access check (ADR-0009). Replaces prior subject-only gate.
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	auth := AuthFromContext(r.Context())

	var req struct {
		SectionID       *string          `json:"sectionId"`
		GroupID         *string          `json:"groupId"`
		StimulusID      *string          `json:"stimulusId"`
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
		BlueprintSlotID *string          `json:"blueprintSlotId"`
		ForceLink       bool             `json:"forceLink"`
		Options         []questionOption `json:"options"`
		// Phase 9.8 — inline kisi-kisi metadata. When uses_kisi_kisi=true
		// and no slot is supplied we auto-create one carrying these
		// pedagogical fields. When a slot is supplied these are written
		// through to the slot row in the same transaction.
		CompetencyCode        *string `json:"competencyCode"`
		CompetencyDescription *string `json:"competencyDescription"`
		Materi                *string `json:"materi"`
		Indikator             *string `json:"indikator"`
		CognitiveLevel        *string `json:"cognitiveLevel"`
		Difficulty            *string `json:"difficulty"`
		AkmKonten      *string `json:"akmKonten"`
		AkmKonteks     *string `json:"akmKonteks"`
		AkmProses      *string `json:"akmProses"`
		AkmLevel       *int    `json:"akmLevel"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	if errs := validateQuestionPayload(req.QuestionType, req.Content, req.ScoringMode, req.Options); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}

	// Section is mandatory (Phase 9.8). Fall back to the exam's first
	// section when the client does not specify one — belt-and-suspenders
	// for older clients that haven't migrated to the new canvas.
	if req.SectionID == nil || *req.SectionID == "" {
		var fallback string
		if err := a.db.QueryRowContext(r.Context(), `
			SELECT id FROM exam_sections
			 WHERE exam_id = $1 AND tenant_id = $2
			 ORDER BY sort_order ASC, created_at ASC
			 LIMIT 1`,
			examID, tenantID,
		).Scan(&fallback); err != nil {
			writeValidationError(w, map[string]string{
				"sectionId": "Exam has no sections; create one before adding questions",
			}, r)
			return
		}
		req.SectionID = &fallback
	}
	// Validate the section actually belongs to this exam (and tenant).
	var sectionExamID string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT exam_id::text FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
		*req.SectionID, tenantID,
	).Scan(&sectionExamID); err != nil || sectionExamID != examID {
		writeValidationError(w, map[string]string{
			"sectionId": "Section does not belong to this exam",
		}, r)
		return
	}

	// Stimulus axis is mutex (ADR-0012). A question carries a stimulus
	// through stimulus_id (direct) OR group_id (group-mediated snapshot),
	// never both. The DB has a CHECK constraint as defence in depth; here
	// we surface a structured 422 instead of letting the constraint hit.
	hasStimulus := req.StimulusID != nil && *req.StimulusID != ""
	hasGroup := req.GroupID != nil && *req.GroupID != ""
	if hasStimulus && hasGroup {
		writeValidationError(w, map[string]string{
			"stimulusId": "Cannot set both stimulusId and groupId; pick one",
		}, r)
		return
	}
	if hasStimulus {
		if errs := a.validateStimulusForExam(r.Context(), tenantID, examID, *req.StimulusID); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}
	if hasGroup {
		if errs := a.validateGroupForExam(r.Context(), tenantID, examID, *req.GroupID); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}

	// Kisi-kisi axis (ADR-0012, refined Phase 9.8). When uses_kisi_kisi=true:
	//
	//   - If the request points at an existing slot, validate it.
	//   - Otherwise we auto-create a slot below carrying the inline
	//     pedagogical fields. The blueprint itself is auto-created via
	//     ensureExamBlueprint when missing. This replaces the old
	//     "must load template before authoring" gate.
	//
	// When uses_kisi_kisi=false the link is optional and we leave the
	// behaviour untouched.
	usesKisiKisi, err := a.loadExamUsesKisiKisi(r.Context(), tenantID, examID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "exam_lookup_failed", "Could not load exam state", r)
		return
	}
	if req.BlueprintSlotID != nil && *req.BlueprintSlotID != "" {
		if errs := a.validateSlotForExam(r.Context(), tenantID, examID, *req.BlueprintSlotID, req.ForceLink, ""); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}

	points := 1.0
	if req.Points != nil {
		points = *req.Points
	}
	scoringMode := req.ScoringMode
	if scoringMode == "" {
		scoringMode = "correct_all"
	}
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	} else {
		sortOrder = resolveQuestionPosition(
			r.Context(), a.db,
			examID,
			ptrToString(req.SectionID),
			ptrToString(req.GroupID),
		)
	}
	contentHash := hashContent(req.Content)

	// Block duplicate question content within the same exam (committed rows).
	if contentHash != "" {
		var dupExists bool
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM exam_questions WHERE exam_id = $1 AND content_hash = $2)`,
			examID, contentHash,
		).Scan(&dupExists)
		if dupExists {
			writeValidationError(w, map[string]string{
				"content": "A question with the same text already exists in this exam",
			}, r)
			return
		}
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create question", r)
		return
	}
	defer tx.Rollback()

	// Auto-blueprint + auto-slot path (Phase 9.8). When the exam tracks
	// kisi-kisi but the request does not point at an existing slot, we
	// ensure a blueprint exists and append a new slot carrying the
	// inline metadata fields. The newly-minted slot id is then used as
	// the question's binding.
	slotIDForInsert := ""
	if req.BlueprintSlotID != nil && *req.BlueprintSlotID != "" {
		slotIDForInsert = *req.BlueprintSlotID
	} else if usesKisiKisi {
		blueprintID, berr := a.ensureExamBlueprint(r.Context(), tx, tenantID, examID)
		if berr != nil {
			a.logger.Error("ensure blueprint failed", "error", berr)
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not initialize kisi-kisi blueprint", r)
			return
		}
		newSlotID, serr := a.appendBlueprintSlot(r.Context(), tx, blueprintID, slotPayload{
			CompetencyCode:        req.CompetencyCode,
			CompetencyDescription: req.CompetencyDescription,
			Materi:                req.Materi,
			Indikator:             req.Indikator,
			CognitiveLevel:        req.CognitiveLevel,
			Difficulty:            req.Difficulty,
			QuestionType:          &req.QuestionType,
			Points:                &points,
			AkmKonten:             req.AkmKonten,
			AkmKonteks:            req.AkmKonteks,
			AkmProses:             req.AkmProses,
			AkmLevel:              req.AkmLevel,
		})
		if serr != nil {
			a.logger.Error("append blueprint slot failed", "error", serr)
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create slot for kisi-kisi question", r)
			return
		}
		slotIDForInsert = newSlotID
	}

	var id string
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO exam_questions (
		    tenant_id, exam_id, section_id, question_type, content, explanation,
		    correct_answer, rubric, points, sort_order, scoring_mode,
		    wrong_penalty_pct, shuffle_options_override, content_hash, created_by,
		    blueprint_slot_id, stimulus_id, group_id
		) VALUES (
		    $1, $2, NULLIF($3,'')::uuid, $4, $5, NULLIF($6,''),
		    NULLIF($7,''), NULLIF($8,'')::jsonb, $9, $10, $11,
		    $12, $13, NULLIF($14,''), $15,
		    NULLIF($16,'')::uuid, NULLIF($17,'')::uuid, NULLIF($18,'')::uuid
		) RETURNING id`,
		tenantID, examID, ptrToString(req.SectionID), req.QuestionType, req.Content, req.Explanation,
		req.CorrectAnswer, string(req.Rubric), points, sortOrder, scoringMode,
		req.WrongPenaltyPct, req.ShuffleOpts, contentHash, auth.UserID,
		slotIDForInsert,
		ptrToString(req.StimulusID), ptrToString(req.GroupID),
	).Scan(&id)
	if err != nil {
		a.logger.Error("create question failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create question", r)
		return
	}

	// If forceLink was used on an existing slot, clear any other question
	// previously linked to it so the partial unique index stays satisfied.
	if req.BlueprintSlotID != nil && *req.BlueprintSlotID != "" && req.ForceLink {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE exam_questions SET blueprint_slot_id = NULL
			  WHERE blueprint_slot_id = $1 AND id <> $2`,
			*req.BlueprintSlotID, id,
		); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not relink slot", r)
			return
		}
	}

	// When the request supplies an existing slot AND inline kisi-kisi
	// fields, write them through to the slot so the metadata stays in
	// sync with the question. Slots from auto-blueprints carry no lock,
	// slots from template clones carry their template metadata; the
	// frontend gates whether the inputs are editable.
	if req.BlueprintSlotID != nil && *req.BlueprintSlotID != "" {
		inlinePatch := slotPayload{
			CompetencyCode:        req.CompetencyCode,
			CompetencyDescription: req.CompetencyDescription,
			Materi:                req.Materi,
			Indikator:             req.Indikator,
			CognitiveLevel:        req.CognitiveLevel,
			Difficulty:            req.Difficulty,
			AkmKonten:             req.AkmKonten,
			AkmKonteks:            req.AkmKonteks,
			AkmProses:             req.AkmProses,
			AkmLevel:              req.AkmLevel,
		}
		if slotPayloadHasMeta(inlinePatch) {
			q, args := buildSlotUpdateSQL("exam_blueprint_slots", *req.BlueprintSlotID, inlinePatch)
			if q != "" {
				if _, err := tx.ExecContext(r.Context(), q, args...); err != nil {
					writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not patch slot metadata", r)
					return
				}
			}
		}
	}

	// Insert options in the same transaction so a failure rolls back the
	// question too. The trigger fn_exam_question_recount_correct will fire
	// per-row and update correct_count automatically.
	for i, opt := range req.Options {
		order := opt.SortOrder
		if order == 0 {
			order = i
		}
		_, err := tx.ExecContext(r.Context(), `
			INSERT INTO exam_question_options
			    (tenant_id, question_id, content, is_correct, sort_order, points_weight)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			tenantID, id, opt.Content, opt.IsCorrect, order, opt.PointsWeight,
		)
		if err != nil {
			a.logger.Error("create option failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create option", r)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not finalize question", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "questions.create", "exam_question", id, r)
	resp := map[string]any{"id": id}
	if slotIDForInsert != "" {
		resp["slotId"] = slotIDForInsert
		a.audit(r.Context(), &tenantID, auth.UserID, "exam_blueprints.slot_filled", "exam_blueprint_slot", slotIDForInsert, r)
	}
	writeJSON(w, http.StatusCreated, resp)
}

// validateQuestionPayload returns a fields map suitable for writeValidationError.
// Empty map = clean.
func validateQuestionPayload(qType, content, scoringMode string, options []questionOption) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(content) == "" {
		errs["content"] = "Content is required"
	}
	switch qType {
	case "multiple_choice":
		if len(options) < 2 {
			errs["options"] = "Multiple choice requires at least 2 options"
		}
		if len(options) > 10 {
			errs["options"] = "Multiple choice supports at most 10 options"
		}
		correct := 0
		for _, o := range options {
			if o.IsCorrect {
				correct++
			}
		}
		if correct == 0 {
			errs["options"] = "Mark at least one option as correct"
		}
		if scoringMode != "" && scoringMode != "correct_all" && scoringMode != "correct_one" && scoringMode != "percentage" {
			errs["scoringMode"] = "Invalid scoring mode"
		}
	case "true_false":
		if len(options) != 2 {
			errs["options"] = "True/False requires exactly 2 options"
		}
		correct := 0
		for _, o := range options {
			if o.IsCorrect {
				correct++
			}
		}
		if correct != 1 {
			errs["options"] = "True/False must have exactly one correct option"
		}
	case "short_answer", "essay":
		// Free-text. No options. correctAnswer / rubric optional.
		if len(options) > 0 {
			errs["options"] = "This question type does not accept options"
		}
	case "":
		errs["questionType"] = "questionType is required"
	default:
		errs["questionType"] = "Unsupported question type"
	}
	return errs
}

// --- Update question ---

// loadExamUsesKisiKisi returns the uses_kisi_kisi column for an exam in
// the given tenant. Used by question handlers to decide whether to
// require a slot binding (true) or to allow free questions (false).
func (a *App) loadExamUsesKisiKisi(ctx context.Context, tenantID, examID string) (bool, error) {
	var uses bool
	err := a.db.QueryRowContext(ctx,
		`SELECT uses_kisi_kisi FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&uses)
	if err != nil {
		return false, err
	}
	return uses, nil
}

// ensureExamBlueprint returns the exam's blueprint id, creating an
// ad-hoc one (no template_id, created_via='manual') when missing.
// Runs inside the supplied transaction so the question insert and
// blueprint creation share a fate.
//
// Phase 9.8 — the new UX flips kisi-kisi=on without forcing the user
// to load a template. Slots are minted as questions are authored.
func (a *App) ensureExamBlueprint(ctx context.Context, tx *sql.Tx, tenantID, examID string) (string, error) {
	var blueprintID string
	err := tx.QueryRowContext(ctx,
		`SELECT id::text FROM exam_blueprints WHERE exam_id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&blueprintID)
	if err == nil {
		return blueprintID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	// Pick a curriculum default. We default to k13 because that's the
	// most common Indonesian school context; admins who use Merdeka
	// will swap by loading a Merdeka template later.
	var curriculumID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id::text FROM curricula WHERE code = 'k13' LIMIT 1`,
	).Scan(&curriculumID); err != nil {
		return "", err
	}

	var examTitle string
	_ = tx.QueryRowContext(ctx,
		`SELECT title FROM exams WHERE id = $1 AND tenant_id = $2`,
		examID, tenantID,
	).Scan(&examTitle)
	if examTitle == "" {
		examTitle = "Kisi-Kisi"
	}

	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exam_blueprints (
		    tenant_id, exam_id, source_template_id, source_template_version,
		    created_via, title, description, curriculum_id,
		    blueprint_type, total_slots, total_points, strict_coverage, status
		) VALUES ($1, $2, NULL, NULL, 'manual', $3, NULL, $4,
		          'reguler', 0, 0, false, 'draft')
		RETURNING id::text`,
		tenantID, examID, examTitle, curriculumID,
	).Scan(&blueprintID); err != nil {
		return "", err
	}
	return blueprintID, nil
}

// appendBlueprintSlot inserts a new slot at the end of the blueprint
// and returns its id. Runs inside the supplied transaction. Used by
// the auto-blueprint path in handleCreateQuestion to mint a slot
// carrying the question's inline kisi-kisi metadata.
func (a *App) appendBlueprintSlot(ctx context.Context, tx *sql.Tx, blueprintID string, p slotPayload) (string, error) {
	var nextPos int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) + 1 FROM exam_blueprint_slots WHERE exam_blueprint_id = $1`,
		blueprintID,
	).Scan(&nextPos); err != nil {
		return "", err
	}
	q, args := buildSlotInsertSQL("exam_blueprint_slots", "exam_blueprint_id", blueprintID, nextPos, p)
	var id string
	if err := tx.QueryRowContext(ctx, q, args...).Scan(&id); err != nil {
		return "", err
	}
	// Refresh totals so the coverage badge stays accurate without a
	// follow-up reload race.
	if _, err := tx.ExecContext(ctx, `
		UPDATE exam_blueprints SET
		    total_slots  = (SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1),
		    total_points = (SELECT COALESCE(SUM(points), 0) FROM exam_blueprint_slots WHERE exam_blueprint_id = $1),
		    updated_at = now()
		 WHERE id = $1`, blueprintID,
	); err != nil {
		return "", err
	}
	return id, nil
}

// slotPayloadHasMeta reports whether the inline kisi-kisi fields on a
// slot payload are non-empty. Used to decide whether handleUpdate /
// handleCreate question handlers should write through to the slot.
func slotPayloadHasMeta(p slotPayload) bool {
	if p.CompetencyCode != nil || p.CompetencyDescription != nil || p.Materi != nil || p.Indikator != nil {
		return true
	}
	if p.CognitiveLevel != nil || p.Difficulty != nil {
		return true
	}
	if p.AkmKonten != nil || p.AkmKonteks != nil || p.AkmProses != nil || p.AkmLevel != nil {
		return true
	}
	return false
}

// validateStimulusForExam ensures the stimulus exists in the tenant
// and is reachable from this exam (lifecycle=shared OR exam_scoped
// belonging to this exam). Returns a fields map (empty when valid).
func (a *App) validateStimulusForExam(
	ctx context.Context, tenantID, examID, stimulusID string,
) map[string]string {
	errs := map[string]string{}
	var lifecycle string
	var parentExamID sql.NullString
	err := a.db.QueryRowContext(ctx,
		`SELECT lifecycle, parent_exam_id::text FROM stimuli WHERE id = $1 AND tenant_id = $2`,
		stimulusID, tenantID,
	).Scan(&lifecycle, &parentExamID)
	if err != nil {
		errs["stimulusId"] = "Stimulus not found in this tenant"
		return errs
	}
	switch lifecycle {
	case "shared":
		return errs
	case "exam_scoped":
		if !parentExamID.Valid || parentExamID.String != examID {
			errs["stimulusId"] = "Exam-scoped stimulus belongs to a different exam"
		}
		return errs
	default:
		errs["stimulusId"] = "Stimulus is archived; pick another"
		return errs
	}
}

// validateGroupForExam ensures the question group belongs to this exam
// (and tenant). Returns a fields map (empty when valid).
func (a *App) validateGroupForExam(
	ctx context.Context, tenantID, examID, groupID string,
) map[string]string {
	errs := map[string]string{}
	var groupExamID string
	err := a.db.QueryRowContext(ctx,
		`SELECT exam_id::text FROM exam_question_groups WHERE id = $1 AND tenant_id = $2`,
		groupID, tenantID,
	).Scan(&groupExamID)
	if err != nil {
		errs["groupId"] = "Group not found in this tenant"
		return errs
	}
	if groupExamID != examID {
		errs["groupId"] = "Group belongs to a different exam"
	}
	return errs
}

// validateSlotForExam ensures the slot belongs to the exam's blueprint,
// is not currently filled by a different question, and matches the
// exam's tenant. Pass `excludeQuestionID` when validating during update
// so the question's own existing link doesn't trigger the filled check.
func (a *App) validateSlotForExam(
	ctx context.Context, tenantID, examID, slotID string, forceLink bool, excludeQuestionID string,
) map[string]string {
	errs := map[string]string{}
	var slotExamID, blueprintStatus string
	err := a.db.QueryRowContext(ctx, `
		SELECT b.exam_id::text, b.status
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE s.id = $1 AND b.tenant_id = $2`,
		slotID, tenantID,
	).Scan(&slotExamID, &blueprintStatus)
	if err != nil {
		errs["blueprintSlotId"] = "Slot not found in this tenant's blueprints"
		return errs
	}
	if slotExamID != examID {
		errs["blueprintSlotId"] = "Slot belongs to a different exam"
		return errs
	}
	if blueprintStatus == "locked" {
		errs["blueprintSlotId"] = "Blueprint is locked (parent exam is published)"
		return errs
	}
	// Filled check unless forceLink: at most one question per slot per
	// the partial unique index. forceLink lets the caller swap atomically.
	if !forceLink {
		var existing string
		_ = a.db.QueryRowContext(ctx,
			`SELECT id::text FROM exam_questions WHERE blueprint_slot_id = $1 LIMIT 1`,
			slotID,
		).Scan(&existing)
		if existing != "" && existing != excludeQuestionID {
			errs["blueprintSlotId"] = "Slot already has a question. Pass forceLink=true to swap."
			return errs
		}
	}
	return errs
}

// handleCreateQuestionFromSlot is the slot-first authoring entry point
// (ADR-0011). It mirrors handleCreateQuestion but the slot's
// pedagogical metadata (questionType, points) is the source of truth
// when not overridden in the body, and the link is established in the
// same transaction.
//
// Body: { slotId, content, explanation?, correctAnswer?, rubric?,
//         points?, scoringMode?, wrongPenaltyPct?, options?,
//         questionType? (override; defaults to slot's), forceLink? }
func (a *App) handleCreateQuestionFromSlot(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	examID := r.PathValue("id")
	if !a.requireExamAccess(w, r, examID, ActionWrite) {
		return
	}
	auth := AuthFromContext(r.Context())

	var req struct {
		SlotID          string           `json:"slotId"`
		SectionID       *string          `json:"sectionId"`
		QuestionType    string           `json:"questionType"`
		Content         string           `json:"content"`
		Explanation     string           `json:"explanation"`
		CorrectAnswer   string           `json:"correctAnswer"`
		Rubric          json.RawMessage  `json:"rubric"`
		Points          *float64         `json:"points"`
		ScoringMode     string           `json:"scoringMode"`
		WrongPenaltyPct *float64         `json:"wrongPenaltyPct"`
		ShuffleOpts     *bool            `json:"shuffleOptionsOverride"`
		ForceLink       bool             `json:"forceLink"`
		Options         []questionOption `json:"options"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}
	if req.SlotID == "" {
		writeValidationError(w, map[string]string{"slotId": "slotId is required"}, r)
		return
	}

	// Load slot defaults
	var (
		slotQType  sql.NullString
		slotPoints float64
	)
	err := a.db.QueryRowContext(r.Context(), `
		SELECT s.question_type, s.points
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE s.id = $1 AND b.exam_id = $2 AND b.tenant_id = $3`,
		req.SlotID, examID, tenantID,
	).Scan(&slotQType, &slotPoints)
	if err != nil {
		writeValidationError(w, map[string]string{
			"slotId": "Slot not found for this exam",
		}, r)
		return
	}

	qType := req.QuestionType
	if qType == "" && slotQType.Valid {
		qType = slotQType.String
	}
	if qType == "" {
		qType = "multiple_choice"
	}
	points := slotPoints
	if req.Points != nil {
		points = *req.Points
	}
	scoringMode := req.ScoringMode
	if scoringMode == "" {
		scoringMode = "correct_all"
	}

	if errs := validateQuestionPayload(qType, req.Content, scoringMode, req.Options); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}
	if errs := a.validateSlotForExam(r.Context(), tenantID, examID, req.SlotID, req.ForceLink, ""); len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}

	contentHash := hashContent(req.Content)
	if contentHash != "" {
		var dupExists bool
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM exam_questions WHERE exam_id = $1 AND content_hash = $2)`,
			examID, contentHash,
		).Scan(&dupExists)
		if dupExists {
			writeValidationError(w, map[string]string{
				"content": "A question with the same text already exists in this exam",
			}, r)
			return
		}
	}

	sortOrder := 0
	_ = a.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM exam_questions WHERE exam_id = $1`, examID,
	).Scan(&sortOrder)

	// Override with section-unified position so the new slot-bound
	// question appends after the last existing block in its section
	// (groups + standalones interleaved), not after the last question
	// across the entire exam.
	if req.SectionID != nil && *req.SectionID != "" {
		sortOrder = nextSectionPosition(r.Context(), a.db, *req.SectionID)
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create question", r)
		return
	}
	defer tx.Rollback()

	if req.ForceLink {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE exam_questions SET blueprint_slot_id = NULL WHERE blueprint_slot_id = $1`,
			req.SlotID,
		); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not clear prior slot link", r)
			return
		}
	}

	var id string
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO exam_questions (
		    tenant_id, exam_id, section_id, question_type, content, explanation,
		    correct_answer, rubric, points, sort_order, scoring_mode,
		    wrong_penalty_pct, shuffle_options_override, content_hash, created_by,
		    blueprint_slot_id
		) VALUES (
		    $1, $2, NULLIF($3,'')::uuid, $4, $5, NULLIF($6,''),
		    NULLIF($7,''), NULLIF($8,'')::jsonb, $9, $10, $11,
		    $12, $13, NULLIF($14,''), $15, $16
		) RETURNING id`,
		tenantID, examID, ptrToString(req.SectionID), qType, req.Content, req.Explanation,
		req.CorrectAnswer, string(req.Rubric), points, sortOrder, scoringMode,
		req.WrongPenaltyPct, req.ShuffleOpts, contentHash, auth.UserID,
		req.SlotID,
	).Scan(&id)
	if err != nil {
		a.logger.Error("create question from slot failed", "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create question", r)
		return
	}

	for i, opt := range req.Options {
		order := opt.SortOrder
		if order == 0 {
			order = i
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO exam_question_options
			    (tenant_id, question_id, content, is_correct, sort_order, points_weight)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			tenantID, id, opt.Content, opt.IsCorrect, order, opt.PointsWeight,
		); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create option", r)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not finalize question", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "questions.create", "exam_question", id, r)
	a.audit(r.Context(), &tenantID, auth.UserID, "exam_blueprints.slot_filled", "exam_blueprint_slot", req.SlotID, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "slotId": req.SlotID})
}

func (a *App) handleUpdateQuestion(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	qid := r.PathValue("questionId")

	auth := AuthFromContext(r.Context())
	examID, ok := a.requireQuestionWriteAccess(w, r, tenantID, auth, qid)
	if !ok {
		return
	}

	var req struct {
		SectionID       *string           `json:"sectionId"`
		GroupID         *string           `json:"groupId"`
		StimulusID      *string           `json:"stimulusId"`
		QuestionType    *string           `json:"questionType"`
		Content         *string           `json:"content"`
		Explanation     *string           `json:"explanation"`
		CorrectAnswer   *string           `json:"correctAnswer"`
		Rubric          *json.RawMessage  `json:"rubric"`
		Points          *float64          `json:"points"`
		SortOrder       *int              `json:"sortOrder"`
		ScoringMode     *string           `json:"scoringMode"`
		WrongPenaltyPct *float64          `json:"wrongPenaltyPct"`
		ShuffleOpts     *bool             `json:"shuffleOptionsOverride"`
		BlueprintSlotID *string           `json:"blueprintSlotId"`
		ForceLink       bool              `json:"forceLink"`
		Options         *[]questionOption `json:"options"`
		// Phase 9.8 — inline kisi-kisi metadata writes through to the
		// bound slot. Frontend gates whether these fields are editable
		// (template-locked vs auto-blueprint).
		CompetencyCode        *string `json:"competencyCode"`
		CompetencyDescription *string `json:"competencyDescription"`
		Materi                *string `json:"materi"`
		Indikator             *string `json:"indikator"`
		CognitiveLevel        *string `json:"cognitiveLevel"`
		Difficulty            *string `json:"difficulty"`
		AkmKonten      *string `json:"akmKonten"`
		AkmKonteks     *string `json:"akmKonteks"`
		AkmProses      *string `json:"akmProses"`
		AkmLevel       *int    `json:"akmLevel"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid request body", r)
		return
	}

	// Kisi-kisi axis (ADR-0012). When uses_kisi_kisi=true the caller
	// cannot null out blueprint_slot_id without first toggling kisi-kisi
	// off. New slot bindings must validate against the exam's blueprint.
	usesKisiKisi, err := a.loadExamUsesKisiKisi(r.Context(), tenantID, examID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "exam_lookup_failed", "Could not load exam state", r)
		return
	}
	if req.BlueprintSlotID != nil {
		if *req.BlueprintSlotID == "" {
			if usesKisiKisi {
				writeValidationError(w, map[string]string{
					"blueprintSlotId": "Cannot clear slot link while exam uses kisi-kisi",
				}, r)
				return
			}
		} else {
			if errs := a.validateSlotForExam(r.Context(), tenantID, examID, *req.BlueprintSlotID, req.ForceLink, qid); len(errs) > 0 {
				writeValidationError(w, errs, r)
				return
			}
		}
	}

	// Stimulus axis (ADR-0012) — mutex on update too. We compute the
	// post-state using current row values when the patch omits a field.
	var curStimulusID, curGroupID sql.NullString
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT stimulus_id::text, group_id::text FROM exam_questions
		  WHERE id = $1 AND tenant_id = $2`,
		qid, tenantID,
	).Scan(&curStimulusID, &curGroupID); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Question not found", r)
		return
	}
	effStimulus := ""
	if curStimulusID.Valid {
		effStimulus = curStimulusID.String
	}
	if req.StimulusID != nil {
		effStimulus = *req.StimulusID
	}
	effGroup := ""
	if curGroupID.Valid {
		effGroup = curGroupID.String
	}
	if req.GroupID != nil {
		effGroup = *req.GroupID
	}
	if effStimulus != "" && effGroup != "" {
		writeValidationError(w, map[string]string{
			"stimulusId": "Cannot set both stimulusId and groupId; pick one",
		}, r)
		return
	}
	if req.StimulusID != nil && *req.StimulusID != "" {
		if errs := a.validateStimulusForExam(r.Context(), tenantID, examID, *req.StimulusID); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}
	if req.GroupID != nil && *req.GroupID != "" {
		if errs := a.validateGroupForExam(r.Context(), tenantID, examID, *req.GroupID); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}

	// Load current question type + content for post-state validation and
	// duplicate-on-update checks.
	var curType, curContent, curScoringMode string
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT question_type, content, scoring_mode FROM exam_questions
		  WHERE id = $1 AND tenant_id = $2`,
		qid, tenantID,
	).Scan(&curType, &curContent, &curScoringMode); err != nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Question not found", r)
		return
	}

	// Validate optional section reassignment belongs to the same exam/tenant
	// (unless cleared). Without this check, a malicious caller can move a
	// question into an unrelated section by guessing UUIDs.
	if req.SectionID != nil && *req.SectionID != "" {
		var sectionExamID string
		if err := a.db.QueryRowContext(r.Context(),
			`SELECT exam_id FROM exam_sections WHERE id = $1 AND tenant_id = $2`,
			*req.SectionID, tenantID,
		).Scan(&sectionExamID); err != nil || sectionExamID != examID {
			writeValidationError(w, map[string]string{
				"sectionId": "Section does not belong to this exam",
			}, r)
			return
		}
	}

	// If options are provided in the patch, run the type-specific validation
	// against the post-state and check for duplicate content within the exam.
	effectiveContent := curContent
	if req.Content != nil {
		effectiveContent = *req.Content
	}
	effectiveScoringMode := curScoringMode
	if req.ScoringMode != nil {
		effectiveScoringMode = *req.ScoringMode
	}
	effectiveType := curType
	if req.QuestionType != nil && *req.QuestionType != "" {
		// QuestionType is locked when bound to a template slot.
		var slotID sql.NullString
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT blueprint_slot_id::text FROM exam_questions
			  WHERE id = $1 AND tenant_id = $2`,
			qid, tenantID,
		).Scan(&slotID)
		if slotID.Valid && slotID.String != "" {
			var templateID sql.NullString
			_ = a.db.QueryRowContext(r.Context(), `
				SELECT b.template_id::text
				  FROM exam_blueprint_slots s
				  JOIN exam_blueprints b ON b.id = s.blueprint_id
				 WHERE s.id = $1 AND s.tenant_id = $2`,
				slotID.String, tenantID,
			).Scan(&templateID)
			if templateID.Valid && templateID.String != "" && *req.QuestionType != curType {
				writeValidationError(w, map[string]string{
					"questionType": "Type is locked from template; unlink the slot first",
				}, r)
				return
			}
		}
		switch *req.QuestionType {
		case "multiple_choice", "true_false", "short_answer", "essay":
			effectiveType = *req.QuestionType
		default:
			writeValidationError(w, map[string]string{
				"questionType": "Unsupported question type",
			}, r)
			return
		}
	}
	if req.Options != nil {
		if errs := validateQuestionPayload(effectiveType, effectiveContent, effectiveScoringMode, *req.Options); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	} else if req.Content != nil || req.ScoringMode != nil || req.QuestionType != nil {
		// No new options supplied. Validate against existing options so a
		// content/scoringMode/questionType patch alone cannot break invariants.
		existingOptions := a.loadOptions(r, qid)
		if errs := validateQuestionPayload(effectiveType, effectiveContent, effectiveScoringMode, existingOptions); len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}

	// Duplicate-on-update: if content is changing, ensure no other question
	// in this exam has the same content_hash.
	if req.Content != nil {
		newHash := hashContent(*req.Content)
		if newHash != "" {
			var dupExists bool
			_ = a.db.QueryRowContext(r.Context(),
				`SELECT EXISTS(
				    SELECT 1 FROM exam_questions
				     WHERE exam_id = $1 AND tenant_id = $2 AND content_hash = $3 AND id <> $4
				)`,
				examID, tenantID, newHash, qid,
			).Scan(&dupExists)
			if dupExists {
				writeValidationError(w, map[string]string{
					"content": "Another question with the same text already exists in this exam",
				}, r)
				return
			}
		}
	}

	// Validate scoringMode enum. The DB has CHECK constraints but surfacing
	// a structured field error is friendlier than a 500.
	if req.ScoringMode != nil {
		switch *req.ScoringMode {
		case "", "correct_all", "correct_one", "percentage":
		default:
			writeValidationError(w, map[string]string{
				"scoringMode": "Invalid scoring mode",
			}, r)
			return
		}
	}
	if req.WrongPenaltyPct != nil {
		if *req.WrongPenaltyPct < 0 || *req.WrongPenaltyPct > 1 {
			writeValidationError(w, map[string]string{
				"wrongPenaltyPct": "Must be between 0 and 1",
			}, r)
			return
		}
	}

	// Build the partial UPDATE.
	parts := []string{"updated_at = now()"}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}

	if req.SectionID != nil {
		if *req.SectionID == "" {
			parts = append(parts, "section_id = NULL")
		} else {
			add("section_id", *req.SectionID)
		}
	}
	if req.Content != nil {
		add("content", *req.Content)
		add("content_hash", hashContent(*req.Content))
	}
	if req.Explanation != nil {
		if *req.Explanation == "" {
			parts = append(parts, "explanation = NULL")
		} else {
			add("explanation", *req.Explanation)
		}
	}
	if req.CorrectAnswer != nil {
		if *req.CorrectAnswer == "" {
			parts = append(parts, "correct_answer = NULL")
		} else {
			add("correct_answer", *req.CorrectAnswer)
		}
	}
	if req.Rubric != nil {
		if len(*req.Rubric) == 0 || string(*req.Rubric) == "null" {
			parts = append(parts, "rubric = NULL")
		} else {
			add("rubric", string(*req.Rubric))
		}
	}
	if req.Points != nil {
		add("points", *req.Points)
	}
	if req.SortOrder != nil {
		add("sort_order", *req.SortOrder)
	}
	if req.ScoringMode != nil {
		add("scoring_mode", *req.ScoringMode)
	}
	if req.WrongPenaltyPct != nil {
		add("wrong_penalty_pct", *req.WrongPenaltyPct)
	}
	if req.ShuffleOpts != nil {
		add("shuffle_options_override", *req.ShuffleOpts)
	}
	if req.QuestionType != nil && *req.QuestionType != "" && effectiveType != curType {
		add("question_type", effectiveType)
	}
	if req.BlueprintSlotID != nil {
		if *req.BlueprintSlotID == "" {
			parts = append(parts, "blueprint_slot_id = NULL")
		} else {
			add("blueprint_slot_id", *req.BlueprintSlotID)
		}
	}
	if req.StimulusID != nil {
		if *req.StimulusID == "" {
			parts = append(parts, "stimulus_id = NULL")
		} else {
			add("stimulus_id", *req.StimulusID)
		}
	}
	if req.GroupID != nil {
		if *req.GroupID == "" {
			parts = append(parts, "group_id = NULL")
		} else {
			add("group_id", *req.GroupID)
		}
	}

	if len(args) == 0 && req.Options == nil && req.SectionID == nil && req.BlueprintSlotID == nil && req.StimulusID == nil && req.GroupID == nil &&
		req.CompetencyCode == nil && req.CompetencyDescription == nil &&
		req.Materi == nil && req.Indikator == nil &&
		req.CognitiveLevel == nil && req.Difficulty == nil &&
		req.AkmKonten == nil && req.AkmKonteks == nil && req.AkmProses == nil && req.AkmLevel == nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": qid, "status": "no_change"})
		return
	}

	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update question", r)
		return
	}
	defer tx.Rollback()

	// forceLink: clear any other question currently bound to the slot so
	// the partial unique index stays satisfied when we set the new link.
	if req.BlueprintSlotID != nil && *req.BlueprintSlotID != "" && req.ForceLink {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE exam_questions SET blueprint_slot_id = NULL
			  WHERE blueprint_slot_id = $1 AND id <> $2`,
			*req.BlueprintSlotID, qid,
		); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not relink slot", r)
			return
		}
	}

	if len(args) > 0 || req.SectionID != nil || req.BlueprintSlotID != nil || req.StimulusID != nil || req.GroupID != nil {
		q := "UPDATE exam_questions SET " + joinComma(parts) +
			" WHERE id = $" + strconv.Itoa(idx) +
			" AND tenant_id = $" + strconv.Itoa(idx+1)
		args = append(args, qid, tenantID)
		if _, err := tx.ExecContext(r.Context(), q, args...); err != nil {
			a.logger.Error("update question failed", "error", err)
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update question", r)
			return
		}
	}

	// Replace-on-update for options. The audit identified that the previous
	// PATCH silently dropped the options field, so MCQ option text/correctness
	// edits never persisted. We delete-then-insert inside the same tx so an
	// invalid set rolls back cleanly. Validation already ran above against
	// the post-state, so by this point the new set is known-valid.
	if req.Options != nil {
		if _, err := tx.ExecContext(r.Context(),
			`DELETE FROM exam_question_options WHERE question_id = $1 AND tenant_id = $2`,
			qid, tenantID,
		); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not replace options", r)
			return
		}
		for i, opt := range *req.Options {
			order := opt.SortOrder
			if order == 0 {
				order = i
			}
			_, err := tx.ExecContext(r.Context(), `
				INSERT INTO exam_question_options
				    (tenant_id, question_id, content, is_correct, sort_order, points_weight)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				tenantID, qid, opt.Content, opt.IsCorrect, order, opt.PointsWeight,
			)
			if err != nil {
				writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not insert option", r)
				return
			}
		}
	}

	// Phase 9.8: inline kisi-kisi field writeback. When the question is
	// bound to a slot AND the patch carries pedagogical metadata, write
	// it through to the slot row. We resolve the post-state slot id
	// (request override or current binding) before the commit so the
	// audit trail reflects the right target.
	slotPatch := slotPayload{
		CompetencyCode:        req.CompetencyCode,
		CompetencyDescription: req.CompetencyDescription,
		Materi:                req.Materi,
		Indikator:             req.Indikator,
		CognitiveLevel:        req.CognitiveLevel,
		Difficulty:            req.Difficulty,
		AkmKonten:             req.AkmKonten,
		AkmKonteks:            req.AkmKonteks,
		AkmProses:             req.AkmProses,
		AkmLevel:              req.AkmLevel,
	}
	if slotPayloadHasMeta(slotPatch) {
		effectiveSlotID := ""
		if req.BlueprintSlotID != nil && *req.BlueprintSlotID != "" {
			effectiveSlotID = *req.BlueprintSlotID
		} else {
			var curSlot sql.NullString
			_ = tx.QueryRowContext(r.Context(),
				`SELECT blueprint_slot_id::text FROM exam_questions WHERE id = $1 AND tenant_id = $2`,
				qid, tenantID,
			).Scan(&curSlot)
			if curSlot.Valid {
				effectiveSlotID = curSlot.String
			}
		}
		if effectiveSlotID != "" {
			sq, sargs := buildSlotUpdateSQL("exam_blueprint_slots", effectiveSlotID, slotPatch)
			if sq != "" {
				if _, err := tx.ExecContext(r.Context(), sq, sargs...); err != nil {
					a.logger.Error("slot writeback failed", "error", err)
					writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not patch slot metadata", r)
					return
				}
				a.audit(r.Context(), &tenantID, auth.UserID, "exam_blueprints.slot_updated", "exam_blueprint_slot", effectiveSlotID, r)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not finalize question update", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "questions.update", "exam_question", qid, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": qid, "status": "updated"})
}

// --- Delete question ---

func (a *App) handleDeleteQuestion(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	qid := r.PathValue("questionId")

	auth := AuthFromContext(r.Context())
	if _, ok := a.requireQuestionWriteAccess(w, r, tenantID, auth, qid); !ok {
		return
	}

	res, err := a.db.ExecContext(r.Context(),
		`DELETE FROM exam_questions WHERE id = $1 AND tenant_id = $2`,
		qid, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete question", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Question not found", r)
		return
	}
	a.audit(r.Context(), &tenantID, auth.UserID, "questions.delete", "exam_question", qid, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": qid, "status": "deleted"})
}

// --- Options CRUD ---

// validateOptionsAfterMutation simulates the option set after a CRUD
// mutation and runs validateQuestionPayload to catch invariant breaks.
// E.g. cannot add an 11th option to MCQ, cannot mark both true/false
// options correct, cannot delete down to 1 option for MCQ.
//
// `simulated` is the post-mutation option list. The function loads the
// parent question type/content/scoringMode and validates against it.
func (a *App) validateOptionsAfterMutation(
	ctx context.Context, tenantID, questionID string, simulated []questionOption,
) (map[string]string, error) {
	var qType, content, scoringMode string
	err := a.db.QueryRowContext(ctx,
		`SELECT question_type, content, scoring_mode FROM exam_questions
		  WHERE id = $1 AND tenant_id = $2`,
		questionID, tenantID,
	).Scan(&qType, &content, &scoringMode)
	if err != nil {
		return nil, err
	}
	return validateQuestionPayload(qType, content, scoringMode, simulated), nil
}

func (a *App) handleCreateOption(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	qid := r.PathValue("questionId")

	// Verify the parent question belongs to this tenant AND the caller has
	// subject access. Without this check, a tenant-A user could attach an
	// option to a tenant-B question by guessing its UUID.
	auth := AuthFromContext(r.Context())
	if _, ok := a.requireQuestionWriteAccess(w, r, tenantID, auth, qid); !ok {
		return
	}

	var req struct {
		Content      string   `json:"content"`
		IsCorrect    bool     `json:"isCorrect"`
		SortOrder    *int     `json:"sortOrder"`
		PointsWeight *float64 `json:"pointsWeight"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeValidationError(w, map[string]string{"content": "Content is required"}, r)
		return
	}

	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	} else {
		_ = a.db.QueryRowContext(r.Context(),
			`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM exam_question_options WHERE question_id = $1`, qid,
		).Scan(&sortOrder)
	}

	// Post-state validation: load existing options + this new one and check
	// it doesn't break MCQ/TF invariants (e.g. 11th option, both TF correct).
	existing := a.loadOptions(r, qid)
	simulated := append(existing, questionOption{
		Content:      req.Content,
		IsCorrect:    req.IsCorrect,
		SortOrder:    sortOrder,
		PointsWeight: req.PointsWeight,
	})
	if errs, err := a.validateOptionsAfterMutation(r.Context(), tenantID, qid, simulated); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "validation_failed", "Could not validate options", r)
		return
	} else if len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}

	var id string
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO exam_question_options
		    (tenant_id, question_id, content, is_correct, sort_order, points_weight)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		tenantID, qid, req.Content, req.IsCorrect, sortOrder, req.PointsWeight,
	).Scan(&id)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", "Could not create option", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "options.create", "exam_question_option", id, r)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *App) handleUpdateOption(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	oid := r.PathValue("optionId")

	auth := AuthFromContext(r.Context())
	questionID, ok := a.requireOptionWriteAccess(w, r, tenantID, auth, oid)
	if !ok {
		return
	}

	var req struct {
		Content      *string  `json:"content"`
		IsCorrect    *bool    `json:"isCorrect"`
		SortOrder    *int     `json:"sortOrder"`
		PointsWeight *float64 `json:"pointsWeight"`
	}
	if err := readJSON(r, &req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "Invalid body", r)
		return
	}

	// Post-state validation: simulate the option set after this update so
	// flipping isCorrect (e.g. on true_false) cannot break the invariants.
	if req.IsCorrect != nil || req.Content != nil {
		existing := a.loadOptions(r, questionID)
		simulated := make([]questionOption, len(existing))
		copy(simulated, existing)
		for i := range simulated {
			if simulated[i].ID == oid {
				if req.Content != nil {
					simulated[i].Content = *req.Content
				}
				if req.IsCorrect != nil {
					simulated[i].IsCorrect = *req.IsCorrect
				}
				break
			}
		}
		if errs, err := a.validateOptionsAfterMutation(r.Context(), tenantID, questionID, simulated); err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "validation_failed", "Could not validate options", r)
			return
		} else if len(errs) > 0 {
			writeValidationError(w, errs, r)
			return
		}
	}

	parts := []string{}
	args := []any{}
	idx := 1
	add := func(col string, val any) {
		parts = append(parts, col+" = $"+strconv.Itoa(idx))
		args = append(args, val)
		idx++
	}
	if req.Content != nil {
		add("content", *req.Content)
	}
	if req.IsCorrect != nil {
		add("is_correct", *req.IsCorrect)
	}
	if req.SortOrder != nil {
		add("sort_order", *req.SortOrder)
	}
	if req.PointsWeight != nil {
		add("points_weight", *req.PointsWeight)
	}
	if len(parts) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"id": oid, "status": "no_change"})
		return
	}

	q := "UPDATE exam_question_options SET " + joinComma(parts) +
		" WHERE id = $" + strconv.Itoa(idx) +
		" AND tenant_id = $" + strconv.Itoa(idx+1)
	args = append(args, oid, tenantID)

	if _, err := a.db.ExecContext(r.Context(), q, args...); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "update_failed", "Could not update option", r)
		return
	}

	a.audit(r.Context(), &tenantID, auth.UserID, "options.update", "exam_question_option", oid, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": oid, "status": "updated"})
}

func (a *App) handleDeleteOption(w http.ResponseWriter, r *http.Request) {
	if !a.RequirePermission(w, r, "exams:write") {
		return
	}
	tenantID := a.RequireEffectiveTenant(w, r)
	if tenantID == "" {
		return
	}
	if !a.RequireCSRF(w, r) {
		return
	}
	oid := r.PathValue("optionId")

	auth := AuthFromContext(r.Context())
	questionID, ok := a.requireOptionWriteAccess(w, r, tenantID, auth, oid)
	if !ok {
		return
	}

	// Post-state validation: simulate the option set after this delete so
	// removing an option below MCQ minimum (2) or removing the only correct
	// answer is blocked instead of leaving an unscoreable question.
	existing := a.loadOptions(r, questionID)
	simulated := make([]questionOption, 0, len(existing))
	for _, o := range existing {
		if o.ID != oid {
			simulated = append(simulated, o)
		}
	}
	if errs, err := a.validateOptionsAfterMutation(r.Context(), tenantID, questionID, simulated); err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "validation_failed", "Could not validate options", r)
		return
	} else if len(errs) > 0 {
		writeValidationError(w, errs, r)
		return
	}

	res, err := a.db.ExecContext(r.Context(),
		`DELETE FROM exam_question_options WHERE id = $1 AND tenant_id = $2`,
		oid, tenantID,
	)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "delete_failed", "Could not delete option", r)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "Option not found", r)
		return
	}
	a.audit(r.Context(), &tenantID, auth.UserID, "options.delete", "exam_question_option", oid, r)
	writeJSON(w, http.StatusOK, map[string]any{"id": oid, "status": "deleted"})
}
