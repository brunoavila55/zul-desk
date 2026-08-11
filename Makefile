.PHONY: up down logs test sqlc migrate

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=100

test:
	docker run --rm -v "$(PWD)/backend:/src" -w /src golang:1.24-alpine go test ./...
	docker run --rm -v "$(PWD)/frontend:/app" -v zuldesk_frontend_node_modules:/app/node_modules -w /app node:22-alpine sh -lc "corepack enable && pnpm install --frozen-lockfile && pnpm typecheck"

sqlc:
	docker run --rm -v "$(PWD)/backend:/src" -w /src sqlc/sqlc generate

migrate:
	docker compose up -d postgres redis backend
