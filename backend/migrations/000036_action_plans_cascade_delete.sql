-- +goose Up
-- Change agent_action_plans.session_id from SET NULL to CASCADE
-- so that deleting a chat session also deletes its action plans.

ALTER TABLE agent_action_plans
    DROP CONSTRAINT agent_action_plans_session_id_fkey;

ALTER TABLE agent_action_plans
    ADD CONSTRAINT agent_action_plans_session_id_fkey
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE CASCADE;

-- +goose Down
-- Revert to SET NULL behavior.

ALTER TABLE agent_action_plans
    DROP CONSTRAINT agent_action_plans_session_id_fkey;

ALTER TABLE agent_action_plans
    ADD CONSTRAINT agent_action_plans_session_id_fkey
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE SET NULL;
