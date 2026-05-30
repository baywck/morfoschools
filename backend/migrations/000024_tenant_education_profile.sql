-- 000024_tenant_education_profile.sql
-- Tenant-level curriculum scope. This becomes the source of truth for CP,
-- subject choices, blueprints, and exams instead of inferring only from classes.

ALTER TABLE tenants
  ADD COLUMN IF NOT EXISTS school_type TEXT NOT NULL DEFAULT 'sma'
    CHECK (school_type IN ('sd', 'smp', 'sma', 'smk', 'mixed')),
  ADD COLUMN IF NOT EXISTS enabled_phases TEXT[] NOT NULL DEFAULT ARRAY['e','f']::TEXT[],
  ADD COLUMN IF NOT EXISTS include_vocational_subjects BOOLEAN NOT NULL DEFAULT false;

UPDATE tenants
   SET include_vocational_subjects = true
 WHERE school_type = 'smk';
