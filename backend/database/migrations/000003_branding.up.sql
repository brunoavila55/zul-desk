CREATE TABLE IF NOT EXISTS app_settings (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  app_name TEXT NOT NULL DEFAULT 'Zul Desk',
  company_name TEXT NOT NULL DEFAULT 'New Life',
  logo_url TEXT,
  favicon_url TEXT,
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_settings (id, app_name, company_name)
VALUES (1, 'Zul Desk', 'New Life')
ON CONFLICT (id) DO UPDATE
SET app_name = CASE WHEN app_settings.app_name = 'New Life Comercial' THEN 'Zul Desk' ELSE app_settings.app_name END;
