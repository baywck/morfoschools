-- 000013_exams_questions.sql
-- Phase 9 Exams: extends the skeleton from 000006_exams.sql with the
-- flexibility needed for real authoring.
--
-- This migration is purely additive (ALTER TABLE ADD COLUMN IF NOT EXISTS).
-- It does NOT modify the original 000006 schema beyond adding new columns
-- and supporting indexes, so existing data is preserved.

-- --- Exams: per-question shuffle + author-restricted flag ---

-- The existing schema has shuffle_questions + shuffle_options at exam level
-- (boolean defaults). We add a per-question override mechanism so an exam
-- author can mark "this question's options must always appear in order"
-- (e.g. for chronological ordering questions) even when exam.shuffle_options
-- is true. Resolution rule (in app code): if question.shuffle_options_override
-- IS NOT NULL, it wins; otherwise inherit from exam.shuffle_options.
ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS shuffle_options_override BOOLEAN;

-- Author bookkeeping: who created the question, useful for teacher RBAC
-- (a teacher can edit only questions they authored unless they own the exam).
ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id);

-- MCQ flexible scoring mode
--   correct_all  : student must select EXACTLY all correct options to score
--                  full points (default; back-compat with binary correct_answer)
--   correct_one  : selecting any one correct option scores full points
--   percentage   : score = points * (correct_selected / total_correct)
--                  with no penalty for wrong selections by default; per-option
--                  weight via exam_question_options.points_weight when set
ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS scoring_mode TEXT NOT NULL DEFAULT 'correct_all'
        CHECK (scoring_mode IN ('correct_all', 'correct_one', 'percentage'));

-- Wrong-answer penalty for percentage mode. NULL = no penalty.
-- Stored as a fraction of points to subtract per wrong selection,
-- bounded so the question can't go negative below 0.
ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS wrong_penalty_pct NUMERIC
        CHECK (wrong_penalty_pct IS NULL OR (wrong_penalty_pct >= 0 AND wrong_penalty_pct <= 1));

-- Cached count of correct options. Trigger maintains this so the scoring
-- code doesn't have to count on every grade pass. Can be NULL during
-- authoring before any option is marked correct.
ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS correct_count INTEGER NOT NULL DEFAULT 0;

-- Question position alias for ordering when sort_order ties (LLM-generated
-- batches commonly leave sort_order at 0 then re-number later).
CREATE INDEX IF NOT EXISTS idx_exam_questions_section_sort
    ON exam_questions(section_id, sort_order, id);

-- Content-hash fingerprint for in-flight + committed dedup. Computed by app
-- code as md5(lower(trim(content))) so the AI dupe-guard can ask
-- "have I already proposed this question text in this exam?" cheaply.
ALTER TABLE exam_questions
    ADD COLUMN IF NOT EXISTS content_hash TEXT;
CREATE INDEX IF NOT EXISTS idx_exam_questions_exam_hash
    ON exam_questions(exam_id, content_hash) WHERE content_hash IS NOT NULL;

-- --- Question options: per-option weight + structured order ---

-- Per-option point weight for percentage mode. NULL means equal-share
-- (1.0 / correct_count for correct options, 0 for wrong). Sum of weights
-- across correct options should equal 1.0 when explicitly set.
ALTER TABLE exam_question_options
    ADD COLUMN IF NOT EXISTS points_weight NUMERIC
        CHECK (points_weight IS NULL OR (points_weight >= 0 AND points_weight <= 1));

-- Position alias: sort_order plus a tiebreaker. Same rationale as for
-- exam_questions.sort_order.
CREATE INDEX IF NOT EXISTS idx_exam_question_options_question_sort
    ON exam_question_options(question_id, sort_order, id);

-- --- Gate windows: rename password concept to access_code ---
--
-- The existing column is `password` which is misleading: it's a per-window
-- access code, not a credential. Add `access_code` and backfill from the
-- old column. Do NOT drop the old column in this migration so a rollback
-- to 000012 still has the data.
ALTER TABLE exam_gate_windows
    ADD COLUMN IF NOT EXISTS access_code TEXT;

UPDATE exam_gate_windows
   SET access_code = password
 WHERE password IS NOT NULL AND access_code IS NULL;

-- Helpful timeline lookups: "is this exam takeable right now?" needs an
-- index that can answer it without a full scan.
CREATE INDEX IF NOT EXISTS idx_exam_gate_windows_active
    ON exam_gate_windows(exam_id, opens_at, closes_at);

-- --- Trigger: maintain exam_questions.correct_count ---

-- Recompute on insert/update/delete of options. We keep the function in
-- a fixed search_path to avoid CVE-2018-1058-class issues.
CREATE OR REPLACE FUNCTION fn_exam_question_recount_correct()
RETURNS TRIGGER AS $$
DECLARE
    qid UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        qid := OLD.question_id;
    ELSE
        qid := NEW.question_id;
    END IF;
    UPDATE exam_questions
       SET correct_count = (
           SELECT COUNT(*) FROM exam_question_options
            WHERE question_id = qid AND is_correct = true
       )
     WHERE id = qid;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql SET search_path = pg_catalog, public;

DROP TRIGGER IF EXISTS trg_exam_question_options_recount ON exam_question_options;
CREATE TRIGGER trg_exam_question_options_recount
    AFTER INSERT OR UPDATE OF is_correct OR DELETE
    ON exam_question_options
    FOR EACH ROW
    EXECUTE FUNCTION fn_exam_question_recount_correct();

-- --- Teacher-subject RBAC support ---
-- The teacher_subjects table already exists from migration 000007. Phase 9
-- handlers will read it directly to enforce "teacher can only author exams
-- for assigned subjects". No schema change needed here, just a comment for
-- future reference.
