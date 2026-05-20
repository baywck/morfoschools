-- 000016_kisi_kisi_toggle_and_stimulus.sql
-- Phase 9.7 — introduce the 3-axis authoring model: kisi-kisi boolean
-- toggle on exams, AKM derived from exam_blueprints.blueprint_type at
-- the read site, and a universal stimulus axis on exam_questions.
--
-- Per ADR-0012. The earlier 3-mode authoring_mode discriminator was
-- never shipped — the column was added and removed within the same
-- merge cycle, so this migration introduces uses_kisi_kisi as the
-- canonical flag and never references the dropped column.

-- New flag (boolean axis). Default false; backfill below promotes any
-- exam that already has a blueprint to the on state so existing 9.5
-- exams keep their kisi-kisi-aware behavior after migration.
ALTER TABLE exams ADD COLUMN uses_kisi_kisi BOOLEAN NOT NULL DEFAULT false;

UPDATE exams e SET uses_kisi_kisi = true
 WHERE EXISTS (SELECT 1 FROM exam_blueprints b WHERE b.exam_id = e.id);

-- Index for filtering. Tenant-leading because every read is
-- tenant-scoped.
CREATE INDEX exams_uses_kisi_kisi_idx ON exams (tenant_id, uses_kisi_kisi);

-- Direct stimulus link on questions (solo / non-group attachment).
-- The group-mediated path stays via exam_questions.group_id from
-- migration 000015.
ALTER TABLE exam_questions ADD COLUMN stimulus_id UUID
  REFERENCES stimuli(id) ON DELETE SET NULL;

CREATE INDEX exam_questions_stimulus_idx ON exam_questions (tenant_id, stimulus_id)
  WHERE stimulus_id IS NOT NULL;

-- Mutual exclusion: a question carries a stimulus through one path or
-- the other, never both. Enforced both at the DB layer (defence in
-- depth) and at the handler layer (so we can return a structured 422).
ALTER TABLE exam_questions ADD CONSTRAINT exam_questions_stimulus_xor_group_chk
  CHECK (stimulus_id IS NULL OR group_id IS NULL);
