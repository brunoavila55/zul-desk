CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('ADMIN', 'SUPERVISOR', 'AGENT');
CREATE TYPE conversation_status AS ENUM ('OPEN', 'WAITING_CUSTOMER', 'WAITING_AGENT', 'CLOSED');
CREATE TYPE message_status AS ENUM ('PENDING', 'QUEUED', 'SENT', 'DELIVERED', 'READ', 'FAILED');
CREATE TYPE sender_type AS ENUM ('AGENT', 'CUSTOMER', 'SYSTEM');

CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name TEXT NOT NULL, email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL, role user_role NOT NULL DEFAULT 'AGENT', active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE refresh_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE, expires_at TIMESTAMPTZ NOT NULL, revoked_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE customers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), external_id TEXT UNIQUE, name TEXT NOT NULL, phone TEXT NOT NULL UNIQUE,
  document TEXT, customer_since DATE, product TEXT, city TEXT, assigned_user_id UUID REFERENCES users(id), tags TEXT[] NOT NULL DEFAULT '{}',
  whatsapp_opt_in BOOLEAN NOT NULL DEFAULT FALSE, opt_in_date TIMESTAMPTZ, opt_in_source TEXT,
  active BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), customer_id UUID NOT NULL REFERENCES customers(id),
  assigned_user_id UUID NOT NULL REFERENCES users(id), status conversation_status NOT NULL DEFAULT 'OPEN',
  service_window_expires_at TIMESTAMPTZ, started_at TIMESTAMPTZ NOT NULL DEFAULT now(), closed_at TIMESTAMPTZ,
  result TEXT, result_note TEXT, template_name TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX one_active_conversation_per_customer ON conversations(customer_id) WHERE status <> 'CLOSED';
CREATE INDEX conversations_assignee_status_idx ON conversations(assigned_user_id, status);
CREATE TABLE messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  whatsapp_message_id TEXT UNIQUE, sender_type sender_type NOT NULL, user_id UUID REFERENCES users(id), type TEXT NOT NULL DEFAULT 'TEXT',
  body TEXT NOT NULL, status message_status NOT NULL DEFAULT 'PENDING', sent_at TIMESTAMPTZ, delivered_at TIMESTAMPTZ,
  read_at TIMESTAMPTZ, failed_at TIMESTAMPTZ, error_code TEXT, error_message TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX messages_conversation_created_idx ON messages(conversation_id, created_at);
CREATE TABLE templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), meta_template_id TEXT UNIQUE, name TEXT NOT NULL, language TEXT NOT NULL DEFAULT 'pt_BR',
  category TEXT NOT NULL, status TEXT NOT NULL, content TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE notes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), customer_id UUID NOT NULL REFERENCES customers(id), conversation_id UUID REFERENCES conversations(id),
  user_id UUID NOT NULL REFERENCES users(id), content TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE opt_outs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), customer_id UUID NOT NULL REFERENCES customers(id), reason TEXT,
  created_by UUID NOT NULL REFERENCES users(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX one_current_opt_out ON opt_outs(customer_id);
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID REFERENCES users(id), action TEXT NOT NULL,
  entity_type TEXT NOT NULL, entity_id UUID, metadata JSONB NOT NULL DEFAULT '{}', ip_address INET, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE webhook_events (
  id TEXT PRIMARY KEY, payload JSONB NOT NULL, processed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- senha local: comercial123 (o bootstrap troca este marcador por um hash bcrypt no primeiro start)
INSERT INTO users (name, email, password_hash, role) VALUES
('Ana Martins', 'admin@newlife.local', '$bootstrap$', 'ADMIN'),
('Carlos Lima', 'carlos@newlife.local', '$bootstrap$', 'AGENT'),
('Maria Costa', 'maria@newlife.local', '$bootstrap$', 'AGENT');
INSERT INTO customers (external_id,name,phone,customer_since,product,city,assigned_user_id,tags,whatsapp_opt_in,opt_in_date,opt_in_source)
SELECT '38481','João da Silva','5555999999999','2020-03-15','500 Mbps','São Gabriel',id,ARRAY['Cliente antigo','Upgrade'],true,'2024-05-10','Contrato' FROM users WHERE email='carlos@newlife.local';
INSERT INTO customers (external_id,name,phone,customer_since,product,city,assigned_user_id,tags,whatsapp_opt_in,opt_in_date,opt_in_source)
SELECT '38482','Maria Oliveira','5555988888888','2022-08-01','300 Mbps','São Gabriel',id,ARRAY['Upgrade'],true,'2024-05-10','Contrato' FROM users WHERE email='maria@newlife.local';
INSERT INTO customers (external_id,name,phone,customer_since,product,city,whatsapp_opt_in) VALUES
('38483','José Pereira','5555977777777','2018-05-18','600 Mbps','Santa Maria',false);
INSERT INTO templates (meta_template_id,name,category,status,content) VALUES
('local-1','cliente_antigo_promocao','MARKETING','APPROVED','Olá {{1}}!\n\nAqui é {{2}}, do setor comercial da New Life.\n\nVocê já é nosso cliente há {{3}} e temos uma condição especial disponível para você.\n\nPosso te passar mais informações?'),
('local-2','upgrade_plano','MARKETING','APPROVED','Olá {{1}}! Aqui é {{2}} da New Life. Temos uma condição especial para melhorar seu plano atual. Posso apresentar?');
