# Zul Desk

Central de atendimento multiusuário para WhatsApp Business, construída para equipes de vendas, suporte e financeiro. O projeto integra a Meta WhatsApp Cloud API, organiza conversas por responsáveis e grupos e permite personalização white label pela própria interface.

> Projeto em estágio MVP. O modo de demonstração funciona integralmente sem credenciais da Meta.

## Funcionalidades

- Caixa de entrada em tempo real com conversas próprias e visão da equipe.
- Texto, emojis, imagens, vídeos MP4, documentos, figurinhas WebP e áudios.
- Gravação de áudio pelo navegador e reprodução dentro da conversa.
- Transferência de atendimentos entre usuários.
- Perfis de Administrador, Supervisor e Usuário.
- Grupos configuráveis, como Vendas, Financeiro e Suporte.
- Cadastro de múltiplos números e contas WABA.
- Configuração do webhook, App Secret, token de verificação e credenciais pela interface.
- Templates aprovados pela Meta e controle da janela de 24 horas.
- Cadastro e importação de clientes, opt-in, opt-out, notas e histórico.
- Personalização white label de nome, empresa, logotipo e favicon.
- Dashboard operacional e atualizações por WebSocket.
- Relatórios de 7, 30 e 90 dias com conversão, produtividade, evolução diária e exportação CSV.

## Tecnologias

| Camada | Tecnologias |
| --- | --- |
| Backend | Go, Chi, pgx e JWT |
| Frontend | Next.js, React e TypeScript |
| Dados | PostgreSQL 16 |
| Filas | Redis e Asynq |
| Infraestrutura | Docker Compose e Caddy |
| Integração | Meta WhatsApp Cloud API |

## Executando com Docker

### Pré-requisitos

- Docker Desktop com Docker Compose.
- Portas necessárias: `3000` no computador.

### Inicialização

```powershell
Copy-Item .env.example .env
docker compose up --build
```

Abra [http://localhost:3000](http://localhost:3000).

Usuários de demonstração:

| Perfil | E-mail | Senha |
| --- | --- | --- |
| Administrador | `admin@newlife.local` | `comercial123` |
| Usuário | `carlos@newlife.local` | `comercial123` |

O ambiente inicia com `WHATSAPP_MOCK=true`. Nesse modo, as mensagens passam pela fila e recebem identificadores simulados, sem fazer chamadas à Meta.

Para encerrar:

```powershell
docker compose down
```

Para também apagar os dados locais do PostgreSQL, as mídias e a identidade visual:

```powershell
docker compose down -v
```

### Servidor de testes

Em um servidor Linux com Docker e Docker Compose instalados:

```bash
git clone https://github.com/brunoavila55/zul-desk.git
cd zul-desk
cp .env.example .env
# Edite o .env antes de iniciar.
docker compose up --build -d
docker compose ps
```

No `.env`, defina `APP_ENV=production`, troque `POSTGRES_PASSWORD`, ajuste a mesma senha dentro de `DATABASE_URL` e gere valores longos e diferentes para `JWT_SECRET` e `CREDENTIAL_ENCRYPTION_KEY`.

Para um teste apenas por IP e HTTP, mantenha `APP_DOMAIN=:80`, escolha a porta pública em `HTTP_PORT` e acesse `http://IP-DO-SERVIDOR:PORTA`. Para usar um domínio com HTTPS automático, aponte o DNS para o servidor e configure:

```env
APP_DOMAIN=desk.seudominio.com
HTTP_PORT=80
HTTPS_PORT=443
CORS_ORIGIN=https://desk.seudominio.com
```

As portas 80 e 443 devem estar liberadas no firewall. O Caddy obtém e renova o certificado automaticamente. HTTPS público é obrigatório para receber webhooks da Meta; nesse caso, use `https://desk.seudominio.com/api/webhooks/whatsapp`.

Comandos úteis no servidor:

```bash
docker compose logs -f --tail=200
docker compose pull
docker compose up --build -d
```

## Configuração

As variáveis disponíveis estão documentadas em [`.env.example`](.env.example). Antes de usar em produção, altere obrigatoriamente:

```env
JWT_SECRET=gere-um-segredo-forte
CREDENTIAL_ENCRYPTION_KEY=gere-outro-segredo-forte
WHATSAPP_MOCK=false
```

As credenciais de cada número e do webhook podem ser cadastradas em **Configurações > Números WhatsApp**. Tokens e o App Secret são criptografados com AES-256-GCM. O Access Token e o App Secret nunca são devolvidos pela API ou enviados novamente ao navegador.

## Meta WhatsApp Cloud API

### Número de teste da Meta

O número gratuito fornecido na etapa **Experimente** pode ser cadastrado no Zul Desk com o WABA ID, Phone Number ID e token temporário exibidos pela Meta. Deixe **Usar coexistência** desmarcado, adicione até cinco destinatários verificados no painel da Meta, sincronize o template `hello_world` e use-o para iniciar o primeiro contato. Tokens temporários podem ser renovados em **Configurações > Números WhatsApp > Atualizar token**.

Para conectar um número:

1. Crie um Business Portfolio e um aplicativo em [Meta for Developers](https://developers.facebook.com/).
2. Adicione o produto WhatsApp e obtenha o WABA ID, Phone Number ID e um token permanente de usuário do sistema.
3. Publique a aplicação em um endereço HTTPS.
4. Configure o webhook como `https://seu-dominio/api/webhooks/whatsapp`.
5. Assine o campo `messages` da conta WABA.
6. No Zul Desk, abra **Configurações > Números WhatsApp**, cadastre a conta e teste a conexão.

O endpoint `GET /api/webhooks/whatsapp` atende ao desafio de verificação. O `POST` valida `X-Hub-Signature-256` quando o App Secret está configurado, trata eventos de forma idempotente e atualiza mensagens e conversas.

### Múltiplos números

Cada conversa registra o número de saída. O worker seleciona a credencial criptografada correspondente, enquanto o webhook usa `metadata.phone_number_id` para encaminhar mensagens à conta correta. É possível adicionar novos números posteriormente, inclusive de WABAs diferentes.

### Coexistência com WhatsApp Business no celular

Para manter o mesmo número ativo no aplicativo WhatsApp Business e na Cloud API, faça o onboarding pela modalidade **WhatsApp Business App Coexistence** no Embedded Signup da Meta. Não use o fluxo convencional de migração para esse número.

O cadastro atual no Zul Desk recebe os identificadores e o token emitidos após o onboarding. Automatizar o Embedded Signup exige também App ID, App Secret e Configuration ID da Meta.

## Perfis e permissões

| Recurso | Usuário | Supervisor | Administrador |
| --- | :---: | :---: | :---: |
| Atender conversas próprias | Sim | Sim | Sim |
| Transferir conversas próprias | Sim | Sim | Sim |
| Ver conversas da equipe | Não | Sim | Sim |
| Transferir conversas da equipe | Não | Sim | Sim |
| Acessar templates | Não | Sim | Sim |
| Gerenciar equipe e grupos | Não | Não | Sim |
| Gerenciar números e marca | Não | Não | Sim |

Administradores configuram Templates, Equipe, Números WhatsApp e Marca e aparência dentro da área **Configurações**.

## Regras de atendimento

- Contatos sem opt-in ou com opt-out são bloqueados para contato comercial.
- Cada cliente pode ter somente um atendimento ativo.
- Fora da janela de 24 horas, a abertura exige um template aprovado.
- Texto livre só é permitido após uma resposta válida do cliente.
- Somente o responsável pela conversa pode enviar mensagens.
- Supervisores e administradores podem acompanhar e transferir atendimentos da equipe.
- IDs de mensagens e eventos são únicos para tolerar webhooks repetidos.
- Os estados entregue e lida dependem dos webhooks da Meta.

## Mídias

Arquivos são armazenados no volume Docker `media_data` e acessados por rotas autenticadas. No envio, o worker faz o upload para a Meta e envia a mensagem com o número associado à conversa. No recebimento, a API baixa a mídia usando a credencial da conta correta.

Gravações WebM do navegador são convertidas para OGG/Opus com FFmpeg. O limite atual é de 25 MB por arquivo.

## Estrutura do projeto

```text
backend/
  cmd/api/                 API REST e WebSocket
  cmd/worker/              processamento assíncrono
  internal/config/         configuração por ambiente
  internal/jobs/           contratos das filas
  internal/secure/         criptografia de credenciais
  internal/whatsapp/       cliente da Meta Cloud API
  database/migrations/     esquema e dados de demonstração
  database/queries/        queries para sqlc
frontend/
  app/                     interface Next.js
  lib/api.ts               cliente REST autenticado
docker-compose.yml         stack local completa
Caddyfile                  proxy para frontend, API e WebSocket
```

## Desenvolvimento

Executar os testes do backend:

```powershell
docker run --rm -v "${PWD}/backend:/src" -w /src golang:1.24-alpine go test ./...
```

Validar o frontend:

```powershell
docker run --rm -v "${PWD}/frontend:/app" -v zuldesk_frontend_node_modules:/app/node_modules -w /app node:22-alpine sh -lc "corepack enable && pnpm install --frozen-lockfile && pnpm typecheck"
```

Regenerar os tipos do sqlc após alterar as queries:

```powershell
docker run --rm -v "${PWD}/backend:/src" -w /src sqlc/sqlc generate
```

## Endpoints principais

- `POST /api/auth/login`, `/refresh` e `/logout`
- `GET /api/auth/me`
- `GET|POST|PATCH /api/customers`
- `POST /api/customers/import`
- `GET|POST /api/conversations`
- Mensagens, mídias, notas, atribuição e encerramento em subrotas de conversas
- `GET /api/templates`
- `GET /api/dashboard`
- `GET /api/reports?days=7|30|90` para supervisores e administradores
- `GET|POST|PATCH /api/users`
- `GET|POST|PATCH /api/groups`
- `GET|POST /api/webhooks/whatsapp`
- WebSocket em `/ws`

## Segurança para produção

- Use segredos longos e diferentes para JWT e criptografia de credenciais.
- Nunca envie o arquivo `.env` ao repositório.
- Troque ou remova os usuários de demonstração.
- Use HTTPS no proxy público.
- Restrinja o banco e o Redis à rede interna.
- Configure backups para PostgreSQL e volumes de mídia.

## Escopo atual

O MVP não inclui campanhas em massa, chatbot, IA, billing ou multitenancy completo. A arquitetura atual prioriza atendimento humano, organização por equipes e integração segura com a Cloud API.
