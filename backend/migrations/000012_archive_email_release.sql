-- 000012_archive_email_release.sql
-- Allow email reuse after a user is archived.
--
-- Rationale: previously `users.email` had a global unique index, so a graduated
-- student or former staff member kept blocking their email forever. The new
-- design preserves the original email in `original_email` and swaps `email`
-- with a synthetic value, while a partial unique index makes archived rows
-- exempt from the uniqueness constraint.

ALTER TABLE users ADD COLUMN IF NOT EXISTS original_email TEXT;

-- Backfill: any user already archived under the old constraint loses their
-- email-slot to a future signup. Move the real email into `original_email`
-- (so admins can restore later) and replace `email` with a synthetic value
-- that embeds the user UUID for traceability and uniqueness-by-construction.
UPDATE users
SET original_email = email,
    email = 'archived+' || id || '@archived.morfoschools.local',
    updated_at = now()
WHERE status = 'archived' AND original_email IS NULL;

-- Drop the existing global unique index and replace with a partial index that
-- only enforces uniqueness for non-archived users. Synthetic archived emails
-- are unique-by-construction (UUID embedded) so collisions are impossible.
DROP INDEX IF EXISTS idx_users_email;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active
    ON users(email) WHERE status != 'archived';

-- Helpful lookup for restore flows.
CREATE INDEX IF NOT EXISTS idx_users_original_email
    ON users(original_email) WHERE original_email IS NOT NULL;
