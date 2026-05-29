package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type agentCreateBlueprintSlotsArgs struct {
	ExamID      string                    `json:"examId"`
	Topic       string                    `json:"topic,omitempty"`
	Slots       []agentBlueprintSlotDraft `json:"slots"`
	Warnings    []string                  `json:"warnings,omitempty"`
	CPStatus    string                    `json:"cpStatus,omitempty"`
	CPSource    string                    `json:"cpSource,omitempty"`
	Confirmable bool                      `json:"confirmable"`
}

func (a *App) validateAgentCreateBlueprintSlotsArgs(ctx context.Context, tenantID, userID string, args agentCreateBlueprintSlotsArgs) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(args.ExamID) == "" {
		fields["examId"] = "Exam aktif wajib tersedia"
	} else if !a.agentCanWriteExam(ctx, tenantID, userID, args.ExamID) {
		fields["examId"] = "Anda tidak punya akses tulis ke exam ini"
	}
	if len(args.Slots) == 0 {
		fields["slots"] = "Minimal satu slot kisi-kisi wajib dibuat"
	}
	if len(args.Slots) > 100 {
		fields["slots"] = "Maksimal 100 slot dalam satu proposal"
	}
	for i, slot := range args.Slots {
		issues := validateKurikulumMerdekaBlueprintSlot(slot)
		if hasBlockingCurriculumIssues(issues) {
			fields[fmt.Sprintf("slots.%d", i)] = "Slot kisi-kisi belum memenuhi aturan Kurikulum Merdeka"
		}
	}
	return fields
}

func (a *App) buildAgentCreateBlueprintSlotsPreview(ctx context.Context, tenantID string, args agentCreateBlueprintSlotsArgs) string {
	var title string
	_ = a.db.QueryRowContext(ctx, `SELECT title FROM exams WHERE id=$1 AND tenant_id=$2`, args.ExamID, tenantID).Scan(&title)
	var b strings.Builder
	b.WriteString("Proposal: buat kisi-kisi")
	if title != "" {
		b.WriteString(" untuk exam \"")
		b.WriteString(title)
		b.WriteString("\"")
	}
	b.WriteString(".\n\n")
	if args.Topic != "" {
		b.WriteString("Topik: ")
		b.WriteString(args.Topic)
		b.WriteString("\n")
	}
	if args.CPStatus != "" {
		b.WriteString("CP context: ")
		b.WriteString(args.CPStatus)
		if args.CPSource != "" {
			b.WriteString(" / ")
			b.WriteString(args.CPSource)
		}
		b.WriteString("\n")
	}
	if len(args.Warnings) > 0 {
		b.WriteString("\nPeringatan:\n")
		for _, warning := range args.Warnings {
			b.WriteString("- ")
			b.WriteString(warning)
			b.WriteString("\n")
		}
	}
	b.WriteString("\nSlot yang akan dibuat:\n")
	for _, slot := range args.Slots {
		b.WriteString(fmt.Sprintf("%d. %s · %s · %s · %s\n", slot.Position, slot.MateriPokok, normalizeCognitiveLevel(slot.CognitiveLevel), slot.QuestionType, slot.IndikatorSoal))
	}
	b.WriteString("\nKonfirmasi untuk menyimpan kisi-kisi ini.")
	return b.String()
}

func (a *App) executeAgentCreateBlueprintSlots(ctx context.Context, tenantID, userID string, raw json.RawMessage) (agentWorkflowResult, error) {
	var args agentCreateBlueprintSlotsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return agentWorkflowResult{}, err
	}
	if fields := a.validateAgentCreateBlueprintSlotsArgs(ctx, tenantID, userID, args); len(fields) > 0 {
		return agentWorkflowResult{}, fmt.Errorf("invalid blueprint slot proposal")
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return agentWorkflowResult{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE exams SET uses_kisi_kisi=true, updated_at=now() WHERE id=$1 AND tenant_id=$2`, args.ExamID, tenantID); err != nil {
		return agentWorkflowResult{}, err
	}
	blueprintID, err := a.ensureExamBlueprintContainer(ctx, tx, tenantID, args.ExamID, "Kisi-Kisi")
	if err != nil {
		return agentWorkflowResult{}, err
	}
	var nextPosition int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position),0)+1 FROM exam_blueprint_slots WHERE blueprint_id=$1`, blueprintID).Scan(&nextPosition); err != nil {
		return agentWorkflowResult{}, err
	}
	created := 0
	for i, slot := range args.Slots {
		position := slot.Position
		if position <= 0 {
			position = nextPosition + i
		}
		points := float64(slot.Points)
		if points <= 0 {
			points = 1
		}
		kelas := ""
		var grade sql.NullString
		_ = tx.QueryRowContext(ctx, `SELECT grade_level FROM exams WHERE id=$1 AND tenant_id=$2`, args.ExamID, tenantID).Scan(&grade)
		if grade.Valid {
			kelas = grade.String
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO exam_blueprint_slots (
				blueprint_id, position, points, materi, indikator, cognitive_level, difficulty, question_type,
				capaian_pembelajaran, elemen_cp, tujuan_pembelajaran, materi_pokok, kelas, indikator_soal
			) VALUES ($1,$2,$3,$4,$5,$6,'sedang',$7,$8,$9,$10,$11,NULLIF($12,''),$13)`,
			blueprintID, position, points, slot.MateriPokok, slot.IndikatorSoal, normalizeCognitiveLevel(slot.CognitiveLevel), normalizeQuestionType(slot.QuestionType),
			slot.CapaianPembelajaran, slot.ElemenCP, slot.TujuanPembelajaran, slot.MateriPokok, kelas, slot.IndikatorSoal,
		)
		if err != nil {
			return agentWorkflowResult{}, err
		}
		created++
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE exam_blueprints b SET total_slots=s.cnt, total_points=s.points, updated_at=now()
		FROM (SELECT COUNT(*)::int cnt, COALESCE(SUM(points),0) points FROM exam_blueprint_slots WHERE blueprint_id=$1) s
		WHERE b.id=$1`, blueprintID); err != nil {
		return agentWorkflowResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return agentWorkflowResult{}, err
	}
	return agentWorkflowResult{Workflow: agentWorkflowCreateBlueprintSlots, Data: map[string]any{"examId": args.ExamID, "blueprintId": blueprintID, "createdSlots": created}}, nil
}
