CREATE TABLE IF NOT EXISTS agent_action_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID REFERENCES ai_sessions(id) ON DELETE SET NULL,
    exam_id UUID,
    scope_type TEXT NOT NULL DEFAULT 'generic'
        CHECK (scope_type IN ('exam', 'blueprint', 'question_set', 'section', 'generic')),
    source TEXT NOT NULL DEFAULT 'chat'
        CHECK (source IN ('chat', 'audit', 'reverse_planning', 'create', 'update', 'repair', 'bulk')),
    goal TEXT NOT NULL DEFAULT '',
    intent_summary TEXT NOT NULL DEFAULT '',
    plan_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'paused', 'completed', 'failed', 'cancelled')),
    current_batch_index INT NOT NULL DEFAULT 0,
    total_batches INT NOT NULL DEFAULT 0,
    progress_percent INT NOT NULL DEFAULT 0
        CHECK (progress_percent BETWEEN 0 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_action_plans_exam_status
    ON agent_action_plans(exam_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_action_plans_session
    ON agent_action_plans(session_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS agent_action_plan_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES agent_action_plans(id) ON DELETE CASCADE,
    batch_index INT NOT NULL,
    action_type TEXT NOT NULL DEFAULT 'update'
        CHECK (action_type IN ('analyze', 'create', 'update', 'repair', 'merge', 'link', 'generate', 'finalize')),
    workflow TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL DEFAULT 'generic'
        CHECK (target_type IN ('blueprint_slot', 'question', 'exam', 'section', 'question_set', 'generic')),
    target_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    args_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    preview TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'proposed', 'running', 'confirmed', 'failed', 'skipped')),
    progress_units INT NOT NULL DEFAULT 1,
    completed_units INT NOT NULL DEFAULT 0,
    proposal_id UUID,
    result_json JSONB,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (plan_id, batch_index)
);

CREATE INDEX IF NOT EXISTS idx_agent_action_plan_batches_plan
    ON agent_action_plan_batches(plan_id, batch_index);

CREATE INDEX IF NOT EXISTS idx_agent_action_plan_batches_status
    ON agent_action_plan_batches(plan_id, status, batch_index);
