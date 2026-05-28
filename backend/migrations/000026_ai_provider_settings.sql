-- 000026_ai_provider_settings.sql
-- Tenant/user scoped AI provider configuration.
-- API keys are stored encrypted by the backend; never returned to clients.

CREATE TABLE IF NOT EXISTS tenant_ai_provider_settings (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    base_url TEXT NOT NULL,
    encrypted_api_key TEXT NOT NULL,
    default_model TEXT,
    available_models JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_roles TEXT[] NOT NULL DEFAULT '{}'::text[],
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_ai_provider_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    base_url TEXT NOT NULL,
    encrypted_api_key TEXT NOT NULL,
    default_model TEXT,
    available_models JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_user_ai_provider_settings_tenant
    ON user_ai_provider_settings(tenant_id);
