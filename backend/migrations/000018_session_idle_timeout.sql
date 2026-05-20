-- 000018_session_idle_timeout.sql
-- M-6 — Idle session timeout. Add a `last_activity_at` column on
-- sessions so the auth middleware can refuse stale tokens (sliding
-- window) without forcing the user to log in again every 30 minutes
-- of inactivity. The middleware bumps this column on every request;
-- sessions that pass `expires_at` AND idle longer than the policy are
-- treated as expired.
--
-- Forward-only. Existing sessions inherit `now()` so they are not
-- immediately invalidated when this migration runs.

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Index leading on last_activity_at lets a future janitor query for
-- idle-but-not-yet-expired rows efficiently.
CREATE INDEX IF NOT EXISTS idx_sessions_last_activity
    ON sessions (last_activity_at);
