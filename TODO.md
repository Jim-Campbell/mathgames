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
  Low priority while play is single-sitting in practice. (The screen-time
  dial is unaffected — it counts `attempts` rows directly — but daily
  streak/counts integrity still wants this closed.)

## Screen time — future extensions

V1 (the dial) is designed and prompted — see Done and
build-prompts/feature-screen-time.md. Deliberately left out of v1:

- **Richer earning rules**: bonus minutes for daily-challenge completion,
  streaks, or random events (a Lucky Egg that doubles minutes too?). One
  flat per-correct rate for now; revisit once the family has lived with
  the deal for a while.
- **In-app time limits**: parent-configurable caps on this app's own play
  time (server-enforced). Different feature from earning; still parked.
- **Multi-hour banking**: the dial caps at 60 and pauses earning. If that
  cap chafes in practice (he fills it faster than parents reset it),
  consider letting redeemed-but-unspent hours accumulate in the log
  instead of raising the cap.

## Ideas (unsorted, no commitment)

- Story text / saga chapter generation isn't exposed in the `#/parents` AI
  bank UI (only word_problems/logic batch generation is) — currently only
  reachable via `scripts/seed-content.sh` or a raw `POST /api/generate
  {kind:"story"}`. Low priority since sagas are seeded once and rarely need
  regenerating.
- Daily challenge resume UX could show which specific questions are already
  answered once the double-counting fix lands (right now the "Continue"
  card just says X/N done with no per-question detail).

## Random events — future event kinds

The engine (build-prompts/feature-random-events.md) is a weighted registry;
each new banner-style XP event is one struct literal, zero PWA work. Ideas
for after Lucky Egg ×2 ships:

- **Lucky Egg ×3 / ×4** — rarer, bigger multipliers (weight them low; the
  escalation is canon).
- **Full Restore** — fires on a *wrong* answer occasionally instead: the
  miss doesn't break the streak ("Full Restore! Your streak is safe!").
  Needs a small engine extension (events on incorrect answers, non-XP
  effect).
- **Team Rocket blast-off** — pure comedy, no mechanical effect, just a gag
  overlay ("TEAM ROCKET'S BLASTING OFF AGAIN!"). Cheap, and comic relief
  events make the real ones feel bigger.
- **Poké Radar blip** — grants progress toward a gym badge or reveals a
  hint about the next unlock. Needs unlock-engine integration.
- **Teleport** — skip straight to the next level-up celebration if he's
  within N XP (top off the window). Flashy but touches the adaptive
  ladder — design carefully or skip.
- **Rival's challenge** — "Bet you can't get the NEXT one right in under
  10 seconds." Beat it → ×4 and a grudging "…not bad."; miss → nothing
  lost. Needs a pending-challenge state the engine remembers for one
  attempt; the did-he-rise-to-it data is scorecard gold.
- **Z-Move charge-up** — the next 3 correct answers each "charge the
  move," then it unleashes their combined XP again as a bonus. A mini-arc
  instead of a moment; same pending-state machinery as the rival's
  challenge.
- **Exp. Share surge** — all XP ×2 for the next 60 seconds with a
  countdown aura. Most gameplay-warping (timed buff state, session pacing);
  build last.

## Feature brainstorm (Claude, 2026-07-13)

Grouped, roughly ordered by bang-for-buck within each group. None scoped.

### New game modes (reuse existing generators + scoring)

- **Gym Leader battles.** A Gym Leader (Brock, Misty, Erika, Koga, Blaine —
  matching the Pewter/Cerulean/Celadon/Fuchsia/Cinnabar arcs) with an HP
  bar; correct answers land hits (damage scaled by XP earned, so
  speed/streak matter), wrong answers mean the Gym Leader hits back. Beat
  them before your own HP runs out. Pure presentation over the existing
  attempt loop — no new question machinery — and the highest-energy way to
  reuse the arc bosses. Could gate arc chapter completion ("defeat Brock")
  instead of the current bare counter.
- **Quick Attack sprint.** 60-second blitz: as many facts as possible in
  one skill, vs his own best score. Fact fluency is the one place raw
  speed drilling genuinely helps, and "beat your own record" is
  self-renewing content at zero content cost.
- **Evolution questions.** Two skills fused into one question
  (multiplication inside a fraction-of-a-quantity, place value inside
  addsub chains) with an evolution-flash reveal and bonus XP. Template
  generators can compose; also a natural AI-batch kind. Good stretch
  material for when single skills get easy.
- **Legendary problem of the week.** One very hard, optional problem (2–3
  levels above his current ladder), big reward, one attempt, resets weekly.
  Cheap to build on the daily-challenge plumbing and gives the "gifted kid
  wants a real fight" itch a place to go that doesn't distort the adaptive
  ladder.

### Learning depth (the pedagogy payoff)

- **Redemption queue (spaced repetition on misses).** Missed questions come
  back for another shot after 1 day, then 3, then 7 (deterministic
  SM-2-lite; integer day intervals; clear on two consecutive correct).
  Attempts already store everything needed. Frame it as a training montage:
  a trainer who studies a loss comes back stronger — retrying old misses
  pays comeback-style bonus XP. Probably the single highest-learning-value
  item on this list.
- **Misconception tagging.** Classify wrong answers deterministically where
  the error is recognizable — forgot the carry, remainder off by one,
  compared numerators instead of cross-multiplying, off-by-one-term in a
  sequence. Generators know the right answer's structure, so many wrong
  answers are diagnosable at grade time. Surface patterns in `#/parents`
  ("8 of his last 10 subtraction misses are borrow errors") — that's the
  scorecard becoming actionable instead of just descriptive.
- **Hints from Professor Oak.** Optional hint button that costs XP (so it's
  a tradeoff, not a crutch): first hint = strategy nudge, second = first
  step worked. Pre-generate hint text at AI-batch time for AI questions;
  template skills can derive hints from the generator's own working.
  Record hint usage on attempts — it's great signal for the scorecard and
  future difficulty tuning.
- **Number-line input kind.** A `numline` payload kind: tap where 3/7 (or
  638 rounded to hundreds) lives on an SVG number line, graded by integer
  tolerance bands. Estimation is a genuinely different mental skill than
  computation, it's very iPad-native, and it unlocks a whole family of
  fraction/place-value questions the current input kinds can't ask.

### Content & AI (build on the batch pipeline)

- **Skyler-authored problems.** He writes his own word problem; an AI batch
  call validates/solves it; accepted ones enter the bank flagged
  `author: skyler` and can show up in *the parents'* challenge (see below)
  or his own future dailies. Writing a good problem is harder than solving
  one — perfect for a voracious reader, and it's the app's first
  creative/authorship loop.
- **Professor chat in `#/parents`.** Natural-language questions over the
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

- **Ghost races.** Uncle Jim (or Dad) plays the same daily set once; Skyler
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
- **Evolution transformations.** At major XP thresholds his trainer
  avatar/aura on Home evolves (base → evolved → mega …) and the
  home-screen ambience shifts. The XP number already animates; this gives
  the grind a visible identity payoff between Pokédex unlocks.

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

## Video clips — future triggers

V1 (build-prompts/feature-video-clips.md) plays a clip on a random roll on
any answer, with per-clip correct/wrong tags. The condition model is built
to carry more triggers Jim mentioned adding later — wire these into the same
clip-eligibility model rather than new systems:

- **Milestone triggers**: play a specific clip on a level-up, an XP
  threshold, a Pokémon unlock, a daily-challenge completion, or a gym-arc
  chapter beaten. Needs a `trigger`/condition field on `clips` richer than
  the current two booleans (e.g. a tag set), plus firing hooks at those
  moments (the unlock/level-up sites already exist in the service).
- **Streak triggers**: a clip at a streak milestone (10 in a row → "Uncle
  Jim is proud").
- **Scheduled/first-of-day**: a clip on the first correct answer of the day.
- **Bearer-authed streaming proxy** (hardening): serve clips through
  `/api/clips/{id}/video` with range-request support instead of a public R2
  URL, if clip privacy ever matters more than it does for a personal family
  app. Non-trivial (HTTP range handling).

## Messages — future extensions

V1 (build-prompts/feature-messages.md) is one-way: Skyler → Jim, by email +
a parents inbox. Later:

- **Two-way replies**: Jim answers from the parents inbox and Skyler sees a
  note on Home ("Uncle Jim wrote back!"). Pairs well with the video-clip
  viewer — a reply could be a recorded clip instead of text.
- **Screenshot attach on bug reports**: canvas-capture the current screen and
  attach it (R2 is already wired for video; same upload path).
- **Voice notes**: record a short audio message instead of typing (journal's
  PWA has the mic pattern).
- **Auto bug context**: attach the last few attempt IDs / the current
  question so a "this problem is wrong" report is actionable without
  guessing.

## Done

- **Messages v1** — prompt written 2026-07-18
  (build-prompts/feature-messages.md). Kid-facing "📮 Message Uncle Jim"
  compose on Home (bug/idea/message chips, auto-attached version+screen
  context); server saves every message then emails it via Gmail SMTP app
  password (stdlib `net/smtp`, no dependency), gated like AI/R2; parents
  inbox with read/email status; 10/hour rate limit; recipient is env-only,
  never from the request.
- **Pokémon retheme** — prompt written 2026-07-14
  (build-prompts/retheme-pokemon.md). Full reskin DBZ → Pokémon (Skyler
  doesn't know DBZ): fighters→Pokédex, 7 dragon balls→8 gym badges,
  Shenron wish→Master Ball catch, sagas→gym arcs, events reflavored
  (Kaio-ken→Lucky Egg, etc.), power level→XP, zenkai→comeback, ki-gauge→
  Poké Ball meter. Reskin only — all math/thresholds/probabilities
  preserved (sole mechanic change: 7→8 collectibles).
- **Screen-time daily auto-reset** — prompt written 2026-07-14
  (build-prompts/feature-screentime-daily-reset.md). Dial auto-resets to 0
  on first app use each local day (a `reason='daily'` reset row); manual
  parent reset unchanged. Device supplies the local day; idempotent per day.
- **Video clips v1** — prompt written 2026-07-14
  (build-prompts/feature-video-clips.md). Jim uploads personal clips
  (Cloudflare R2, reusing the sibling apps' credentials) via a hidden
  `#/clips` manage route; each clip is tagged for correct/wrong triggers
  with a weight + enabled flag. A random per-answer roll (default 1 in 40,
  per-session cap 2, both tunable) plays a clip in a tap-to-play in-app
  viewer — the roll is decoupled from the XP event engine and fires on
  wrong answers too. `clip_plays` records every play.
- **Screen-time dial v1** — prompt written 2026-07-14
  (build-prompts/feature-screen-time.md). Correct answers fill a ki-gauge
  on Home at `minutes_per_correct` (default 3, settings knob), capped at
  60; value derived from `attempts` since the last reset, never stored;
  parents view gets a confirm-guarded Reset plus a Screen Time Log subpage
  (reset history + current period). App is the meter, parents enforce
  redemption.
- **Random events batch 2: Ultra Instinct + Bulma's capsule + Elder Kai** —
  prompt written 2026-07-14
  (build-prompts/feature-random-events-2.md). Adds the per-event
  eligibility predicate (Ultra Instinct: ×3 on answers under the
  speed-bonus threshold; Elder Kai: ×2 on slow answers past the ok
  threshold) and the flat-XP bonus field (capsule: +100 ⚡), plus per-slug
  overlay palettes/sounds in the PWA.
