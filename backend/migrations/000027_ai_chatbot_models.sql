ALTER TABLE tenant_ai_provider_settings
  ADD COLUMN IF NOT EXISTS chatbot_models JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE user_ai_provider_settings
  ADD COLUMN IF NOT EXISTS chatbot_models JSONB NOT NULL DEFAULT '[]'::jsonb;
