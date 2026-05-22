package app

import (
	"context"
)

// position_helpers — single source of truth for "where does the next
// thing go?" in the exam canvas. The frontend renders groups +
// standalone questions interleaved per section, ordered by a unified
// position field (group.display_order vs question.sort_order). For
// new inserts we need to compute a position that doesn't collide with
// existing siblings, otherwise the canvas falls back to id-based
// tie-breaks and the new item appears at unpredictable positions.
//
// The three scopes:
//
//   sectionTopPosition  — appending a group or standalone question to
//                         a section. Considers BOTH counters.
//   groupTopPosition    — appending a question into a group. Scoped
//                         to that group only.
//   examTopPosition     — appending a section-less question (rare).
//
// All return the next free integer (existing MAX + 1, or 0 if empty).

// nextSectionPosition returns MAX over both display_order in groups
// (same section) and sort_order in standalone questions (same
// section, group_id IS NULL), then +1. Use this when creating a new
// group OR a standalone question inside a section.
func nextSectionPosition(ctx context.Context, db dbExecer, sectionID string) int {
	if sectionID == "" {
		return 0
	}
	var pos int
	_ = db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(pos), -1) + 1 FROM (
			SELECT display_order AS pos
			  FROM exam_question_groups
			 WHERE section_id = $1
			UNION ALL
			SELECT sort_order AS pos
			  FROM exam_questions
			 WHERE section_id = $1 AND group_id IS NULL
		) t`, sectionID,
	).Scan(&pos)
	return pos
}

// nextGroupPosition returns MAX(sort_order) + 1 over questions
// belonging to the given group. Use when inserting a new question
// into an existing group.
func nextGroupPosition(ctx context.Context, db dbExecer, groupID string) int {
	if groupID == "" {
		return 0
	}
	var pos int
	_ = db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM exam_questions WHERE group_id = $1`,
		groupID,
	).Scan(&pos)
	return pos
}

// nextExamPosition is the legacy fallback for the rare case of a
// section-less standalone question (pre-migration data). Computes
// MAX(sort_order)+1 across the entire exam.
func nextExamPosition(ctx context.Context, db dbExecer, examID string) int {
	if examID == "" {
		return 0
	}
	var pos int
	_ = db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM exam_questions WHERE exam_id = $1`,
		examID,
	).Scan(&pos)
	return pos
}

// dbExecer is defined in archive_helpers.go and shared across
// helpers in this package; we just consume QueryRowContext here.

// resolveQuestionPosition returns the proper sort_order for a new
// question based on its container hierarchy:
//   - If groupID set: scoped to that group
//   - Else if sectionID set: section-unified (groups + standalones)
//   - Else: exam-wide fallback
func resolveQuestionPosition(ctx context.Context, db dbExecer, examID, sectionID, groupID string) int {
	if groupID != "" {
		return nextGroupPosition(ctx, db, groupID)
	}
	if sectionID != "" {
		return nextSectionPosition(ctx, db, sectionID)
	}
	return nextExamPosition(ctx, db, examID)
}
