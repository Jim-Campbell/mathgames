# CLAUDE.md - Math Games App

## Project Overview

DBZ-themed math + logic games PWA for Skylar (gifted 8-year-old, 4th-grade
content, adaptive). Single user, iPad-first. Sibling app to
`~/projects/food`, `~/projects/finance`, and `~/projects/journal` — same
structure, patterns, and bearer-key auth.

**Read ARCHITECTURE.md before making changes** — it holds the full design:
schema, API surface, skill/difficulty tables, XP + adaptive algorithms with
worked examples, AI content pipeline, and every PWA screen. Build-phase
prompts live in `build-prompts/`.

## Tech Stack

- Go 1.25, chi router, pgx/v5, PostgreSQL (Render-hosted in production)
- Vanilla JS single-file PWA in `pwa/` served by the Go binary
- Anthropic API via hand-rolled client (no SDK), default `claude-sonnet-5`,
  used only for batch content generation — the app runs without the key

## Reference implementations (read these, copy their patterns)

- `~/projects/food/internal/ai/client.go` — Anthropic HTTP client, copy near-verbatim
- `~/projects/food/internal/db/migrate.go` — migrations runner with
  advisory-lock serialization, copy it
- `~/projects/food/internal/db/` + `~/projects/finance/internal/db/` — pgx
  store layout
- `~/projects/food/cmd/server/main.go` — env config, DI, graceful shutdown
- `~/projects/food/internal/api/middleware.go` — bearer auth + logging
- `~/projects/food/pwa/sw.js` — network-first service worker (do NOT use
  cache-first with a version constant)
- `~/projects/finance/pwa/index.html` — dialog/`S`-state/`render()` conventions
- `~/projects/food/build-prompts/` — the phase-prompt style this repo repeats

## Domain invariants (do not break)

- **No floats in stored data or scoring math.** XP/levels/streaks: integers.
  Times: integer milliseconds. Accuracy: integer basis points (8750 =
  87.50%). Multiply before dividing.
- **Answers never reach the client before an attempt is submitted.**
  `GET /api/next` strips `answer`/`explanation`; grading is server-side in
  `POST /api/attempts`.
- **Every served question is a `questions` row**; every attempt is stored
  raw (given answer, elapsed_ms, streak/level at that moment). Never discard
  or aggregate away raw attempt rows.
- **All game logic is deterministic Go with unit tests** — XP, streaks,
  zenkai, adaptive ladder, grading, daily seeding, unlocks. The AI generates
  content (word problems, logic puzzles, saga stories) in reviewed batches;
  it never grades, scores, or writes game state.
- AI questions are validated before insert (shape + numeric `check`
  expression) and can be retired from the parent view; keep raw batch output
  in `ai_batches.raw`.
- Level-downs are silent in the UI; level-ups celebrate. Never show the kid
  a demotion.
- Wrong answers always show the correct answer **with the explanation** —
  that's the teaching moment.
- Parent scorecard is the hidden `#/parents` route (no tab, no PIN).
- The earned-screen-time ledger is **parked** — don't build it, but don't
  break the raw data (sessions + attempts + timings) that will power it.

## Verifying changes

- `go build ./... && go test ./...`
- Unit tests live in `internal/game/*_test.go` (scoring, adaptive, grading,
  generators, daily seeding) and `internal/ai/*_test.go` (validation against
  recorded fixtures).
- Smoke test against a scratch DB (never the cloud DB):
  `createdb mathgames_smoke && DATABASE_URL=postgres://localhost:5432/mathgames_smoke?sslmode=disable MATHGAMES_API_KEY=x go run ./cmd/server`
  — `dropdb mathgames_smoke` when done.

## Deployment

Render, from the `Dockerfile` (multi-stage `golang:1.25-alpine` → `alpine`,
`GOTOOLCHAIN=local`). Migrations run automatically at startup. Never point a
local server at the production `DATABASE_URL`. Backups = `/api/export`
download (Render disks are ephemeral).

## Environment

Required: `DATABASE_URL`, `MATHGAMES_API_KEY`. Optional: `PORT` (default
8083), `ANTHROPIC_API_KEY` (content generation off without it), `AI_MODEL`
(default `claude-sonnet-5`). See ARCHITECTURE.md → Environment.
