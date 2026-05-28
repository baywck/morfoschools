package app

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

const (
	kdPatternNumericDot = "numeric_dot"
	kdPatternUnknown    = "unknown"
)

type examAuthoringPolicy struct {
	TenantID      string
	ExamID        string
	UsesKisiKisi  bool
	KDPattern     kdPattern
	QuestionRules generatedQuestionRules
}

type kdPattern struct {
	Kind      string
	Prefix    string
	MaxSuffix int
}

type generatedQuestionRules struct {
	DefaultQuestionType       string
	MinStandaloneWords        int
	MinStandaloneSent         int
	RequireExplanation        bool
	DefaultOptionCount        int
	AllowConstructedResponse  bool
	RequireHomogeneousOptions bool
}

func defaultExamAuthoringPolicy(tenantID, examID string) examAuthoringPolicy {
	return examAuthoringPolicy{
		TenantID:     tenantID,
		ExamID:       examID,
		UsesKisiKisi: false,
		KDPattern:    detectKDPattern(nil),
		QuestionRules: generatedQuestionRules{
			DefaultQuestionType:       "multiple_choice",
			MinStandaloneWords:        35,
			MinStandaloneSent:         2,
			RequireExplanation:        true,
			DefaultOptionCount:        5,
			AllowConstructedResponse:  false,
			RequireHomogeneousOptions: true,
		},
	}
}

func loadExamAuthoringPolicy(ctx context.Context, tx *sql.Tx, tenantID, examID string) (examAuthoringPolicy, error) {
	p := defaultExamAuthoringPolicy(tenantID, examID)
	if examID == "" {
		p.KDPattern = detectKDPattern(nil)
		return p, nil
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(uses_kisi_kisi,false) FROM exams WHERE id=$1 AND tenant_id=$2`, examID, tenantID).Scan(&p.UsesKisiKisi); err != nil {
		return p, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT s.competency_code
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id = s.exam_blueprint_id
		 WHERE b.exam_id=$1 AND b.tenant_id=$2
		   AND s.competency_code IS NOT NULL
		   AND s.competency_code <> ''
		 ORDER BY s.position ASC`, examID, tenantID)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	codes := make([]string, 0, 16)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err == nil {
			codes = append(codes, code)
		}
	}
	p.KDPattern = detectKDPattern(codes)
	return p, nil
}

func detectKDPattern(codes []string) kdPattern {
	p := kdPattern{Kind: kdPatternUnknown}
	for _, raw := range codes {
		code := strings.TrimSpace(raw)
		if code == "" || strings.HasPrefix(code, "KOMP-") {
			continue
		}
		lastDot := strings.LastIndex(code, ".")
		if lastDot < 0 || lastDot >= len(code)-1 {
			continue
		}
		n, err := strconv.Atoi(code[lastDot+1:])
		if err != nil {
			continue
		}
		if n > p.MaxSuffix {
			p.Kind = kdPatternNumericDot
			p.Prefix = code[:lastDot+1]
			p.MaxSuffix = n
		}
	}
	return p
}

func (p examAuthoringPolicy) nextCompetencyCode(fallbackPos int) string {
	if p.KDPattern.Kind == kdPatternNumericDot && p.KDPattern.Prefix != "" && p.KDPattern.MaxSuffix > 0 {
		return fmt.Sprintf("%s%d", p.KDPattern.Prefix, p.KDPattern.MaxSuffix+1)
	}
	return fallbackCompetencyCode(fallbackPos)
}

func inferNextCompetencyCodeTx(ctx context.Context, tx *sql.Tx, blueprintID string, fallbackPos int) string {
	rows, err := tx.QueryContext(ctx, `
		SELECT competency_code
		  FROM exam_blueprint_slots
		 WHERE exam_blueprint_id=$1
		   AND competency_code IS NOT NULL
		   AND competency_code <> ''
		 ORDER BY position ASC`, blueprintID)
	if err != nil {
		return fallbackCompetencyCode(fallbackPos)
	}
	defer rows.Close()
	codes := make([]string, 0, 16)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err == nil {
			codes = append(codes, code)
		}
	}
	return inferNextCompetencyCode(codes, fallbackPos)
}

func inferNextCompetencyCode(codes []string, fallbackPos int) string {
	pattern := detectKDPattern(codes)
	return examAuthoringPolicy{KDPattern: pattern}.nextCompetencyCode(fallbackPos)
}

func fallbackCompetencyCode(pos int) string {
	return fmt.Sprintf("KD-%d", pos+1)
}
