package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

type agentContextPack struct {
	SessionID string                `json:"sessionId,omitempty"`
	ExamID    string                `json:"examId,omitempty"`
	Exam      agentContextExam      `json:"exam,omitempty"`
	Blueprint agentContextBlueprint `json:"blueprint,omitempty"`
	Recent    []agentContextMessage `json:"recentMessages,omitempty"`
	Notes     []string              `json:"notes,omitempty"`
}

type agentContextExam struct {
	SubjectName string `json:"subjectName,omitempty"`
	GradeLevel  string `json:"gradeLevel,omitempty"`
	Phase       string `json:"phase,omitempty"`
	CPStatus    string `json:"cpStatus,omitempty"`
	CPSource    string `json:"cpSource,omitempty"`
}

type agentContextBlueprint struct {
	ExistingSlotCount      int            `json:"existingSlotCount"`
	ByElement              map[string]int `json:"byElement,omitempty"`
	ByCognitiveLevel       map[string]int `json:"byCognitiveLevel,omitempty"`
	ByQuestionType         map[string]int `json:"byQuestionType,omitempty"`
	RecentMaterials        []string       `json:"recentMaterials,omitempty"`
	RecentIndicators       []string       `json:"recentIndicators,omitempty"`
	PotentialDuplicateHint []string       `json:"potentialDuplicateHints,omitempty"`
}

type agentContextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (a *App) buildAgentContextPack(ctx context.Context, tenantID, sessionID string, active map[string]string) agentContextPack {
	pack := agentContextPack{SessionID: sessionID}
	if active != nil {
		pack.ExamID = strings.TrimSpace(active["examId"])
	}
	pack.Recent = a.loadAgentRecentMessages(ctx, sessionID, 16, 2200)
	if pack.ExamID != "" {
		if ctxResp, err := a.ensureExamCurriculumContext(ctx, tenantID, pack.ExamID); err == nil {
			pack.Exam = agentContextExam{SubjectName: ctxResp.SubjectName, GradeLevel: ctxResp.GradeLevel, Phase: ctxResp.Phase, CPStatus: ctxResp.Status, CPSource: ctxResp.Source}
			for _, warning := range ctxResp.Warnings {
				pack.Notes = append(pack.Notes, "CP warning: "+warning)
			}
		}
		pack.Blueprint = a.loadAgentBlueprintContext(ctx, tenantID, pack.ExamID)
	}
	return pack
}

func (a *App) agentContextPackJSON(ctx context.Context, tenantID, sessionID string, active map[string]string) string {
	return mustJSON(a.buildAgentContextPack(ctx, tenantID, sessionID, active))
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (a *App) loadAgentRecentMessages(ctx context.Context, sessionID string, limit, maxChars int) []agentContextMessage {
	if sessionID == "" || a.db == nil {
		return nil
	}
	rows, err := a.db.QueryContext(ctx, `SELECT role, content FROM ai_messages WHERE session_id=$1 AND role IN ('user','assistant') ORDER BY created_at DESC LIMIT $2`, sessionID, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var reversed []agentContextMessage
	for rows.Next() {
		var m agentContextMessage
		if err := rows.Scan(&m.Role, &m.Content); err == nil {
			m.Content = truncateForPrompt(m.Content, maxChars)
			reversed = append(reversed, m)
		}
	}
	out := make([]agentContextMessage, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		out = append(out, reversed[i])
	}
	return out
}

func (a *App) loadAgentBlueprintContext(ctx context.Context, tenantID, examID string) agentContextBlueprint {
	bp := agentContextBlueprint{ByElement: map[string]int{}, ByCognitiveLevel: map[string]int{}, ByQuestionType: map[string]int{}}
	if a.db == nil || examID == "" {
		return bp
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(TRIM(s.elemen_cp), ''), 'unknown') AS elemen_cp,
		       COALESCE(NULLIF(TRIM(s.cognitive_level), ''), 'unknown') AS cognitive_level,
		       COALESCE(NULLIF(TRIM(s.question_type), ''), 'unknown') AS question_type,
		       NULLIF(TRIM(COALESCE(s.materi_pokok, s.materi, '')), '') AS materi,
		       NULLIF(TRIM(COALESCE(s.indikator_soal, s.indikator, '')), '') AS indikator
		FROM exam_blueprint_slots s
		JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		WHERE b.tenant_id=$1 AND b.exam_id=$2
		ORDER BY s.position ASC
	`, tenantID, examID)
	if err != nil {
		return bp
	}
	defer rows.Close()
	seenMaterial := map[string]bool{}
	seenIndicator := map[string]bool{}
	for rows.Next() {
		var element, level, qType string
		var material, indicator sql.NullString
		if err := rows.Scan(&element, &level, &qType, &material, &indicator); err != nil {
			continue
		}
		bp.ExistingSlotCount++
		bp.ByElement[element]++
		bp.ByCognitiveLevel[level]++
		bp.ByQuestionType[qType]++
		if material.Valid {
			m := strings.TrimSpace(material.String)
			key := strings.ToLower(m)
			if m != "" && !seenMaterial[key] && len(bp.RecentMaterials) < 20 {
				seenMaterial[key] = true
				bp.RecentMaterials = append(bp.RecentMaterials, truncateForPrompt(m, 180))
			} else if m != "" && seenMaterial[key] && len(bp.PotentialDuplicateHint) < 10 {
				bp.PotentialDuplicateHint = append(bp.PotentialDuplicateHint, "materi repeated: "+truncateForPrompt(m, 120))
			}
		}
		if indicator.Valid {
			ind := strings.TrimSpace(indicator.String)
			key := strings.ToLower(ind)
			if ind != "" && !seenIndicator[key] && len(bp.RecentIndicators) < 20 {
				seenIndicator[key] = true
				bp.RecentIndicators = append(bp.RecentIndicators, truncateForPrompt(ind, 220))
			} else if ind != "" && seenIndicator[key] && len(bp.PotentialDuplicateHint) < 10 {
				bp.PotentialDuplicateHint = append(bp.PotentialDuplicateHint, "indikator repeated: "+truncateForPrompt(ind, 120))
			}
		}
	}
	return bp
}
