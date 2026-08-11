CREATE TABLE user_groups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  color TEXT NOT NULL DEFAULT '#15a76e',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX user_groups_name_unique ON user_groups(lower(name));

CREATE TABLE user_group_members (
  group_id UUID NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, user_id)
);

CREATE INDEX user_group_members_user_idx ON user_group_members(user_id);

INSERT INTO user_groups(name, description, color) VALUES
  ('Vendas', U&'Equipe respons\00e1vel pelas oportunidades comerciais', '#15a76e'),
  ('Financeiro', U&'Cobran\00e7as, pagamentos e negocia\00e7\00f5es financeiras', '#5267c9'),
  ('Suporte', U&'Atendimento e resolu\00e7\00e3o de solicita\00e7\00f5es t\00e9cnicas', '#e08b32');

INSERT INTO user_group_members(group_id, user_id)
SELECT g.id, u.id FROM user_groups g CROSS JOIN users u
WHERE g.name = 'Vendas' AND u.email IN ('carlos@newlife.local', 'maria@newlife.local');
