# Phase 6 — Polish, seed, deploy

Read `CLAUDE.md` and `ARCHITECTURE.md` first. The app is feature-complete;
this phase makes it shippable and shipped.

## Tasks

1. **Feel pass** (drive interactively with Jim): timing of feedback moments
   (XP flyup speed, explanation dwell, celebration length), sound levels,
   keypad ergonomics in both orientations, phone-size sanity check, any copy
   that reads flat. Small tweaks only — no new features.
2. **Content seed**: run `scripts/seed-content.sh` against the target DB to
   fill word_problems + logic (levels 1–10, ~40 each) and all saga stories.
   Spot-check ~20 questions in the parent review UI; retire any duds.
3. **Empty/error states**: no-AI-key messaging in the parent generate UI,
   `bank_low` nudge in play, offline banner when `/api` is unreachable
   (the shell still loads from sw cache), graceful 401 → re-prompt for key.
4. **README.md**: what the app is, screenshots section (placeholder), local
   dev (scratch-DB smoke command), env vars, deploy notes, backup story
   (`/api/export` from `#/parents`), and the parked screen-time feature
   with a pointer to the raw data that will power it.
5. **Render deploy**: create the web service from the Dockerfile + a
   Render Postgres; set `DATABASE_URL`, `MATHGAMES_API_KEY` (long random),
   `ANTHROPIC_API_KEY`, `AI_MODEL`. Verify health, run the seed script
   against production (this is the one sanctioned production write), install
   on the actual iPad, play a session, check `#/parents`, download an
   export.

## Out of scope

New game features, screen-time ledger, multi-user anything.

## Acceptance checklist

- `go build ./... && go test ./...` passes.
- Production URL: health OK; PWA installs on the iPad from Safari; a real
  session, the daily challenge, an unlock, and the parent view all work on
  device with sound.
- Question bank ≥ 40 per AI skill×level in production; saga stories read
  well.
- Export downloaded from production and spot-checked.
- README accurate; final commit tagged `v1.0.0`.
