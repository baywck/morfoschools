-- AI task states for multi-step workflows
CREATE TABLE IF NOT EXISTS ai_task_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    intent TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'collecting' CHECK (status IN ('collecting', 'ready', 'confirming', 'executing', 'done', 'failed')),
    frame JSONB NOT NULL DEFAULT '{}',
    plan JSONB,
    current_step INT DEFAULT 0,
    total_steps INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '30 minutes')
);
CREATE INDEX IF NOT EXISTS idx_ai_task_states_session ON ai_task_states(session_id, status);
