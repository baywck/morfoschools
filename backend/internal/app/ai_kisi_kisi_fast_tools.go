package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ai_kisi_kisi_fast_tools.go — fast LLM-driven kisi-kisi application
// paths for inline magic. These intentionally avoid the older
// apply_blueprint_analysis / convert_questions_to_kisi_kisi flow for
// single-question edits because that flow is blueprint-wide, can
// replace existing blueprints, and is too slow for per-card UX.
//
// Model does only the semantic part (KD/Materi/Indikator/etc.). The
// backend deterministically appends a slot to the exam's blueprint and
// links the existing question via blueprint_slot_id.

type questionKisiKisiItem struct {
	QuestionID            string   `json:"questionId"`
	CompetencyCode        string   `json:"competencyCode"`
	CompetencyDescription string   `json:"competencyDescription"`
	Materi                string   `json:"materi"`
	Indikator             string   `json:"indikator"`
	CognitiveLevel        string   `json:"cognitiveLevel"`
	Difficulty            string   `json:"difficulty"`
	QuestionType          string   `json:"questionType"`
	Points                *float64 `json:"points"`
}

func (a *App) capApplyQuestionKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	var p struct {
		ExamID         string  `json:"examId"`
		CurriculumCode string  `json:"curriculumCode"`
		questionKisiKisiItem
	}
	_ = json.Unmarshal(args, &p)
	if p.CurriculumCode == "" {
		p.CurriculumCode = "merdeka"
	}
	if errMsg := validateKisiKisiItem(p.questionKisiKisiItem); errMsg != "" {
		return errValidationFailed("kisiKisi", errMsg), nil
	}
	confirm := confirmApplyQuestionKisiKisi(p.ExamID, p.CurriculumCode, []questionKisiKisiItem{p.questionKisiKisiItem}, false)
	return a.createProposal(ctx, sessionID, tenantID, userID, "apply_question_kisi_kisi", args, confirm)
}

func (a *App) capBulkApplyQuestionKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	sessionID, _ := ctx.Value(ctxKeySessionID{}).(string)
	var p struct {
		ExamID         string               `json:"examId"`
		CurriculumCode string               `json:"curriculumCode"`
		Replace        bool                 `json:"replace"`
		Items          []questionKisiKisiItem `json:"items"`
	}
	_ = json.Unmarshal(args, &p)
	if p.CurriculumCode == "" {
		p.CurriculumCode = "merdeka"
	}
	if len(p.Items) == 0 {
		return errValidationFailed("items", "minimal 1 item kisi-kisi required"), nil
	}
	if len(p.Items) > 60 {
		return errValidationFailed("items", "maksimal 60 item per call; split per-section untuk exam besar"), nil
	}
	for i, it := range p.Items {
		if errMsg := validateKisiKisiItem(it); errMsg != "" {
			return errValidationFailed(fmt.Sprintf("items[%d]", i), errMsg), nil
		}
	}
	confirm := confirmApplyQuestionKisiKisi(p.ExamID, p.CurriculumCode, p.Items, p.Replace)
	return a.createProposal(ctx, sessionID, tenantID, userID, "bulk_apply_question_kisi_kisi", args, confirm)
}

func validateKisiKisiItem(it questionKisiKisiItem) string {
	if !isUUID(it.QuestionID) {
		return "questionId harus UUID valid"
	}
	if strings.TrimSpace(it.Materi) == "" {
		return "materi wajib diisi"
	}
	if strings.TrimSpace(it.Indikator) == "" {
		return "indikator wajib diisi"
	}
	switch it.CognitiveLevel {
	case "C1", "C2", "C3", "C4", "C5", "C6":
	default:
		return "cognitiveLevel harus C1-C6"
	}
	switch it.Difficulty {
	case "mudah", "sedang", "sulit":
	default:
		return "difficulty harus mudah/sedang/sulit"
	}
	return ""
}

func confirmApplyQuestionKisiKisi(examID, curriculum string, items []questionKisiKisiItem, replace bool) string {
	var sb strings.Builder
	sb.WriteString("**Apply kisi-kisi ke soal**\n")
	fmt.Fprintf(&sb, "\n**Exam:** `%s`\n", examID)
	fmt.Fprintf(&sb, "**Kurikulum:** %s\n", curriculum)
	if replace {
		sb.WriteString("\n⚠ **Replace blueprint existing** sebelum membuat slot baru.\n")
	}
	fmt.Fprintf(&sb, "\n**%d slot akan dibuat + dilink ke soal:**\n", len(items))
	for i, it := range items {
		if i >= 12 {
			fmt.Fprintf(&sb, "  … dan %d slot lainnya\n", len(items)-12)
			break
		}
		parts := []string{}
		if it.CompetencyCode != "" { parts = append(parts, "KD="+it.CompetencyCode) }
		if it.Materi != "" { parts = append(parts, "Materi="+truncateConfirm(it.Materi, 36)) }
		if it.CognitiveLevel != "" { parts = append(parts, it.CognitiveLevel) }
		if it.Difficulty != "" { parts = append(parts, it.Difficulty) }
		if it.Indikator != "" { parts = append(parts, truncateConfirm(it.Indikator, 70)) }
		fmt.Fprintf(&sb, "  %d. Soal `%s` → %s\n", i+1, it.QuestionID, strings.Join(parts, " | "))
	}
	return sb.String()
}

func (a *App) execApplyQuestionKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID         string `json:"examId"`
		CurriculumCode string `json:"curriculumCode"`
		questionKisiKisiItem
	}
	_ = json.Unmarshal(args, &p)
	if p.CurriculumCode == "" { p.CurriculumCode = "merdeka" }
	return a.applyQuestionKisiKisiItems(ctx, tenantID, userID, p.ExamID, p.CurriculumCode, false, []questionKisiKisiItem{p.questionKisiKisiItem})
}

func (a *App) execBulkApplyQuestionKisiKisi(ctx context.Context, tenantID, userID string, args json.RawMessage) (string, error) {
	var p struct {
		ExamID         string                `json:"examId"`
		CurriculumCode string                `json:"curriculumCode"`
		Replace        bool                  `json:"replace"`
		Items          []questionKisiKisiItem `json:"items"`
	}
	_ = json.Unmarshal(args, &p)
	if p.CurriculumCode == "" { p.CurriculumCode = "merdeka" }
	return a.applyQuestionKisiKisiItems(ctx, tenantID, userID, p.ExamID, p.CurriculumCode, p.Replace, p.Items)
}

func (a *App) applyQuestionKisiKisiItems(ctx context.Context, tenantID, userID, examID, curriculumCode string, replace bool, items []questionKisiKisiItem) (string, error) {
	if denied := a.checkAIExamAccess(ctx, tenantID, userID, examID, ActionWrite); denied != "" {
		return denied, nil
	}
	if len(items) == 0 {
		return errValidationFailed("items", "minimal 1 item"), nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return "", err }
	defer tx.Rollback()

	if replace {
		if _, err := tx.ExecContext(ctx, `DELETE FROM exam_blueprints WHERE exam_id = $1 AND tenant_id = $2`, examID, tenantID); err != nil {
			return "", err
		}
	}

	bpID, err := ensureExamBlueprintTx(ctx, tx, tenantID, examID, curriculumCode)
	if err != nil { return "", err }

	// Find current max position so appends don't collide.
	var pos int
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM exam_blueprint_slots WHERE exam_blueprint_id = $1`, bpID).Scan(&pos)

	linked := 0
	for _, it := range items {
		// Verify question belongs to exam + tenant and infer missing type/points.
		var qType string
		var qPoints float64
		if err := tx.QueryRowContext(ctx,
			`SELECT question_type, points FROM exam_questions WHERE id = $1 AND exam_id = $2 AND tenant_id = $3`,
			it.QuestionID, examID, tenantID,
		).Scan(&qType, &qPoints); err != nil {
			if err == sql.ErrNoRows { return errEntityNotFound("question", "questionId", it.QuestionID), nil }
			return "", err
		}
		if it.QuestionType == "" { it.QuestionType = qType }
		points := qPoints
		if it.Points != nil { points = *it.Points }

		var slotID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO exam_blueprint_slots (
			    exam_blueprint_id, position,
			    competency_code, competency_description, materi, indikator,
			    cognitive_level, difficulty, question_type, points
			) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10)
			RETURNING id::text`,
			bpID, pos, it.CompetencyCode, it.CompetencyDescription, it.Materi, it.Indikator,
			it.CognitiveLevel, it.Difficulty, it.QuestionType, points,
		).Scan(&slotID); err != nil {
			return "", err
		}
		pos++
		if _, err := tx.ExecContext(ctx,
			`UPDATE exam_questions SET blueprint_slot_id = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`,
			slotID, it.QuestionID, tenantID,
		); err != nil { return "", err }
		linked++
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE exam_blueprints SET
		    total_slots = (SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
		    total_points = (SELECT COALESCE(SUM(points),0) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
		    updated_at = now()
		WHERE id=$1`, bpID); err != nil { return "", err }

	if _, err := tx.ExecContext(ctx, `UPDATE exams SET uses_kisi_kisi = true, updated_at = now() WHERE id=$1 AND tenant_id=$2`, examID, tenantID); err != nil { return "", err }

	if err := tx.Commit(); err != nil { return "", err }
	a.auditAI(ctx, tenantID, userID, "exam_blueprints.fast_apply_kisi_kisi", "exam_blueprint", bpID)
	return fmt.Sprintf(`{"success":true,"message":"Kisi-kisi diterapkan ke %d soal","blueprintId":"%s","linkedQuestions":%d}`, linked, bpID, linked), nil
}

func ensureExamBlueprintTx(ctx context.Context, tx *sql.Tx, tenantID, examID, curriculumCode string) (string, error) {
	var bpID string
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM exam_blueprints WHERE exam_id=$1 AND tenant_id=$2 ORDER BY created_at DESC LIMIT 1`, examID, tenantID).Scan(&bpID)
	if err == nil { return bpID, nil }
	if err != sql.ErrNoRows { return "", err }

	var curriculumID string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM curricula WHERE code=$1`, curriculumCode).Scan(&curriculumID); err != nil {
		return "", err
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exam_blueprints (
		    tenant_id, exam_id, source_template_id, source_template_version,
		    created_via, title, curriculum_id, blueprint_type,
		    strict_coverage, status
		) VALUES ($1,$2,NULL,NULL,'ai_fast_extract','Kisi-Kisi (auto)', $3, 'reguler', false, 'draft')
		RETURNING id::text`,
		tenantID, examID, curriculumID,
	).Scan(&bpID); err != nil { return "", err }
	return bpID, nil
}
