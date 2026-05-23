package app

import (
	"context"
	"database/sql"
	"strings"
)

type ensureQuestionKisiKisiResult struct {
	SlotID  string
	Created bool
}

func ensureQuestionKisiKisiLink(ctx context.Context, tx *sql.Tx, tenantID string, policy examAuthoringPolicy, blueprintID string, questionID string, draft map[string]any, position int) (ensureQuestionKisiKisiResult, error) {
	if !policy.UsesKisiKisi || blueprintID == "" || questionHasBlueprintSlot(ctx, tx, questionID) {
		return ensureQuestionKisiKisiResult{}, nil
	}
	item := kisiItemFromQuestionMap(questionID, draft, position)
	if stringsHasKOMPPrefix(item.CompetencyCode) {
		item.CompetencyCode = inferNextCompetencyCodeTx(ctx, tx, blueprintID, position)
	}
	var slotID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exam_blueprint_slots (
		    exam_blueprint_id, position,
		    competency_code, competency_description, materi, indikator,
		    cognitive_level, difficulty, question_type, points
		) VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10)
		RETURNING id::text`,
		blueprintID, position, item.CompetencyCode, item.CompetencyDescription, item.Materi, item.Indikator,
		item.CognitiveLevel, item.Difficulty, item.QuestionType, item.points,
	).Scan(&slotID); err != nil {
		return ensureQuestionKisiKisiResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE exam_questions SET blueprint_slot_id=$1, updated_at=now() WHERE id=$2 AND tenant_id=$3`, slotID, questionID, tenantID); err != nil {
		return ensureQuestionKisiKisiResult{}, err
	}
	markExamAIContextStale(ctx, tx, tenantID, policy.ExamID)
	return ensureQuestionKisiKisiResult{SlotID: slotID, Created: true}, nil
}

func updateExamBlueprintTotals(ctx context.Context, tx *sql.Tx, blueprintID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE exam_blueprints SET
		    total_slots = (SELECT COUNT(*) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
		    total_points = (SELECT COALESCE(SUM(points),0) FROM exam_blueprint_slots WHERE exam_blueprint_id=$1),
		    updated_at = now()
		WHERE id=$1`, blueprintID)
	return err
}

func stringsHasKOMPPrefix(s string) bool {
	return strings.HasPrefix(s, "KOMP-")
}
