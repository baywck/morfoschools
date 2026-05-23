package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type examAIContextIndex struct {
	TenantID    string         `json:"-"`
	ExamID      string         `json:"-"`
	ContentHash string         `json:"contentHash"`
	Summary     map[string]any `json:"summary"`
	Stale       bool           `json:"stale"`
}

func markExamAIContextStale(ctx context.Context, tx *sql.Tx, tenantID, examID string) {
	if strings.TrimSpace(examID) == "" {
		return
	}
	_, _ = tx.ExecContext(ctx, `
		UPDATE exam_ai_context_indexes
		   SET stale=true, updated_at=now()
		 WHERE tenant_id=$1 AND exam_id=$2`, tenantID, examID)
}

func loadOrRebuildExamAIContextIndex(ctx context.Context, db *sql.DB, tenantID, examID string) (examAIContextIndex, error) {
	currentHash, err := computeExamAIContextHash(ctx, db, tenantID, examID)
	if err != nil {
		return examAIContextIndex{}, err
	}
	var idx examAIContextIndex
	var summaryRaw []byte
	err = db.QueryRowContext(ctx, `
		SELECT content_hash, summary, stale
		  FROM exam_ai_context_indexes
		 WHERE tenant_id=$1 AND exam_id=$2`, tenantID, examID).Scan(&idx.ContentHash, &summaryRaw, &idx.Stale)
	if err == nil {
		_ = json.Unmarshal(summaryRaw, &idx.Summary)
		idx.TenantID = tenantID
		idx.ExamID = examID
		if !idx.Stale && idx.ContentHash == currentHash {
			return idx, nil
		}
	} else if err != sql.ErrNoRows {
		return examAIContextIndex{}, err
	}
	return rebuildExamAIContextIndex(ctx, db, tenantID, examID, currentHash)
}

func rebuildExamAIContextIndex(ctx context.Context, db *sql.DB, tenantID, examID, contentHash string) (examAIContextIndex, error) {
	summary := map[string]any{}
	var questionCount, groupCount, stimulusCount int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exam_questions WHERE tenant_id=$1 AND exam_id=$2`, tenantID, examID).Scan(&questionCount)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM exam_question_groups WHERE tenant_id=$1 AND exam_id=$2`, tenantID, examID).Scan(&groupCount)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stimuli WHERE tenant_id=$1 AND parent_exam_id=$2`, tenantID, examID).Scan(&stimulusCount)
	summary["questionCount"] = questionCount
	summary["groupCount"] = groupCount
	summary["stimulusCount"] = stimulusCount

	qTypes := map[string]int{}
	rows, err := db.QueryContext(ctx, `
		SELECT question_type, COUNT(*)
		  FROM exam_questions
		 WHERE tenant_id=$1 AND exam_id=$2
		 GROUP BY question_type`, tenantID, examID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var k string
			var c int
			if rows.Scan(&k, &c) == nil {
				qTypes[k] = c
			}
		}
	}
	summary["questionTypeCounts"] = qTypes

	kdCodes := make([]string, 0, 16)
	cognitive := map[string]int{}
	difficulty := map[string]int{}
	rows, err = db.QueryContext(ctx, `
		SELECT COALESCE(s.competency_code,''), COALESCE(s.cognitive_level,''), COALESCE(s.difficulty,'')
		  FROM exam_blueprint_slots s
		  JOIN exam_blueprints b ON b.id=s.exam_blueprint_id
		 WHERE b.tenant_id=$1 AND b.exam_id=$2
		 ORDER BY s.position ASC`, tenantID, examID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var code, cog, diff string
			if rows.Scan(&code, &cog, &diff) != nil {
				continue
			}
			if strings.TrimSpace(code) != "" {
				kdCodes = append(kdCodes, code)
			}
			if strings.TrimSpace(cog) != "" {
				cognitive[cog]++
			}
			if strings.TrimSpace(diff) != "" {
				difficulty[diff]++
			}
		}
	}
	pattern := detectKDPattern(kdCodes)
	summary["kdCodes"] = kdCodes
	summary["kdPattern"] = map[string]any{"kind": pattern.Kind, "prefix": pattern.Prefix, "maxSuffix": pattern.MaxSuffix}
	summary["cognitiveDistribution"] = cognitive
	summary["difficultyDistribution"] = difficulty
	summary["style"] = map[string]any{
		"defaultQuestionType": "multiple_choice",
		"stemStyle":           "contextual_2_4_sentences",
		"optionStyle":         "homogeneous_plausible_4_options",
		"explanation":         "required_short",
	}

	b, _ := json.Marshal(summary)
	_, err = db.ExecContext(ctx, `
		INSERT INTO exam_ai_context_indexes (tenant_id, exam_id, content_hash, summary, stale, indexed_at, updated_at)
		VALUES ($1,$2,$3,$4,false,now(),now())
		ON CONFLICT (tenant_id, exam_id) DO UPDATE SET
		    content_hash=EXCLUDED.content_hash,
		    summary=EXCLUDED.summary,
		    stale=false,
		    indexed_at=now(),
		    updated_at=now()`, tenantID, examID, contentHash, b)
	if err != nil {
		return examAIContextIndex{}, err
	}
	return examAIContextIndex{TenantID: tenantID, ExamID: examID, ContentHash: contentHash, Summary: summary, Stale: false}, nil
}

func computeExamAIContextHash(ctx context.Context, db *sql.DB, tenantID, examID string) (string, error) {
	parts := make([]string, 0, 6)
	queries := []string{
		`SELECT COUNT(*)::text || ':' || COALESCE(MAX(updated_at)::text,'') FROM exam_questions WHERE tenant_id=$1 AND exam_id=$2`,
		`SELECT COUNT(*)::text || ':' || COALESCE(MAX(updated_at)::text,'') FROM exam_question_groups WHERE tenant_id=$1 AND exam_id=$2`,
		`SELECT COUNT(*)::text || ':' || COALESCE(MAX(updated_at)::text,'') FROM stimuli WHERE tenant_id=$1 AND parent_exam_id=$2`,
		`SELECT COUNT(*)::text || ':' || COALESCE(MAX(s.updated_at)::text,'') FROM exam_blueprint_slots s JOIN exam_blueprints b ON b.id=s.exam_blueprint_id WHERE b.tenant_id=$1 AND b.exam_id=$2`,
	}
	for _, q := range queries {
		var part string
		if err := db.QueryRowContext(ctx, q, tenantID, examID).Scan(&part); err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", tenantID, examID, strings.Join(parts, "|"))))
	return hex.EncodeToString(sum[:]), nil
}
