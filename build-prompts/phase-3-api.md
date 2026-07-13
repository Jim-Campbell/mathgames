# Phase 3 — API: sessions, serving, attempts, daily, profile, quests, parents, export

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first. This phase wires the
phase-2 service to the complete HTTP surface in ARCHITECTURE.md → "API".
Everything is testable with `curl` — no PWA yet.

## Reference implementations

- `~/projects/food/internal/api/food.go` — handler layout, JSON
  request/response helpers, error envelope
- `~/projects/finance/internal/api/` — the same, richer query-param handling

## Tasks

1. Implement every endpoint in ARCHITECTURE.md → "API" except
   `POST /api/generate` and the `/api/questions` review endpoints (phase 4).
   Notes:
   - `GET /api/next` **must strip `answer` and `explanation`** from the
     serialized questions. Add a serializer test that fails if either field
     ever appears.
   - `POST /api/attempts` returns the full AttemptResult (correct, answer,
     explanation, xp_earned, zenkai, streak, skill_level, level_changed,
     power_level before/after, unlocks).
   - `GET /api/daily?day=` creates + pins the day's set on first fetch
     (`daily_results` row), returns questions if incomplete, results +
     calendar + streak always. Reject attempts on a daily question already
     answered (409).
   - Quest chapters: `requirement` gating, in-order unlocking, progress
     updated inside the attempt flow (already in the phase-2 service —
     surface it).
   - `POST /api/wish`: 409 unless exactly 7 dragon-ball unlocks exist;
     grants the fighter, +1000 XP, deletes the ball rows — in one
     transaction.
   - `GET /api/parents/summary?days=`: aggregates per ARCHITECTURE.md —
     accuracy in **basis points**, minutes derived from session
     start/end (fallback: sum of elapsed_ms), median elapsed_ms per skill,
     7-day trend (sign of accuracy_bp change vs the prior 7 days),
     recent misses (last 20 wrong attempts with prompt/given/answer),
     AI bank counts per skill×level.
   - `GET /api/export`: every table as JSON, streamed, with a
     `Content-Disposition` download filename including the date.
2. Handler tests for the tricky ones: next-strips-answers, daily 409 on
   re-answer, wish 409/success, parents summary math (fixed fixture data →
   hand-checked basis points and median).

## Out of scope

AI generation endpoints (phase 4), PWA (phase 5).

## Acceptance checklist

- `go build ./... && go test ./...` passes.
- Against a scratch DB (phase-1 smoke command), with `curl -H "Authorization: Bearer x"`:
  - `POST /api/sessions {"mode":"training"}` → session id.
  - `GET /api/next?skill=multiplication&count=3` → 3 questions, **no
    `answer` or `explanation` keys anywhere in the JSON**.
  - `POST /api/attempts` with a correct answer → `correct:true`, plausible
    `xp_earned`, `power_level` increased; with a wrong answer →
    `correct:false`, `xp_earned:1`, answer + explanation present.
  - Ten correct attempts in a row at one skill move `streak` to 10 and the
    XP multipliers step up at 3 and 6 (eyeball the values).
  - `GET /api/daily?day=2026-01-15` twice → identical `question_ids`.
  - `GET /api/profile` reflects the XP just earned; `GET /api/collection`
    shows any threshold fighter unlocked.
  - `GET /api/parents/summary` returns sane aggregates for the activity
    just created.
  - `GET /api/export` downloads JSON containing the attempts just made.
- `dropdb mathgames_smoke` at the end.
