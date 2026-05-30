-- ai_sessions.scope_key — per-resource session scoping
--
-- Phase 9.12: AI chat sessions key by user + scope (resource being
-- viewed) instead of one global thread per user. When the user is
-- on /app/exams/{A} and switches to /app/exams/{B}, the chat panel
-- should load the session associated with B, not the leftover
-- conversation from A. Without this column the model sees mixed
-- residue from previous resources and proposes against the wrong
-- target.
--
-- Format: 'exam:<uuid>' | 'blueprint:<uuid>' | 'global' (or NULL
-- for legacy rows). Backend derives this from request shadow on
-- every chat call; clients never set it directly.

ALTER TABLE ai_sessions ADD COLUMN IF NOT EXISTS scope_key TEXT;

-- Index supports the common lookup: "find the most recent session
-- for THIS user in THIS scope". Used by the auto-resume logic and
-- the per-scope listing endpoint.
CREATE INDEX IF NOT EXISTS idx_ai_sessions_user_scope
  ON ai_sessions (user_id, scope_key, last_active_at DESC);

-- Existing rows get scope_key = NULL → treated as 'legacy global'
-- by the listing endpoint so users don't lose their old chat
-- history. Future rows always have scope_key set.
