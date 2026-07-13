# DBZ Math Training

A Dragon Ball Z–themed math + logic training PWA built for Skylar, a gifted
8-year-old heading into 3rd grade, targeting 4th-grade content with
difficulty that adapts on its own. Single user, iPad-first, installed as a
home-screen app.

Answering problems is "training," XP drives a power level, milestones
unlock a collection of fighters, story quests are "sagas," and collecting
seven dragon balls summons Shenron for a wish. All artwork is original
inline SVG — names and lore references are used playfully, non-commercially.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full design (schema, API
surface, skill/difficulty tables, XP + adaptive algorithms, AI content
pipeline, every PWA screen) and [CLAUDE.md](CLAUDE.md) for the project's
working conventions.

## Screenshots

_TODO: add Home, Play, Collection, and Parents screenshots once the app is
running on Skylar's iPad._

## Tech stack

- Go 1.25, chi router, pgx/v5, PostgreSQL
- Vanilla JS single-file PWA in [pwa/](pwa/), served by the Go binary
- Anthropic API via a hand-rolled client, used only for batch content
  generation (word problems, logic puzzles, saga stories) — the app runs
  fully without an API key, it just can't mint new AI content

## Local development

Requires Go 1.25+ and a local PostgreSQL. Never point a local server at the
production `DATABASE_URL` — always use a scratch database:

```sh
createdb mathgames_smoke
DATABASE_URL=postgres://localhost:5432/mathgames_smoke?sslmode=disable \
MATHGAMES_API_KEY=x \
go run ./cmd/server
```

Migrations run automatically at startup. Open `http://localhost:8083`, and
when prompted, enter the `MATHGAMES_API_KEY` value you started the server
with. Drop the scratch database when done:

```sh
dropdb mathgames_smoke
```

Verify changes with:

```sh
go build ./... && go test ./...
```

Unit tests live in `internal/game/*_test.go` (scoring, adaptive ladder,
grading, generators, daily seeding) and `internal/ai/*_test.go` (AI output
validation against recorded fixtures).

## Environment variables

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres connection string |
| `MATHGAMES_API_KEY` | yes | — | Bearer key for all `/api` routes except `/api/health` |
| `ANTHROPIC_API_KEY` | no | — | Enables `/api/generate` (word problems, logic, saga stories). Without it, `/api/health` reports `ai:false` and generation returns 503 — everything else works |
| `AI_MODEL` | no | `claude-sonnet-5` | |
| `PORT` | no | `8083` | Render sets its own `$PORT`, which the app reads |

## Content seeding

The two AI skills (`word_problems`, `logic`) and the five saga stories start
empty. Fill them with:

```sh
MATHGAMES_BASE_URL=http://localhost:8083 \
MATHGAMES_API_KEY=x \
scripts/seed-content.sh
```

This tops up each skill×level to ~40 non-retired questions and rewrites all
saga story text. It's idempotent — safe to re-run to top up a bank after
retiring bad questions in the parent view. Requires `ANTHROPIC_API_KEY` to
be set on the target server.

## Deployment

Deployed to Render from the `Dockerfile`. Cloud resources (the Postgres
instance, the web service, environment variables) are created by hand, not
scripted — see [DEPLOY.md](DEPLOY.md) for the click-through steps.

Migrations run automatically on boot. Render's disk is ephemeral, so the
backup story is downloading `GET /api/export` (a full-DB JSON dump) from the
**Backup** section of the hidden parent scorecard at `#/parents` — do this
periodically, since there's no other durable copy.

## Parent scorecard

`#/parents` is a hidden route (no tab, no PIN) behind the same bearer key —
per-day activity, per-skill accuracy/trend, recent misses, AI content bank
status with a Generate button, question review/retire, settings
(daily count, per-skill level overrides), and the export download.

## Parked: earned screen-time ledger

Not built in rev 1. The idea: banked minutes (earned from XP/streaks/daily
completion) redeemable for other iPad time. Nothing is lost by deferring —
every attempt already records `elapsed_ms`, grouped into `sessions`, so the
raw data needed to compute earned minutes retroactively already exists in
the schema (see `attempts`, `sessions`, `daily_results` in
[ARCHITECTURE.md](ARCHITECTURE.md)). Open questions and design notes for
when this gets picked up live in [TODO.md](TODO.md).
