CREATE TABLE IF NOT EXISTS agent_session_memory (
    session_id UUID PRIMARY KEY REFERENCES ai_sessions(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    scope_key TEXT NOT NULL,
    memory_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_session_memory_tenant_scope
ON agent_session_memory(tenant_id, scope_key);
