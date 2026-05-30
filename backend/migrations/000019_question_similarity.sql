-- 000019_question_similarity.sql
-- Phase 9.13 — robust duplicate prevention for AI-generated questions.
--
-- Background: AI authoring on exams with 50+ questions occasionally
-- proposes near-duplicates because the active-context prompt is
-- token-budgeted (LIMIT 20) and our existing content_hash is exact-
-- match only. Paraphrases ("Apa ibu kota Indonesia?" vs "Sebutkan
-- ibu kota Indonesia") slip past md5(lower(trim(...))) and create
-- duplicate-feeling rows.
--
-- This migration adds:
--   1. pg_trgm extension — postgres-native trigram similarity for
--      cheap fuzzy matching against accepted questions.
--   2. content_normalized column — pre-computed canonical form used
--      for both stricter exact match AND trigram similarity input.
--      Computed via app-side normalizeQuestionContent() before insert.
--   3. GIN index over content_normalized for sub-100ms similarity
--      lookup even on 1000-question exams.
--
-- Reversibility: dropping the column + index restores the prior
-- state. The original content_hash column is untouched.

-- pg_trgm is part of contrib but not enabled by default; safe IF NOT
-- EXISTS in case another migration enabled it earlier.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Pre-computed normalized form. Populated by the app layer at insert
-- + update time using normalizeQuestionContent(). Keeping this as a
-- separate column (rather than a function index) lets us evolve the
-- normalization rules without rebuilding the entire index.
ALTER TABLE exam_questions
  ADD COLUMN IF NOT EXISTS content_normalized text;

-- Backfill existing rows with a SQL-native approximation. The app
-- layer will overwrite on next update with the canonical form.
UPDATE exam_questions
   SET content_normalized = lower(
         regexp_replace(
           regexp_replace(content, '<[^>]+>', ' ', 'g'),
           '[[:space:][:punct:]]+', ' ', 'g'
         )
       )
 WHERE content_normalized IS NULL;

-- Trigram GIN index for fast similarity ranking. Scoped per-exam at
-- query time via WHERE exam_id = ?, the index covers the similarity
-- ordering across all rows in the exam efficiently.
CREATE INDEX IF NOT EXISTS idx_exam_questions_normalized_trgm
  ON exam_questions
  USING gin (content_normalized gin_trgm_ops);
