-- 000017_section_mandatory_and_inline_kisi_kisi.sql
-- Phase 9.8 — Section becomes the mandatory container for every
-- exam question. Per the UX rewrite documented in the worktree
-- TASKS / AUDIT report:
--
--   1. Every exam must have at least one section. Backfill creates
--      "Section 1" for any exam without one.
--   2. exam_questions.section_id becomes NOT NULL after orphan rows
--      are reassigned to the exam's first section.
--   3. exam_question_groups.section_id is left nullable for now (the
--      group can still exist at root) but orphans are reassigned to
--      the first section so the new UI can render them inside a
--      section by default.
--
-- Forward-only. Dev data is allowed because the section is a visual
-- container; orphan rows do not lose data, they are reassigned.

-- 1) Backfill: every exam without a section gets a default "Section 1".
INSERT INTO exam_sections (id, tenant_id, exam_id, title, sort_order, created_at)
SELECT gen_random_uuid(), e.tenant_id, e.id, 'Section 1', 0, now()
  FROM exams e
 WHERE NOT EXISTS (SELECT 1 FROM exam_sections s WHERE s.exam_id = e.id);

-- 2) Reassign orphan questions to the exam's first section
--    (lowest sort_order, then oldest).
UPDATE exam_questions q
   SET section_id = (
        SELECT s.id FROM exam_sections s
         WHERE s.exam_id = q.exam_id
         ORDER BY s.sort_order ASC, s.created_at ASC
         LIMIT 1
   )
 WHERE q.section_id IS NULL
   AND EXISTS (SELECT 1 FROM exam_sections s WHERE s.exam_id = q.exam_id);

-- 3) Reassign orphan groups when the table actually has a section_id
--    column. Migration 000015 introduced this column — guard with
--    information_schema so re-running on partial schemas is safe.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_name = 'exam_question_groups' AND column_name = 'section_id'
  ) THEN
    UPDATE exam_question_groups g
       SET section_id = (
            SELECT s.id FROM exam_sections s
             WHERE s.exam_id = g.exam_id
             ORDER BY s.sort_order ASC, s.created_at ASC
             LIMIT 1
       )
     WHERE g.section_id IS NULL
       AND EXISTS (SELECT 1 FROM exam_sections s WHERE s.exam_id = g.exam_id);
  END IF;
END $$;

-- 4) Lock the invariant in: every question must carry a section_id.
--    The handler reassigns members before deleting a section, so the
--    ON DELETE SET NULL behavior on the FK is never triggered.
ALTER TABLE exam_questions ALTER COLUMN section_id SET NOT NULL;
