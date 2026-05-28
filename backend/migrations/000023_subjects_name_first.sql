-- 000023_subjects_name_first.sql
-- Subjects are selected/managed by name in the product UI. Keep code as an
-- internal compatibility slug for historical APIs/search, but enforce active
-- subject uniqueness by name so users do not need to manage codes manually.

UPDATE subjects
   SET code = lower(regexp_replace(regexp_replace(trim(name), '[^A-Za-z0-9]+', '-', 'g'), '(^-|-$)', '', 'g'))
 WHERE (code IS NULL OR trim(code) = '')
   AND name IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_subjects_tenant_active_name
  ON subjects (tenant_id, lower(name))
  WHERE status != 'archived';
