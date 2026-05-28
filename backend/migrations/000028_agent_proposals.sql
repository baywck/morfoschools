CREATE TABLE IF NOT EXISTS agent_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES ai_sessions(id) ON DELETE CASCADE,
    workflow TEXT NOT NULL,
    args JSONB NOT NULL DEFAULT '{}'::jsonb,
    preview TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'confirmed', 'cancelled', 'expired', 'failed')),
    result JSONB,
    error TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_proposals_session_status
    ON agent_proposals(session_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_proposals_user_status
    ON agent_proposals(user_id, status, created_at DESC);
