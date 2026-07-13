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

## Feature brainstorm (Claude, 2026-07-13)

Grouped, roughly ordered by bang-for-buck within each group. None scoped.

### New game modes (reuse existing generators + scoring)

- **Boss battles.** A villain with an HP bar; correct answers land hits
  (damage scaled by XP earned, so speed/streak matter), wrong answers mean
  the boss hits back. Beat him before your own HP runs out. Pure
  presentation over the existing attempt loop — no new question machinery —
  and the highest-energy way to reuse saga villains. Could gate saga chapter
  completion ("defeat Frieza") instead of the current bare counter.
- **Kamehameha sprint.** 60-second blitz: as many facts as possible in one
  skill, vs his own best score. Fact fluency is the one place raw speed
  drilling genuinely helps, and "beat your own record" is self-renewing
  content at zero content cost.
- **Fusion questions.** Two skills fused into one question (multiplication
  inside a fraction-of-a-quantity, place value inside addsub chains) with a
  fusion-dance reveal and bonus XP. Template generators can compose; also a
  natural AI-batch kind. Good stretch material for when single skills get
  easy.
- **Legendary problem of the week.** One very hard, optional problem (2–3
  levels above his current ladder), big reward, one attempt, resets weekly.
  Cheap to build on the daily-challenge plumbing and gives the "gifted kid
  wants a real fight" itch a place to go that doesn't distort the adaptive
  ladder.

### Learning depth (the pedagogy payoff)

- **Redemption queue (spaced repetition on misses).** Missed questions come
  back for another shot after 1 day, then 3, then 7 (deterministic
  SM-2-lite; integer day intervals; clear on two consecutive correct).
  Attempts already store everything needed. Frame it as DBZ canon: "a Saiyan
  gets stronger from every defeat" — retrying old misses pays zenkai-style
  bonus XP. Probably the single highest-learning-value item on this list.
- **Misconception tagging.** Classify wrong answers deterministically where
  the error is recognizable — forgot the carry, remainder off by one,
  compared numerators instead of cross-multiplying, off-by-one-term in a
  sequence. Generators know the right answer's structure, so many wrong
  answers are diagnosable at grade time. Surface patterns in `#/parents`
  ("8 of his last 10 subtraction misses are borrow errors") — that's the
  scorecard becoming actionable instead of just descriptive.
- **Hints from Master Roshi.** Optional hint button that costs XP (so it's a
  tradeoff, not a crutch): first hint = strategy nudge, second = first step
  worked. Pre-generate hint text at AI-batch time for AI questions;
  template skills can derive hints from the generator's own working.
  Record hint usage on attempts — it's great signal for the scorecard and
  future difficulty tuning.
- **Number-line input kind.** A `numline` payload kind: tap where 3/7 (or
  638 rounded to hundreds) lives on an SVG number line, graded by integer
  tolerance bands. Estimation is a genuinely different mental skill than
  computation, it's very iPad-native, and it unlocks a whole family of
  fraction/place-value questions the current input kinds can't ask.

### Content & AI (build on the batch pipeline)

- **Skylar-authored problems.** He writes his own word problem; an AI batch
  call validates/solves it; accepted ones enter the bank flagged
  `author: skylar` and can show up in *the parents'* challenge (see below)
  or his own future dailies. Writing a good problem is harder than solving
  one — perfect for a voracious reader, and it's the app's first
  creative/authorship loop.
- **Sensei chat in `#/parents`.** Natural-language questions over the
  attempt data ("what should we practice this week?", "is he actually
  faster at division than last month?") via a small read-only tool loop —
  copy `~/projects/finance/internal/finance/assistant.go` wholesale; the
  tools are just aggregations over `attempts`/`skill_state`. The finance
  app already proved this pattern cheap and useful.
- **Auto-refill the AI bank.** When `bank_low` fires at serve time, enqueue
  a background generation batch for that skill×level instead of only
  nudging in the UI. One goroutine + the existing generate path; keeps the
  bank invisible-full without anyone driving to `#/parents`.

### Family & motivation

- **Ghost races.** Uncle Jim (or Dad) plays the same daily set once; Skylar
  races the recorded per-question times as a "ghost" alongside his own run.
  Async multiplayer with zero account infrastructure — just store a second
  run per day keyed to a name. Family rivalry is the strongest free
  motivation available to this app.
- **Share card for the daily.** Wordle-style emoji grid (🟩🟥 + time +
  streak) via the Web Share API / clipboard, so he can text Dad the result.
  He already has the Wordle sharing habit; borrow it. ~30 lines of JS.
- **Parent-composed missions.** In `#/parents`: pick skills, difficulty,
  count, and a custom reward line ("beat this and we get ice cream") → it
  appears on Home as a special mission. Reuses quest plumbing; gives
  parents a lever between "nothing" and the parked screen-time ledger.
- **Super Saiyan transformations.** At major power-level thresholds his
  avatar/aura on Home transforms (base → SSJ → SSJ2 …) and the home-screen
  ambience shifts. The power-level number already animates; this gives the
  grind a visible identity payoff between fighter unlocks.

### Platform & infra

- **Offline question packs.** Pre-fetch a sealed pack of questions for
  car rides; queue attempts locally and sync on reconnect. Tension to
  resolve first: answers-never-leave-the-server is a core invariant, so
  offline grading means shipping answers (obfuscated at best) to the
  client — decide whether offline is worth relaxing the invariant before
  building anything.
- **Weekly automated export snapshot.** Cron-style weekly `/api/export`
  dump like finance's `internal/backup` (banner while it runs, status +
  manual run + download in `#/parents`). Render disks are ephemeral, and
  "Jim remembers to click export" is not a backup story once real history
  accumulates.
- **Voice answers.** Web Speech API mic for numeric answers (journal's PWA
  has the `webkitSpeechRecognition` pattern to copy). Saying "sixty-three"
  beats the keypad for pure fact drills; worth a spike to see if iOS
  Safari's recognition is fast enough for sprint mode.
- **Multi-kid support.** If cousins want in someday: player column on
  attempts/skill_state/unlocks, a player picker at launch, no auth beyond
  the existing key. Big enough to warrant a real design pass — parked here
  so it doesn't sneak in half-done.
