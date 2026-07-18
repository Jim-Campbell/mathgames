# Math Games App — Architecture

Math + logic games app for **Skyler**, a gifted 8-year-old heading into 3rd
grade, targeting **4th-grade content** with adaptive difficulty. Primary
device: **iPad** (installed PWA). Single user. Sibling app to
`~/projects/food`, `~/projects/finance`, and `~/projects/journal` — same
stack, same patterns, same bearer-key auth. Read those repos' conventions
before inventing anything.

The theme is **Pokémon**: answering problems is "training", XP drives a
**Trainer XP** total, milestones unlock a Pokédex collection, story quests are
gym arcs, and collecting all eight gym badges earns the Master Ball for a
catch. This is a private, non-commercial family app; all artwork is
**original inline SVG** (stylized, not copied) — names and lore references
are used playfully.

Design goals, in order:

1. **Genuinely fun** — sounds, animation, collection pull, a daily
   Wordle-style ritual (he already solves Wordle).
2. **Adaptive** — he consumes content fast; difficulty must climb with him
   without a parent touching anything.
3. **Rich raw data** — every attempt with timings and wrong answers is kept
   forever so Jim + Claude can co-design new games and a parent scorecard can
   be honest.
4. **Extensible** — new skills, Pokémon, gym arcs, and game modes bolt on
   without schema surgery.

**Screen time.** Correct answers (any mode — training, quest, daily) fill a
Poké Ball energy meter on Home: `minutes_per_correct` (a settings knob,
default 3) per correct, capped at 60. The dial is **derived, never stored** — it's
`(count of correct attempts since the last reset) × minutes_per_correct`,
capped, computed from `attempts` rows on demand. A parent resets it from the
parents view once the time is spent, snapshotting a `screen_time_resets`
row (`reason='manual'`). The dial also **auto-resets to 0 on the first use of
the app each device-local day** — the first `GET /api/screentime?day=` (or
`POST /api/attempts` that includes `day`) of a new day inserts a
`reason='daily'` reset row snapshotting whatever was on the dial, same
mechanism, no parent involved. `screen_time_resets.day` (the device-local
`YYYY-MM-DD` the reset applies to) and `.reason` (`manual`|`daily`) drive
rollover detection; a partial unique index on `day` where `reason='daily'`
keeps it idempotent per day. The app never locks anything; it's the trusted
meter, and parents are the enforcers (iOS gives a PWA no way to control
other apps). One flat rate, no bonuses or multipliers — richer earning
rules are deliberately future work.

| field | meaning |
|---|---|
| `minutes_earned` | current dial value, capped at 60 |
| `minutes_cap` | 60 |
| `corrects_since_reset` | corrects counted toward the current dial |
| `minutes_per_correct` | the rate (settings knob) |
| `full` | true once pinned at the cap |
| `since_reset` | timestamp of the last reset, or null if never reset |

```
GET  /api/screentime?day=        → the dial (fields above), rolling the day over first if needed
POST /api/screentime/reset {day} → the created screen_time_resets row, reason='manual' (400 if dial is 0)
GET  /api/screentime/log         → resets, newest first, each with reason + day
```

## Tech stack

- Go 1.25, chi router, pgx/v5, PostgreSQL (Render-hosted in production)
- Vanilla JS **single-file PWA** in `pwa/` (index.html + manifest.json +
  sw.js) served by the Go binary
- Anthropic API via a **hand-rolled client copied from
  `~/projects/food/internal/ai/client.go`** (no SDK), gated on
  `ANTHROPIC_API_KEY` — used only for **batch content generation** (word
  problems, logic puzzles, saga story text), never in the answer loop. The
  app runs fully without the key; it just can't mint new AI content.
- **Cloudflare R2** for video-clip storage (see "Video clips" below) — the
  only other external service, off until its env vars are configured.

## Repository structure

```
├── cmd/server/          # entry point, env config, DI (mirror food)
├── internal/
│   ├── api/             # HTTP handlers, bearer auth + logging middleware, PWA serving
│   ├── db/              # pgx store + migrations (auto-run at startup, advisory-lock
│   │                    #   serialized — copy food's internal/db/migrate.go)
│   ├── game/            # domain types, Store interface, service; generators,
│   │                    #   answer checking, XP/streaks, adaptive ladder, unlocks,
│   │                    #   daily-set seeding — ALL deterministic logic lives here
│   └── ai/              # Anthropic client + batch content generation
├── pwa/                 # single-file vanilla JS PWA
├── docs/                # this file's companions if any grow later
├── build-prompts/       # numbered phase prompts
└── Dockerfile           # multi-stage golang:1.25-alpine → alpine (copy food's)
```

## Domain invariants (do not break)

- **No floats in stored data or scoring math.** XP, total XP, streaks,
  levels: integers. Times: integer **milliseconds**. Accuracy anywhere it's
  stored or computed: integer **basis points** (8750 = 87.50%). Multiply
  before dividing.
- **All deterministic logic lives in Go with unit tests**, including at least
  one hand-checked worked example per algorithm (XP formula, adaptive ladder,
  fraction equivalence, daily seeding). The AI generates *content*, never
  *outcomes*.
- **Answers never leave the server before an attempt.** `GET /api/next`
  serves question payloads without answers; grading happens in
  `POST /api/attempts`. (Skyler is exactly the kid who will find the network
  tab one day.)
- **Every attempt is stored raw**: full question snapshot reference, the
  answer given, correctness, elapsed ms, streak and level at that moment.
  Never aggregate away the raw rows.
- **Every question served is a DB row** — template-generated ones are
  inserted at serve time, AI ones at batch time — so attempts always have a
  real `question_id` to reference and bad AI questions can be retired.
- **The AI never writes game state.** Batch generation inserts `questions`
  and `quest_chapters` rows (with full raw output kept in `ai_batches`);
  humans review via the parent view; nothing else.
- JSON API, snake_case throughout. `/api/export` returns the full DB as JSON
  (Render disks are ephemeral; this is the backup story).

## Skills and difficulty

Skills are **code-defined** (a Go registry in `internal/game/skills.go`), DB
stores only per-skill state. Rev 1 skills:

| skill | source | what it is |
|---|---|---|
| `multiplication` | template | fact fluency → multi-digit |
| `division` | template | fact fluency → long division with remainders |
| `addsub` | template | multi-digit addition/subtraction with regrouping |
| `fractions` | template | compare, equivalents, add/sub like denominators, number line |
| `place_value` | template | read/compare/round numbers to 1,000,000 |
| `patterns` | template | number sequences: find the next term / missing term |
| `word_problems` | ai | 2–3 step story problems (his reading strength) |
| `logic` | ai | grid deduction, "which doesn't belong", balance puzzles |

Every skill has an integer **level 1–10**. Level→parameter maps for the
template generators (targets: L1–2 ≈ 3rd grade, L3–5 ≈ 4th grade, L6–8 ≈ 5th
grade, L9–10 ≈ 6th grade — he will get there):

- `multiplication` — L1: facts ≤ 5×5. L2: facts ≤ 9×9. L3: facts ≤ 12×12.
  L4: 2-digit × 1-digit. L5: multiples of 10 (30×70). L6: 2-digit × 2-digit.
  L7: 3-digit × 1-digit. L8: 3-digit × 2-digit. L9: 3 factors (4×7×25-style,
  associativity bait). L10: 4-digit × 2-digit.
- `division` — L1: inverses of ≤ 9×9 facts, exact. L2: ≤ 12×12 facts, exact.
  L3: 2-digit ÷ 1-digit exact. L4: 2-digit ÷ 1-digit **with remainder**
  (answer = quotient + remainder, two inputs). L5: 3-digit ÷ 1-digit exact.
  L6: 3-digit ÷ 1-digit with remainder. L7: dividing multiples of 10
  (4200÷70). L8: 3-digit ÷ 2-digit exact. L9: 3-digit ÷ 2-digit with
  remainder. L10: 4-digit ÷ 2-digit with remainder.
- `addsub` — L1: 2-digit ± 2-digit, no regrouping. L2: 2-digit with
  regrouping. L3: 3-digit with regrouping. L4: 3 addends of 2–3 digits.
  L5: 4-digit ± 4-digit. L6: subtraction across zeros (5003−1876).
  L7: 5-digit. L8: mixed chains (a+b−c). L9: missing-operand
  (___ − 2748 = 1265). L10: 6-digit chains.
- `fractions` — L1: which is bigger, same denominator. L2: fraction of a
  shape (SVG pie/bar rendered from the payload). L3: equivalents
  (2/3 = ?/12). L4: compare unlike denominators (cross-multiply mentally).
  L5: add/sub like denominators. L6: simplify to lowest terms. L7: mixed
  number ↔ improper. L8: add/sub unlike denominators (one denominator a
  multiple of the other). L9: fraction of a quantity (3/4 of 48). L10:
  add/sub any unlike denominators.
- `place_value` — L1: value of a digit in a 3-digit number. L2: 4-digit
  compare (< = >). L3: round to nearest 10/100. L4: expanded form to
  10,000. L5: round to nearest 1,000. L6: 6-digit compare and order.
  L7: round to any place in 6-digit numbers. L8: 7-digit read/write.
  L9: “what number is 10,000 more than…”. L10: mixed multi-step
  (round, then compare).
- `patterns` — L1: +k sequences (skip counting). L2: −k sequences.
  L3: ×2 / ×3 sequences. L4: two-step rules (×2 then +1). L5: alternating
  rules (+3, +5, +3, +5). L6: square numbers, triangle numbers. L7: missing
  middle term. L8: Fibonacci-style (each = sum of previous two). L9: mixed
  rule identification (choose the rule, multiple choice). L10: two
  interleaved sequences.
- `word_problems` / `logic` — AI-generated per level; the generation prompt
  receives the level and a rubric (see "AI content generation").

## Database schema (migration 001)

```sql
CREATE TABLE skill_state (
    skill          TEXT PRIMARY KEY,
    level          INT NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 10),
    xp             BIGINT NOT NULL DEFAULT 0,     -- lifetime XP earned in this skill
    streak         INT NOT NULL DEFAULT 0,        -- current consecutive-correct streak
    wrong_run      INT NOT NULL DEFAULT 0,        -- current consecutive-wrong run (comeback)
    window_total   INT NOT NULL DEFAULT 0,        -- attempts in current adaptive window
    window_correct INT NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE questions (
    id           BIGSERIAL PRIMARY KEY,
    skill        TEXT NOT NULL,
    difficulty   INT NOT NULL CHECK (difficulty BETWEEN 1 AND 10),
    source       TEXT NOT NULL CHECK (source IN ('template','ai')),
    payload      JSONB NOT NULL,   -- prompt, kind, choices, display hints — NO answer
    answer       JSONB NOT NULL,   -- canonical answer (never serialized to the client pre-attempt)
    explanation  TEXT NOT NULL DEFAULT '',
    ai_model     TEXT,
    ai_batch_id  BIGINT,           -- REFERENCES ai_batches(id), added below
    times_served INT NOT NULL DEFAULT 0,
    retired      BOOLEAN NOT NULL DEFAULT FALSE,  -- parent can retire bad AI questions
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX questions_pick_idx ON questions(skill, difficulty, source)
    WHERE NOT retired;

CREATE TABLE sessions (
    id         BIGSERIAL PRIMARY KEY,
    mode       TEXT NOT NULL CHECK (mode IN ('training','quest','daily')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at   TIMESTAMPTZ
);

CREATE TABLE attempts (
    id           BIGSERIAL PRIMARY KEY,
    session_id   BIGINT REFERENCES sessions(id),
    question_id  BIGINT NOT NULL REFERENCES questions(id),
    skill        TEXT NOT NULL,
    difficulty   INT NOT NULL,           -- difficulty at time of attempt
    given        JSONB NOT NULL,         -- exactly what he answered
    correct      BOOLEAN NOT NULL,
    elapsed_ms   INT NOT NULL,
    xp_earned    INT NOT NULL,
    streak_after INT NOT NULL,
    level_after  INT NOT NULL,           -- skill level after adaptive update
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX attempts_skill_idx ON attempts(skill, created_at);
CREATE INDEX attempts_session_idx ON attempts(session_id);

CREATE TABLE unlocks (
    id         BIGSERIAL PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('pokemon','gym_badge','ribbon')),
    ref        TEXT NOT NULL,            -- pokemon slug, badge number '1'..'8', ribbon slug
    source     TEXT NOT NULL DEFAULT '', -- human-readable: 'xp 9001', 'saga cerulean ch4', 'catch'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, ref)
);

CREATE TABLE quest_chapters (
    id           BIGSERIAL PRIMARY KEY,
    saga         TEXT NOT NULL,          -- 'pewter','cerulean','celadon','fuchsia','cinnabar' (in order)
    chapter      INT NOT NULL,           -- 1..N within the arc
    title        TEXT NOT NULL,
    story        TEXT NOT NULL,          -- AI-generated narrative shown on open
    requirement  JSONB NOT NULL,         -- {"correct": 12, "skills": ["multiplication","fractions"], "min_difficulty": 3}
    reward       JSONB NOT NULL,         -- {"xp": 500, "pokemon": "onix", "gym_badge": 2}  (any subset)
    progress     INT NOT NULL DEFAULT 0, -- correct answers counted toward requirement
    completed_at TIMESTAMPTZ,
    ai_batch_id  BIGINT,
    UNIQUE (saga, chapter)
);

CREATE TABLE daily_results (
    day          DATE PRIMARY KEY,
    question_ids BIGINT[] NOT NULL,      -- the 5 questions chosen for that day
    answered     INT NOT NULL DEFAULT 0,
    correct      INT NOT NULL DEFAULT 0,
    elapsed_ms   INT NOT NULL DEFAULT 0,
    xp_earned    INT NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ
);

CREATE TABLE ai_batches (
    id         BIGSERIAL PRIMARY KEY,
    kind       TEXT NOT NULL CHECK (kind IN ('word_problems','logic','story')),
    skill      TEXT,
    difficulty INT,
    model      TEXT NOT NULL,
    prompt     TEXT NOT NULL,            -- the full prompt sent (versioned by content)
    raw        JSONB NOT NULL,           -- full raw API response for later analysis
    accepted   INT NOT NULL DEFAULT 0,   -- rows that passed validation and were inserted
    rejected   INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE questions      ADD CONSTRAINT questions_batch_fk      FOREIGN KEY (ai_batch_id) REFERENCES ai_batches(id);
ALTER TABLE quest_chapters ADD CONSTRAINT quest_chapters_batch_fk FOREIGN KEY (ai_batch_id) REFERENCES ai_batches(id);

CREATE TABLE settings (                   -- single row, id = 1
    id             INT PRIMARY KEY CHECK (id = 1),
    daily_count    INT NOT NULL DEFAULT 5,
    level_override JSONB NOT NULL DEFAULT '{}',  -- {"multiplication": 6} parent pin/boost; empty = adaptive
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO settings (id) VALUES (1) ON CONFLICT DO NOTHING;
```

Migration 001 also seeds `skill_state` with one row per registered skill
(`INSERT ... ON CONFLICT DO NOTHING` for each skill slug — the Go registry is
authoritative; startup re-seeds any missing skill so adding a skill in code
needs no migration).

## Question payloads

`payload` (client-visible) and `answer` (server-only) shapes, by `kind`:

```jsonc
// numeric — one integer input (on-screen keypad)
{"kind":"numeric", "prompt":"27 × 34 = ?"}
   answer: {"value": 918}

// numeric2 — quotient + remainder (two inputs)
{"kind":"numeric2", "prompt":"87 ÷ 4 = ? remainder ?", "labels":["quotient","remainder"]}
   answer: {"values": [21, 3]}

// mc — multiple choice (2–5 choices)
{"kind":"mc", "prompt":"Which fraction is larger?", "choices":["3/8","2/5"]}
   answer: {"index": 1}

// fraction — numerator/denominator inputs; equivalent forms accepted
{"kind":"fraction", "prompt":"1/4 + 2/4 = ?"}
   answer: {"num": 3, "den": 4}   // grading: given.num*4 == 3*given.den (integer cross-multiply)

// text — short free text, case/space-insensitive compare (AI logic answers like "RED")
{"kind":"text", "prompt":"..."}
   answer: {"value":"red", "accept":["red","the red one"]}

// display (optional, any kind) — render hints for the PWA
{"display": {"fraction_bar": {"parts": 8, "shaded": 3}}}       // SVG bar/pie for visual fraction questions
{"display": {"sequence": [3, 6, 12, 24, null]}}                // pattern with blank slot
{"display": {"grid": {"rows": [...], "cols": [...]}}}          // logic grid puzzles
```

Grading (`internal/game/grade.go`, pure function, unit-tested):
`numeric`/`numeric2` exact integer match; `fraction` by integer
cross-multiplication (so 6/8 is accepted for 3/4 — reward equivalent forms,
the explanation shows lowest terms); `mc` by index; `text` by
lowercase/trimmed match against `value` + `accept`.

## Scoring (deterministic, `internal/game/score.go`)

All integer math, multiply before dividing:

- **Base XP** = `10 × difficulty` (difficulty 1–10 → 10–100).
- **Speed bonus**: each skill×difficulty has `fast_ms` and `ok_ms` thresholds
  (a table in Go; defaults `fast_ms = 5000 + 2000×difficulty`,
  `ok_ms = 3×fast_ms`). `elapsed_ms ≤ fast_ms` → ×150/100;
  `≤ ok_ms` → ×120/100; slower → ×100/100.
- **Streak multiplier** (applied after speed, on the running
  consecutive-correct streak *including* this answer): streak ≥ 11 → ×150/100;
  ≥ 6 → ×125/100; ≥ 3 → ×110/100; else ×100/100.
- **Comeback boost** (a Pokémon digging deep after a rough patch): if this
  correct answer follows a run of ≥ 3 consecutive wrong answers in the same
  skill, double the final XP. (`wrong_run` in `skill_state` tracks the run.)
- **Wrong answer** = 1 XP (showing up counts), streak resets to 0,
  `wrong_run` increments.
- **Daily challenge**: all XP ×2; a perfect day (all correct) adds +100.

Worked example (hand-check in `score_test.go`): difficulty 4 multiplication,
answered correctly in 9.2 s with streak reaching 7, no comeback. Base
`10×4 = 40`; `fast_ms = 13000` so 9200 ≤ fast → `40×150/100 = 60`; streak 7 →
`60×125/100 = 75`. **75 XP.** Same answer arriving after 3 straight misses:
`75×2 = 150`.

**Trainer XP** = `100 + total lifetime XP` (sum across skills). It only goes
up. The PWA celebrates threshold crossings; **9001** gets the big one (a
one-time legendary/evolution full-screen moment — the Charizard slot in the
Pokédex).

### Random events

A code-defined registry (`internal/game/events.go`, `[]Event{...}`) of rare
bonus moments on **correct** answers only. Each `Event` carries a
slug/name/message, a relative `Weight` for picking among multiple
registered events, an `XPNum`/`XPDen` integer multiplier
(`Apply(xp) = xp*XPNum/XPDen + XPFlat`, multiply before divide, then add the
flat bonus), and an optional `Eligible(elapsedMS, difficulty) bool`
predicate (nil means always eligible) gating whether the event may fire for
a given attempt.

| slug | weight | effect | condition |
| --- | --- | --- | --- |
| `lucky_egg` | 4 | ×2 | none |
| `rare_candy` | 3 | +100 XP flat | none |
| `slowpoke` | 2 | ×2 | `elapsedMS > okMS(difficulty)` (slow) |
| `critical_hit` | 2 | ×3 | `elapsedMS <= fastMS(difficulty)` (fast) |

`RollEvent(rng, attemptsSinceLast, elapsedMS, difficulty)` fires with
probability **1 in 25** (`eventChance = 25`) and never fires within **10
attempts** (correct or wrong) of the last attempt that fired one
(`eventCooldown = 10`, tracked via `attempts.event` and
`Store.AttemptsSinceLastEvent`). Once it decides to fire, the weighted pick
walks only the events whose `Eligible` predicate (if any) allows the
attempt's `elapsedMS`/`difficulty` — ineligible events are excluded from
the weighted pick entirely, so a fast answer can never roll `slowpoke` and
a slow one can never roll `critical_hit`; if no event is eligible,
nothing fires. The multiplier/flat bonus applies **last** — after speed,
streak, comeback, and the daily ×2 are already baked into `Score()`'s
result — so a daily lucky-egg answer stacks intentionally:
`(base×speed×streak×2 daily)×2 event`. The resulting XP then flows through
skill state, the attempt row, daily progress, and total XP with no
further special-casing.

`attempts.event` (migration 003) is a nullable TEXT column holding the slug
of the event that fired on that attempt, if any.

## Adaptive difficulty (deterministic, `internal/game/adapt.go`)

Per skill, a rolling window in `skill_state` (`window_total`,
`window_correct`), reset whenever the level changes:

- After each attempt: increment `window_total` (and `window_correct` if
  correct).
- When `window_total` reaches **10**: `window_correct ≥ 8` → level +1 (cap
  10); `window_correct ≤ 4` → level −1 (floor 1); otherwise stay. Reset the
  window to 0/0 in all three cases.
- A `settings.level_override` entry for the skill pins the served level;
  adaptive state still updates underneath so removing the pin resumes where
  he really is.

Worked example (hand-check in `adapt_test.go`): fresh skill at L3; sequence
`C C C W C C C C C C` → window 10, correct 9 → **promote to L4**, window
resets; next ten `W W C W W C W W W C` → correct 3 → **back to L3**.

Level-up is a celebration moment (aura flash + fanfare); level-down is
silent — the app just quietly serves easier questions. Never show a kid
"demoted".

## Question serving

`GET /api/next?skill=X&count=N` (or `skill=mixed`):

- Effective level = override or `skill_state.level`.
- **Template skills**: generate N fresh questions via the skill's generator
  at that level (seeded from `crypto/rand`), insert them as `questions` rows
  (`source='template'`), serve payloads (id, skill, difficulty, payload —
  **never** answer/explanation).
- **AI skills**: pick the N least-served non-retired `questions` rows at
  (skill, level) — `ORDER BY times_served, random()` — bump `times_served`.
  If the bank at that level has < N rows, fall back to nearest level
  (±1, then ±2 …) and report `bank_low: true` in the response so the PWA can
  nudge (and the parent view can prompt a generation run).
- `skill=mixed` (the default "Training" mode): weighted round-robin across
  all skills, weighting toward the skill with the *least* attempts in the
  last 7 days (keeps him from grinding only his favorite).

`POST /api/attempts` grades, computes XP, updates `skill_state` (streak,
wrong_run, window, level, xp), updates quest progress and daily state if the
session mode says so, detects new unlocks, and returns everything the PWA
needs for the moment of feedback:

```json
{
  "correct": true, "answer": {"value": 918}, "explanation": "27×34 = 27×30 + 27×4 = 810 + 108",
  "xp_earned": 75, "comeback": false,
  "streak": 7, "skill_level": 4, "level_changed": 0,
  "xp": 9120, "xp_before": 9045,
  "unlocks": [{"kind":"pokemon","ref":"gengar","name":"Gengar","rarity":"epic"}]
}
```

## Collection, quests, daily

**Pokédex** (`internal/game/pokedex.go`, code-defined catalog — DB stores
only unlocks). ~23 Pokémon with slug, name, rarity
(`common/rare/epic/legendary`), an original inline-SVG portrait (drawn in the
PWA phase; stylized silhouettes + type-glow, not copied art), and an unlock
condition. Unlock sources:

- **XP thresholds** (Pidgey 500, Rattata 1,000, Caterpie 2,000, Eevee 3,000,
  Growlithe 4,000, Charizard 9,001 — the signature moment, Psyduck 12,000,
  Gyarados 15,000, …, Mewtwo 250,000). Exact table lives in the Go catalog.
- **Gym-arc chapter rewards** (strong Pokémon join when their arc is beaten:
  Onix, Alakazam, Lapras…).
- **Streak ribbons** (daily-challenge calendar streaks: 3, 7, 14, 30 days).
- **Master Ball catches**: earning all 8 gym badges lets him use the Master
  Ball (`POST /api/catch` with a chosen locked Pokémon slug) — grants any
  Pokémon regardless of condition, +1000 XP, and consumes the badges
  (deletes the 8 `gym_badge` unlock rows; they can be earned again).

**Quests** — 5 gym arcs × 4 chapters, seeded in migration 007 with
placeholder titles/requirements; story text is AI-generated (`kind='story'`
batches rewrite `quest_chapters.story`). A chapter's `requirement` counts
correct answers in `quest` sessions at `min_difficulty`+ in the listed
skills; progress updates on every qualifying attempt; completion grants the
reward and unlocks the next chapter. Chapters must be done in order within
an arc; arcs unlock in order.

**Daily challenge** — one set per calendar day (device-local date sent by the
PWA as `?day=`), `settings.daily_count` questions (default 5): one from each
of 4 rotating template skills at current level + 1 from an AI skill.
Selection is **seeded by the date** (FNV-1a hash of "2026-07-13" →
`math/rand.New(source)`) so re-opening the app the same day shows the same
set; the chosen ids are pinned in `daily_results.question_ids` on first
fetch. One attempt per question, no retries, results grid at the end
(Wordle-style: green/red squares + time), calendar view of past days,
streak = consecutive days completed.

## API

All under `/api`, bearer-key auth (`Authorization: Bearer $MATHGAMES_API_KEY`)
exactly like the siblings; `GET /api/health` unauthenticated. JSON in/out,
snake_case.

```
GET    /api/health                → {"ok":true,"ai":<key configured>,"video":<R2 configured>,
                                     "messaging":<SMTP configured>}

POST   /api/sessions              {mode}                        → Session
POST   /api/sessions/{id}/end                                   → 204

GET    /api/next?skill=&count=&session_id=                      → {questions:[...], bank_low}
POST   /api/attempts              {session_id, question_id, given, elapsed_ms, day?}
                                                                → attempt result (above)
                                       (day is optional, device-local YYYY-MM-DD; when
                                       present, rolls the screen-time dial's daily
                                       reset over first if needed)

GET    /api/daily?day=YYYY-MM-DD  → {day, questions|results, answered, correct,
                                     completed, streak, calendar:[...last 30 days...]}
       (daily answers go through POST /api/attempts with the daily session)

GET    /api/profile               → {xp, xp_by_skill, levels, streaks,
                                     pokemon_unlocked, pokemon_total,
                                     gym_badges:[1,3,4], daily_streak}
GET    /api/collection            → full catalog + unlocked flags + how-to-unlock hints
POST   /api/catch                 {pokemon}                     → unlock result (409 unless 8 badges)

GET    /api/quests                → gym arcs → chapters with progress/completion/locks
GET    /api/quests/{id}           → chapter detail incl. story text

GET    /api/parents/summary?days=30
                                  → {per_day:[{day, attempts, correct_bp, minutes, xp}],
                                     per_skill:[{skill, level, attempts, correct_bp,
                                                 median_ms, trend}],
                                     recent_misses:[{prompt, given, answer, skill, day}],
                                     bank:[{skill, level, available}]}
POST   /api/generate              {kind, skill, difficulty, count}   → batch result (503 without key)
GET    /api/questions?skill=&source=ai&retired=false            → review list (parent)
POST   /api/questions/{id}/retire                               → 204
POST   /api/questions/{id}/unretire                             → 204

GET    /api/settings              → settings row
PUT    /api/settings              {daily_count, level_override, minutes_per_correct,
                                    clip_chance, clip_session_cap}
GET    /api/export                → full-DB JSON download

POST   /api/messages              {kind, body, context}         → saved Message (429 rate-limited)
GET    /api/messages              → [Message] newest first        (parent inbox)
GET    /api/messages/unread       → {count}                       (parent badge)
POST   /api/messages/{id}/read    → 204

GET    /api/clips                 → [Clip] metadata for the manage page
POST   /api/clips                 multipart {file, title, on_correct, on_wrong,
                                    weight, enabled, duration_ms}     → Clip (503 if R2 off)
PUT    /api/clips/{id}            {title, enabled, on_correct, on_wrong, weight} → Clip
DELETE /api/clips/{id}            → 204 (deletes the R2 object, then the row)
GET    /api/clips/plays?limit=    → recent clip_plays w/ clip title + trigger
```

The parent scorecard lives at the **hidden route `#/parents`** in the PWA —
no tab, no PIN; adults type the URL fragment. Same bearer key.

## Video clips (`internal/storage`, `internal/game/clips.go`)

Personal video clips recorded by Jim/family that Skyler unlocks inside the
app. v1 has exactly one trigger: **a random roll on any answer, correct or
wrong**, with a hidden `#/clips` manage route to upload clips and set the
conditions under which each plays. The per-clip condition model (below) is
built to carry more triggers later (streak milestones, daily completion,
quest rewards) without schema surgery — only the random-roll trigger is
wired up now.

**Storage.** Clip bytes live in **Cloudflare R2** (`internal/storage.R2Client`,
copied from `~/projects/food`'s `internal/storage/r2.go`, same `R2_*` env
var names so credentials are reusable across the sibling apps); the DB holds
metadata + the object key + public URL. Video features are **off until all
five `R2_*` vars are set** — `/api/health` reports `"video":false` and
`POST /api/clips` 503s, exactly like food's photo gating. Clips serve
**directly from their R2 public URL** (unguessable key) — anyone with the
URL can view it; a bearer-authed streaming proxy (range requests) is future
hardening if that ever matters, not built now.

**Schema (migration 005):**

```sql
CREATE TABLE clips (
    id, title, r2_key, url, content_type, size_bytes, duration_ms (nullable),
    enabled, on_correct, on_wrong, weight (>0), play_count, created_at
);
CREATE TABLE clip_plays (
    id, clip_id (FK), attempt_id (FK, nullable), trigger ('correct'|'wrong'), played_at
);
-- settings gains: clip_chance (1-in-N per answer, default 40),
--                 clip_session_cap (max clips per session, default 2)
```

`clip_plays` is the raw-data record (per the raw-attempt-data invariant) and
powers both immediate-repeat avoidance and the per-session cap count (joined
to `attempts.session_id` via `attempt_id` — no `session_id` column needed on
`clip_plays` itself). Both tables are included in `/api/export`.

**Trigger model.** Each clip carries `on_correct`/`on_wrong` flags, an
`enabled` flag, and a pick `weight`. `ClipRoll` (`internal/game/clips.go`,
pure/deterministic, rng injected like `RollEvent`) decides whether an answer
triggers a clip and which one:

1. `playsThisSession >= sessionCap` → nil.
2. Filter to `enabled` clips whose `on_correct`/`on_wrong` matches this
   answer's correctness; no matches → nil.
3. Roll `1 in clip_chance` (else nil).
4. Among the matching set, drop the last-played clip **if** more than one
   remains (avoid an immediate repeat — a lone eligible clip is allowed to
   repeat).
5. Weighted pick by `weight` (same walk idiom as `weightedPickSkill` /
   `RollEvent`).

This is a **separate roll from the XP event engine** (`internal/game/events.go`):
it runs on every answer, correct or wrong, independent of `RollEvent`
(which only fires on correct answers). `Service.Attempt()` calls it after
the attempt row is inserted (so `attempt.ID` exists), independent of the
XP-event branch; a pick inserts a `clip_plays` row, bumps `clips.play_count`,
and attaches a `ClipPlay{id, title, url, content_type}` to `AttemptResult`.

**Viewer.** When `result.clip` is present, the PWA pushes a
`{type:'clip', clip}` overlay **last** in the queue — after any XP-event
overlay, so on a correct answer the XP moment plays first and the clip is
the finale; on a wrong answer it's the sole overlay (gentle encouragement).
The overlay is a tap-to-play card ("🎬 A message from Uncle Jim! ▶"); tapping
swaps in a `<video controls playsinline>` and calls `.play()` synchronously
from the tap handler (iOS requires the gesture-to-play() call to have no
`await` in between, or sound is blocked). A ✕ dismisses and continues the
session — no auto-advance timer for this overlay, unlike the others.

**Manage route `#/clips`** — hidden, no tab, mirrors `#/parents` (adults
type the URL fragment, or follow the link from the Parents page). Upload
(file + title + trigger checkboxes + weight + enabled, with a local preview
and client-read `duration_ms`), clip list with inline condition toggles and
delete, the two settings knobs, and a recent-plays log.

## Messages (`internal/game/messages.go`, `internal/mailer`)

Skyler sends short notes (bug report 🐛 / idea 💡 / message 💬) from a
"📮 Message Uncle Jim" button on Home; the server emails them to Jim and
always saves them — this doubles as the feedback channel for co-designing
new games.

- **Save always, email best-effort.** Every message is inserted into
  `messages` before the send is attempted; a send failure records
  `email_error` on the row but never fails the request. The kid always sees
  "Sent!"; Jim sees every message in the hidden parents inbox even when
  email is down or unconfigured.
- **Delivery is Gmail SMTP with an app password** via stdlib `net/smtp`
  (STARTTLS on 587, `internal/mailer` — no SDK, no third-party service),
  gated on env exactly like AI and R2: off until `SMTP_USER` + `SMTP_PASS`
  are both set; health reports `messaging:false` and messages save unsent.
- **The recipient is server-controlled, never from the request.** "To" comes
  only from `MESSAGE_TO` (default = the sending account), so the endpoint
  can't be turned into a spam relay; From is always the authenticated
  account (Gmail requires it). The kid never sees or sets an address.
- **Rate limit: 10 messages per rolling hour** (`CountMessagesSince`);
  the 11th returns 429 and is *not* saved. Body ≤ 2000 chars, trimmed,
  non-empty; kind defaults to `message`.
- The email body is the message text plus a context footer
  (`{version, route, user_agent}` auto-attached by the PWA, stored in
  `messages.context`), so a bug report says which screen it came from.

**Schema (migration 008):**

```sql
CREATE TABLE messages (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT NOT NULL DEFAULT 'message'
                CHECK (kind IN ('bug','idea','message')),
    body        TEXT NOT NULL,
    context     JSONB,                 -- {version, route, user_agent} auto-attached
    emailed     BOOLEAN NOT NULL DEFAULT FALSE,
    email_error TEXT,                  -- last send error, if any
    read_at     TIMESTAMPTZ,           -- parent marked read
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

```
POST /api/messages            {kind, body, context}  → saved Message (429 if rate-limited)
GET  /api/messages            → [Message] newest first  (parent inbox)
GET  /api/messages/unread     → {count}                 (parent badge)
POST /api/messages/{id}/read  → 204
```

`messages` is included in `/api/export`. The **parents inbox** lives on
`#/parents` (summary + unread count) with the full list on the `#/inbox`
subpage: kind icon, body, context, email status (✉️ sent / ⚠️ saved-not-
emailed with the error), and mark-read. When health reports
`messaging:false` the inbox notes that email delivery is off but messages
still land there. No two-way replies, attachments, or user-settable
recipient in v1 (see TODO.md for planned extensions).

## AI content generation (`internal/ai`)

Copy the HTTP client from `~/projects/food/internal/ai/client.go` (hand-rolled,
no SDK; model from `AI_MODEL`, default `claude-sonnet-5`). Generation is a
**single non-agentic call per batch** (no tool loop needed): a system prompt
per kind (word_problems / logic / story) with the difficulty rubric, Skyler's
profile (8, gifted, voracious reader), the target skill/level, **the prompts
of the last 50 questions at that skill/level** (to avoid repeats), and strict
output instructions: a JSON array of `{payload, answer, explanation}` in the
exact shapes above.

Server-side validation before insert (deterministic, tested): JSON parses;
`kind` is known; answer shape matches kind; MC index in range; fraction
denominators > 0; prompt non-empty and ≤ 500 chars; **for any answer the
rubric can express numerically, sanity-check it** (e.g. word problems include
`"check": "34*12+50"` — a tiny integer expression evaluator in Go verifies
the expression equals the answer; reject the question if not). Rejected items
count toward `ai_batches.rejected`; accepted rows insert with `retired=false`.
Story batches rewrite `quest_chapters.title/story` for one saga in one call.

Batches run from the parent view (`POST /api/generate`) and from a seed
script (`scripts/seed-content.sh`, phase 4) that fills every AI skill×level
to ~40 questions plus all saga stories, so the bank starts full.

## PWA (`pwa/index.html`, single file — follow food/finance conventions)

`S` mutable state + `render()`; one reusable `<dialog>` via `openDialog(html)`;
hash routing (`#/home`, `#/play`, `#/collection`, `#/quests`, `#/parents`);
bottom tab bar (Home / Collection / Quests — Play is entered, not a tab;
Parents hidden). iPad-first: layouts must work in **both orientations**,
big touch targets (min 64px), on-screen **custom number pad** for numeric
answers (never the iOS keyboard — it covers half the screen and breaks the
flow). Service worker **network-first for the shell** with cache fallback,
network-only for `/api` (copy food's `sw.js`). Manifest: landscape-friendly,
`display: standalone`, Poké Ball red (`#EE1515`) theme color, original SVG
icon (a Poké Ball). API key in localStorage with a first-run prompt.

**Sound** (`sfx` module): **Web Audio API synthesis only — no audio files**
(keeps the single-file convention; base64 audio would bloat it). Oscillator +
gain-envelope recipes: correct = rising two-note chime; wrong = soft low
thud (never harsh); streak-milestone = rising energy sweep; level-up =
fanfare arpeggio; unlock = gong + shimmer; legendary/evolution moment = the
works. Master mute toggle persisted in localStorage. iOS unlocks audio on
first user gesture — resume the AudioContext in the first tap handler.

**Animation**: CSS transforms/keyframes + inline SVG. Streak ≥ 3 shows a
glowing aura around the streak counter that intensifies at 6 and 11; correct
answers burst a small energy particle effect; the XP number always animates
count-up on change; unlocks play a full-screen card-reveal (silhouette →
flash → Pokémon portrait). Respect `prefers-reduced-motion`.

Screens:

- **Home.** Trainer XP front and center (big animated number + aura),
  today's daily-challenge card (state: not started / in progress / results
  grid + streak), TRAIN button (starts a mixed training session), per-skill
  level chips (tap → train just that skill), recent unlocks strip.
- **Play (session).** One question at a time, huge type. Number pad /
  choice buttons / fraction inputs per `kind`; `display` hints render as SVG
  (fraction bars, sequences, grids). Immediate feedback: correct → green
  flash, XP flyup (+75 ⚡), sound, streak glow; wrong → gentle shake, the
  correct answer **with the explanation shown** (this is the teaching
  moment — make it readable, not skippable-in-100ms). Session header: streak
  glow, XP earned this session, end-session ✕. End screen: totals, best
  streak, any unlocks, "train again".
- **Daily.** Entered from the Home card. Same play UI, `daily_count`
  questions, one shot each, timer visible; end screen is the Wordle-style
  grid (🟩🟥 per question + total time) and the calendar streak view.
- **Collection.** Grid of Pokémon cards — unlocked in full color with
  rarity border and XP-at-unlock; locked as silhouettes with hint text
  ("Reach 25,000 XP"). Gym badge case (1–8, earned ones glowing); with all
  8 a USE MASTER BALL button → catch flow (pick any locked Pokémon, catch
  animation, badges scatter).
- **Quests.** Gym arc list (locked arcs greyed with unlock order); arc →
  chapter cards with progress bars; opening a chapter shows the story text,
  then TRAIN starts a quest session serving the chapter's skills at
  `min_difficulty`+; completion → reward reveal.
- **Parents (`#/parents`, hidden).** Per-day activity chart (attempts,
  accuracy, minutes — inline SVG, finance conventions), per-skill table
  (level, attempts, accuracy bp → shown as %, median time, 7-day trend
  arrow), recent misses list (prompt / his answer / right answer — gold for
  "what should we practice"), AI bank status per skill×level with a
  Generate button, question review list with retire toggles, settings
  (daily count, level overrides), export download link.

## Environment

Required: `DATABASE_URL`, `MATHGAMES_API_KEY`. Optional: `PORT` (default
**8083** — journal 8080, finance 8081, food 8082), `ANTHROPIC_API_KEY`
(without it: `/api/generate` → 503, health reports `ai:false`, everything
else works), `AI_MODEL` (default `claude-sonnet-5`), and the video-clips R2
vars — `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`,
`R2_BUCKET`, `R2_PUBLIC_URL` (off until **all five** are set: health reports
`video:false`, `POST /api/clips` → 503, everything else works).

Message email delivery (also optional, gated): `SMTP_HOST` (default
`smtp.gmail.com`), `SMTP_PORT` (default `587`), `SMTP_USER` (the sending
Gmail address), `SMTP_PASS` (a Google **App Password** — 16 chars, requires
2FA on the account — *not* the account password), `MESSAGE_TO` (default =
`SMTP_USER`). Off until `SMTP_USER` + `SMTP_PASS` are both set: health
reports `messaging:false` and messages save with `emailed=false`; everything
else works.

## Verification

- `go build ./... && go test ./...` at every phase gate.
- Unit tests: grading, XP (worked example above), adaptive ladder (worked
  example above), fraction equivalence, daily seeding determinism (same date
  → same set), generator output validity per skill×level (generate 200,
  assert answers verify and parameters stay in the level's bounds), unlock
  thresholds, AI-output validation (recorded fixtures).
- Smoke test against a scratch DB, never production:
  `createdb mathgames_smoke && DATABASE_URL=postgres://localhost:5432/mathgames_smoke?sslmode=disable MATHGAMES_API_KEY=x go run ./cmd/server`
  — `dropdb mathgames_smoke` when done.

## Deployment

Render web service from the `Dockerfile` (multi-stage `golang:1.25-alpine`
builder with `GOTOOLCHAIN=local` → `alpine` runtime — copy food's). Migrations
run at startup. Production Postgres on Render. The backup story is
`/api/export` (Render disks are ephemeral) — download it from the parent view
now and then.
