-- 000025_exam_grade_level.sql
-- Optional exam grade metadata. Scoped by tenant education profile in UI and
-- validated softly by backend against enabled phases.

ALTER TABLE exams
  ADD COLUMN IF NOT EXISTS grade_level TEXT;

CREATE INDEX IF NOT EXISTS idx_exams_tenant_grade_level
  ON exams(tenant_id, grade_level);
