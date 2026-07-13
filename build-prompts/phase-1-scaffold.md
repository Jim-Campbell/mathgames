# Phase 1 — Scaffold: server skeleton, database, auth, Dockerfile

Read `CLAUDE.md` and `ARCHITECTURE.md` in full before writing any code. This
phase produces a running Go server with the complete database schema, bearer
auth, and deployment plumbing — no game logic yet.

## Reference implementations

Copy structure and idioms from the sibling apps rather than inventing:

- `~/projects/food/cmd/server/main.go` — env config, dependency injection,
  graceful shutdown
- `~/projects/food/internal/db/migrate.go` — the migrations-at-startup runner
  **including the advisory-lock serialization** (embed `migrations/*.sql`,
  apply in filename order, record applied names in `schema_migrations`)
- `~/projects/food/internal/api/middleware.go` — bearer-auth middleware,
  request-logging middleware
- `~/projects/food/internal/api/handler.go` — chi router setup, static PWA
  serving
- `~/projects/food/Dockerfile` — multi-stage build

## Tasks

1. `go mod init github.com/jimgcampbell/mathgames` (Go 1.25). Dependencies:
   `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`. Nothing else yet.
2. `cmd/server/main.go`: read env per ARCHITECTURE.md → Environment
   (`DATABASE_URL` and `MATHGAMES_API_KEY` required — fail fast with a clear
   message; `PORT` default 8083; `ANTHROPIC_API_KEY`/`AI_MODEL` read but only
   logged as configured/not-configured in this phase). Wire pgx pool →
   migrations → router → `http.Server` with graceful shutdown.
3. `internal/db/`: pgx pool constructor + migrations runner +
   `migrations/001_initial.sql` containing **exactly** the schema in
   ARCHITECTURE.md → "Database schema (migration 001)", including the
   settings seed (`ON CONFLICT DO NOTHING`).
4. `internal/game/skills.go`: the skill registry only — the 8 skill slugs
   from ARCHITECTURE.md → "Skills and difficulty" with name, source
   (`template`/`ai`), and display metadata. On startup, seed any missing
   `skill_state` rows from the registry (`INSERT ... ON CONFLICT DO NOTHING`).
5. `internal/api/`: chi router with logging middleware; bearer-auth
   middleware on everything under `/api` except `GET /api/health`; health
   handler returning `{"ok":true,"ai":false}` (`ai` reflects whether
   `ANTHROPIC_API_KEY` is set); serve `pwa/` at `/` (placeholder
   `pwa/index.html` that just says "mathgames" is fine).
6. `Dockerfile` (multi-stage `golang:1.25-alpine` builder with
   `GOTOOLCHAIN=local` → `alpine` runtime, copy `pwa/` into the image),
   `.gitignore` (binaries, `.env`), `.env.example` listing every env var
   from ARCHITECTURE.md with comments.

## Out of scope

No generators, no scoring, no handlers beyond health, no AI, no real PWA.

## Acceptance checklist

- `go build ./... && go test ./...` passes (no tests yet is fine).
- `createdb mathgames_smoke && DATABASE_URL=postgres://localhost:5432/mathgames_smoke?sslmode=disable MATHGAMES_API_KEY=x go run ./cmd/server`
  starts, applies migration 001, seeds 8 `skill_state` rows, and logs the port.
- `curl localhost:8083/api/health` → 200 `{"ok":true,"ai":false}` **without** a key.
- `curl localhost:8083/api/anything-else` → 401 without
  `Authorization: Bearer x`, 404 with it.
- `psql mathgames_smoke -c '\d attempts'` shows the schema; `settings` has
  row id=1; `skill_state` has 8 rows.
- Restarting the server does not re-apply or fail on migration 001.
- `docker build .` succeeds.
- `dropdb mathgames_smoke` at the end.
