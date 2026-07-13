# TODO — Future Work

Not scheduled, not scoped in detail — just a running list so ideas survive
between sessions. Pull an item into a real plan when it's time to build it.

## Bugs / hardening

- **Daily challenge double-counting.** `applyDailyProgress`
  ([internal/game/service.go](internal/game/service.go)) has no guard against
  answering the same `question_id` twice — `Answered`/`Correct` just
  increment blindly. If a session is abandoned mid-daily (tab closed, app
  killed) and resumed, `GET /api/daily` re-serves the full question list
  with no way to tell the PWA which ones were already answered, so replaying
  can inflate the day's counts and streak. Needs either a per-question
  answered-set on `daily_results` (schema change) or a check against
  existing `attempts` rows for that session before crediting progress.
  Low priority while play is single-sitting in practice, but worth closing
  before the earned-screen-time ledger depends on accurate daily counts.

## Earned screen-time ledger (parked in ARCHITECTURE.md, rev 2+)

The schema already captures what this needs (`attempts.elapsed_ms`,
`sessions`, `daily_results`) — nothing is lost by building it later. When it
happens:

- **Time restrictions**: parent-configurable limits on daily/session play
  time, enforced client- or server-side (TBD which — server-side is more
  tamper-resistant against a kid editing localStorage).
- **Earning more time**: convert practice (XP, correct answers, daily
  completion, streaks — exact formula TBD) into banked minutes redeemable
  for other iPad time/games. Needs a redemption mechanism outside this app
  (parental controls integration? a manual ledger a parent checks?) since
  this app has no way to actually unlock other apps on the iPad.
- Depends on resolving the double-counting bug above first, since banked
  minutes are only as trustworthy as the attempt/session data feeding them.
- Needs a design decision on where the ledger balance displays (Home? a new
  screen? parents-only?) and whether Skylar sees his own balance or it's a
  parents-only mechanism revealed at their discretion.

## Ideas (unsorted, no commitment)

- Story text / saga chapter generation isn't exposed in the `#/parents` AI
  bank UI (only word_problems/logic batch generation is) — currently only
  reachable via `scripts/seed-content.sh` or a raw `POST /api/generate
  {kind:"story"}`. Low priority since sagas are seeded once and rarely need
  regenerating.
- Daily challenge resume UX could show which specific questions are already
  answered once the double-counting fix lands (right now the "Continue"
  card just says X/N done with no per-question detail).
