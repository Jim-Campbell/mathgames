# Phase 4 — AI content: Anthropic client, batch generation, validation, seed script

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first — especially "AI content
generation". This phase fills the question bank for the two AI skills
(`word_problems`, `logic`) and writes the saga stories. The AI generates
content in reviewed batches; it never grades or writes game state.

## Reference implementations

- `~/projects/food/internal/ai/client.go` — copy the hand-rolled Anthropic
  HTTP client near-verbatim (no SDK). Single non-agentic calls here — no
  tool loop needed.

## Tasks

1. `internal/ai/client.go`: the copied client; model from `AI_MODEL`
   (default `claude-sonnet-5`); missing `ANTHROPIC_API_KEY` → constructor
   returns a disabled client and `/api/health` reports `ai:false`.
2. `internal/ai/generate.go`: one generation call per batch, per
   ARCHITECTURE.md → "AI content generation":
   - System prompts per kind (`word_problems`, `logic`, `story`) embedding
     Skylar's profile, the difficulty rubric for the requested level, the
     exact payload/answer JSON shapes, and the last 50 prompts at that
     skill×level for repeat avoidance.
   - Word problems must include a `check` field — an integer arithmetic
     expression (e.g. `"34*12+50"`) that must equal the numeric answer.
   - Logic puzzles: kinds `mc` or `text`; `display.grid` /
     `display.sequence` when visual.
   - Story batches: rewrite `quest_chapters.title` + `story` for one saga
     per call — adventurous, funny, reading-level generous (he's a reader),
     ~120 words per chapter, each chapter ending with a hook into its
     `requirement` ("Vegeta blocks the path — land 12 multiplication hits!").
3. `internal/ai/validate.go` (deterministic, unit-tested against recorded
   fixtures — one good response, one with malformed items): JSON parses;
   known kind; answer shape matches kind; MC index in range; fraction den >
   0; prompt ≤ 500 chars; the `check` expression — evaluated by a small
   integer expression evaluator (`+ - * / ( )`, integer division must be
   exact) — equals the answer. Rejects counted in `ai_batches.rejected`;
   full raw response stored in `ai_batches.raw`.
4. Endpoints: `POST /api/generate {kind, skill, difficulty, count}` (503
   when disabled; inserts accepted questions / rewrites chapter stories,
   returns `{accepted, rejected, batch_id}`);
   `GET /api/questions?skill=&source=&retired=` and
   `POST /api/questions/{id}/retire|unretire` for the parent review flow.
   Retired questions never get served (the phase-2 picker already filters).
5. `scripts/seed-content.sh`: loops `POST /api/generate` to fill
   `word_problems` and `logic` to ~40 non-retired questions per level 1–10,
   then story batches for all 5 sagas. Idempotent-ish: skips skill×levels
   already at target (read counts from `/api/parents/summary` bank data or a
   dedicated count query). Also `migrations/002_quests.sql`: seed the 5
   sagas × 4 chapters with placeholder titles/stories and real
   requirements/rewards per ARCHITECTURE.md (fighter + dragon-ball rewards
   spread across sagas so all 7 balls are earnable).

## Out of scope

PWA (phase 5). No agentic tool loops. No per-question runtime AI calls.

## Acceptance checklist

- `go build ./... && go test ./...` passes; validation tests cover the
  recorded fixtures including a bad `check` expression being rejected.
- Without `ANTHROPIC_API_KEY`: server starts, health says `ai:false`,
  `POST /api/generate` → 503, everything else unaffected.
- With the key, against a scratch DB:
  `curl -X POST .../api/generate -d '{"kind":"word_problems","skill":"word_problems","difficulty":3,"count":5}'`
  → `accepted` ≥ 3; the questions read like real 2–3 step story problems
  (eyeball them); `GET /api/next?skill=word_problems` serves them minus
  answers.
- A story batch rewrites one saga's chapters; the text is fun and ends with
  the requirement hook (eyeball it).
- Retiring a question removes it from serving; unretire restores.
- Migration 002 applied cleanly on a fresh DB and on the existing one.
- `dropdb mathgames_smoke` at the end.
