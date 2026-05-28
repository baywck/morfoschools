-- Backfill legacy unscoped AI sessions so the new workflow-first agent
-- never reuses old pre-reset history as the global chat bucket.
UPDATE ai_sessions
   SET scope_key = 'legacy:' || id::text
 WHERE scope_key IS NULL OR scope_key = '';

ALTER TABLE ai_sessions
  ALTER COLUMN scope_key SET DEFAULT 'global';
