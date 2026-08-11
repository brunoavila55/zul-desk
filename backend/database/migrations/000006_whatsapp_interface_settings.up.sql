ALTER TABLE app_settings
  ADD COLUMN IF NOT EXISTS whatsapp_verify_token_encrypted TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS whatsapp_app_secret_encrypted TEXT NOT NULL DEFAULT '';
