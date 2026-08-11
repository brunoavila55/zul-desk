CREATE TABLE IF NOT EXISTS whatsapp_accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  business_account_id TEXT NOT NULL,
  phone_number_id TEXT NOT NULL,
  display_phone_number TEXT,
  access_token_encrypted TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE whatsapp_accounts
  ADD COLUMN IF NOT EXISTS api_version TEXT NOT NULL DEFAULT 'v23.0',
  ADD COLUMN IF NOT EXISTS onboarding_type TEXT NOT NULL DEFAULT 'MANUAL',
  ADD COLUMN IF NOT EXISTS coexistence BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS verified_name TEXT,
  ADD COLUMN IF NOT EXISTS quality_rating TEXT,
  ADD COLUMN IF NOT EXISTS platform_status TEXT,
  ADD COLUMN IF NOT EXISTS token_expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_verified_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id);

CREATE UNIQUE INDEX IF NOT EXISTS whatsapp_accounts_phone_number_id_unique
  ON whatsapp_accounts(phone_number_id);

ALTER TABLE conversations
  ADD COLUMN IF NOT EXISTS whatsapp_account_id UUID REFERENCES whatsapp_accounts(id);

CREATE INDEX IF NOT EXISTS conversations_whatsapp_account_idx
  ON conversations(whatsapp_account_id, status);

ALTER TABLE templates
  ADD COLUMN IF NOT EXISTS whatsapp_account_id UUID REFERENCES whatsapp_accounts(id);

ALTER TABLE templates DROP CONSTRAINT IF EXISTS templates_meta_template_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS templates_account_meta_id_unique
  ON templates(whatsapp_account_id, meta_template_id)
  WHERE meta_template_id IS NOT NULL;

INSERT INTO whatsapp_accounts (
  name, business_account_id, phone_number_id, display_phone_number,
  access_token_encrypted, api_version, onboarding_type, coexistence,
  verified_name, platform_status, active
)
SELECT
  'Número de demonstração',
  COALESCE(NULLIF(current_setting('app.bootstrap_waba_id', true), ''), 'mock-waba'),
  COALESCE(NULLIF(current_setting('app.bootstrap_phone_id', true), ''), 'mock-phone'),
  '+55 55 99999-0000', '', 'v23.0', 'DEMO', true,
  'New Life', 'CONNECTED', true
WHERE NOT EXISTS (SELECT 1 FROM whatsapp_accounts);

UPDATE conversations
SET whatsapp_account_id = (SELECT id FROM whatsapp_accounts ORDER BY created_at LIMIT 1)
WHERE whatsapp_account_id IS NULL;

UPDATE templates
SET whatsapp_account_id = (SELECT id FROM whatsapp_accounts ORDER BY created_at LIMIT 1)
WHERE whatsapp_account_id IS NULL;
