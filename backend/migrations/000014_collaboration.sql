-- 000014_collaboration.sql
-- Phase 9.5 \u2014 ownership + collaborator model on exams, courses,
-- blueprint_templates per ADR-0009.
--
-- This migration is layered: it adds ownership and collaborator tables
-- but does NOT remove subject-based RBAC. Subject membership remains
-- as a read-only institutional fallback so waka kurikulum / ketua MGMP
-- patterns continue to work without explicit invitation.
--
-- Pre-flight: the migrate runner refuses to apply this migration if any
-- exam or course row has NULL `created_by` or `created_by` pointing to
-- a missing/archived user. See preflight_owner_backfill.go for the
-- audit query and operator workflow.

-- =========================================================================
-- 1. Owner column on exams
-- =========================================================================

ALTER TABLE exams
    ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id);

-- Backfill from created_by. The pre-flight audit guarantees there are
-- no NULL or orphan rows by the time we reach this point.
UPDATE exams
   SET owner_user_id = created_by
 WHERE owner_user_id IS NULL;

ALTER TABLE exams
    ALTER COLUMN owner_user_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_exams_owner ON exams(owner_user_id);

-- =========================================================================
-- 2. Owner column on courses
-- =========================================================================

ALTER TABLE courses
    ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES users(id);

UPDATE courses
   SET owner_user_id = created_by
 WHERE owner_user_id IS NULL;

ALTER TABLE courses
    ALTER COLUMN owner_user_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_courses_owner ON courses(owner_user_id);

-- =========================================================================
-- 3. Collaborator tables
-- =========================================================================
--
-- One row per (resource, user). Role determines the permission level.
-- Tenant scoping is denormalized for cleaner audit partitioning even
-- though the resource FK already implies tenant transitively.

CREATE TABLE IF NOT EXISTS exam_collaborators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('editor', 'viewer')),
    invited_by UUID REFERENCES users(id),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (exam_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_exam_collaborators_user
    ON exam_collaborators(user_id, exam_id);
CREATE INDEX IF NOT EXISTS idx_exam_collaborators_exam_role
    ON exam_collaborators(exam_id, role);

CREATE TABLE IF NOT EXISTS course_collaborators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('editor', 'viewer')),
    invited_by UUID REFERENCES users(id),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (course_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_course_collaborators_user
    ON course_collaborators(user_id, course_id);
CREATE INDEX IF NOT EXISTS idx_course_collaborators_course_role
    ON course_collaborators(course_id, role);

-- blueprint_template_collaborators is created together with the
-- blueprint_templates table in migration 000015. We can't create it
-- here because the parent table doesn't exist yet.

-- =========================================================================
-- 4. New permissions for blueprints
-- =========================================================================
--
-- Permissions are seeded by devseed.go on every startup, so we don't
-- INSERT them here. This comment is a reminder that devseed.go must be
-- updated alongside this migration.
--   blueprints:read   \u2014 view blueprint templates and exam blueprints
--   blueprints:write  \u2014 create/edit blueprint templates
--
-- The collaborator role gates access to specific resources; the
-- permission gates the entire feature surface.
