# Math Games App — Architecture

Math + logic games app for **Skyler**, a gifted 8-year-old heading into 3rd
grade, targeting **4th-grade content** with adaptive difficulty. Primary
device: **iPad** (installed PWA). Single user. Sibling app to
`~/projects/food`, `~/projects/finance`, and `~/projects/journal` — same
stack, same patterns, same bearer-key auth. Read those repos' conventions
before inventing anything.

The theme is **Dragon Ball Z**: answering problems is "training", XP drives a
**power level**, milestones unlock a collection of fighters, story quests are
"sagas", and collecting seven dragon balls summons Shenron for a wish. This is
a private, non-commercial family app; all artwork is **original inline SVG**
(stylized, not copied) — names and lore references are used playfully.

Design goals, in order:

1. **Genuinely fun** — sounds, animation, collection pull, a daily
   Wordle-style ritual (he already solves Wordle).
2. **Adaptive** — he consumes content fast; difficulty must climb with him
   without a parent touching anything.
3. **Rich raw data** — every attempt with timings and wrong answers is kept
   forever so Jim + Claude can co-design new games and a parent scorecard can
   be honest.
4. **Extensible** — new skills, fighters, sagas, and game modes bolt on
   without schema surgery.

**Screen time.** Correct answers (any mode — training, quest, daily) fill a
ki-gauge dial on Home: `minutes_per_correct` (a settings knob, default 3)
per correct, capped at 60. The dial is **derived, never stored** — it's
`(count of correct attempts since the last reset) × minutes_per_correct`,
capped, computed from `attempts` rows on demand. A parent resets it from the
parents view once the time is spent, snapshotting a `screen_time_resets`
row. The app never locks anything; it's the trusted meter, and parents are
the enforcers (iOS gives a PWA no way to control other apps). One flat rate,
no bonuses or multipliers — richer earning rules are deliberately future
work.

| field | meaning |
|---|---|
| `minutes_earned` | current dial value, capped at 60 |
| `minutes_cap` | 60 |
| `corrects_since_reset` | corrects counted toward the current dial |
| `minutes_per_correct` | the rate (settings knob) |
| `full` | true once pinned at the cap |
| `since_reset` | timestamp of the last reset, or null if never reset |

```
GET  /api/screentime        → the dial (fields above)
POST /api/screentime/reset  → the created screen_time_resets row (400 if dial is 0)
GET  /api/screentime/log    → resets, newest first
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
- No other external services. No R2, no photos.

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

- **No floats in stored data or scoring math.** XP, power level, streaks,
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
    wrong_run      INT NOT NULL DEFAULT 0,        -- current consecutive-wrong run (zenkai)
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
    kind       TEXT NOT NULL CHECK (kind IN ('fighter','dragon_ball','badge')),
    ref        TEXT NOT NULL,            -- fighter slug, ball number '1'..'7', badge slug
    source     TEXT NOT NULL DEFAULT '', -- human-readable: 'power_level 9000', 'saga saiyan ch3', 'wish'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, ref)
);

CREATE TABLE quest_chapters (
    id           BIGSERIAL PRIMARY KEY,
    saga         TEXT NOT NULL,          -- 'saiyan','namek','android','cell','buu' (in order)
    chapter      INT NOT NULL,           -- 1..N within the saga
    title        TEXT NOT NULL,
    story        TEXT NOT NULL,          -- AI-generated narrative shown on open
    requirement  JSONB NOT NULL,         -- {"correct": 12, "skills": ["multiplication","fractions"], "min_difficulty": 3}
    reward       JSONB NOT NULL,         -- {"xp": 500, "fighter": "vegeta", "dragon_ball": 2}  (any subset)
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
- **Zenkai boost** (DBZ: Saiyans come back stronger from defeat): if this
  correct answer follows a run of ≥ 3 consecutive wrong answers in the same
  skill, double the final XP. (`wrong_run` in `skill_state` tracks the run.)
- **Wrong answer** = 1 XP (showing up counts), streak resets to 0,
  `wrong_run` increments.
- **Daily challenge**: all XP ×2; a perfect day (all correct) adds +100.

Worked example (hand-check in `score_test.go`): difficulty 4 multiplication,
answered correctly in 9.2 s with streak reaching 7, no zenkai. Base
`10×4 = 40`; `fast_ms = 13000` so 9200 ≤ fast → `40×150/100 = 60`; streak 7 →
`60×125/100 = 75`. **75 XP.** Same answer arriving after 3 straight misses:
`75×2 = 150`.

**Power level** = `100 + total lifetime XP` (sum across skills). It only goes
up. The PWA celebrates threshold crossings; **9000** gets the big one
("IT'S OVER 9000!" full-screen moment).

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
| `kaioken` | 4 | ×2 | none |
| `capsule` | 3 | +100 XP flat | none |
| `elder_kai` | 2 | ×2 | `elapsedMS > okMS(difficulty)` (slow) |
| `ultra_instinct` | 2 | ×3 | `elapsedMS <= fastMS(difficulty)` (fast) |

`RollEvent(rng, attemptsSinceLast, elapsedMS, difficulty)` fires with
probability **1 in 25** (`eventChance = 25`) and never fires within **10
attempts** (correct or wrong) of the last attempt that fired one
(`eventCooldown = 10`, tracked via `attempts.event` and
`Store.AttemptsSinceLastEvent`). Once it decides to fire, the weighted pick
walks only the events whose `Eligible` predicate (if any) allows the
attempt's `elapsedMS`/`difficulty` — ineligible events are excluded from
the weighted pick entirely, so a fast answer can never roll `elder_kai` and
a slow one can never roll `ultra_instinct`; if no event is eligible,
nothing fires. The multiplier/flat bonus applies **last** — after speed,
streak, zenkai, and the daily ×2 are already baked into `Score()`'s
result — so a daily kaio-ken answer stacks intentionally:
`(base×speed×streak×2 daily)×2 event`. The resulting XP then flows through
skill state, the attempt row, daily progress, and power level with no
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
  "xp_earned": 75, "zenkai": false,
  "streak": 7, "skill_level": 4, "level_changed": 0,
  "power_level": 9120, "power_level_before": 9045,
  "unlocks": [{"kind":"fighter","ref":"vegeta","name":"Vegeta","rarity":"epic"}]
}
```

## Collection, quests, daily

**Fighters** (`internal/game/fighters.go`, code-defined catalog — DB stores
only unlocks). ~20 fighters with slug, name, rarity
(`common/rare/epic/legendary`), an original inline-SVG portrait (drawn in the
PWA phase; stylized silhouettes + auras, not copied art), and an unlock
condition. Unlock sources:

- **Power-level thresholds** (Krillin 500, Yamcha 1,000, Tien 2,000, Piccolo
  4,000, Goku 9,001 — over 9000, Gohan 15,000, Vegeta 25,000, …, Beerus
  250,000). Exact table lives in the Go catalog.
- **Saga chapter rewards** (villains join when their saga is beaten: Frieza,
  Cell, Majin Buu…).
- **Streak badges** (daily-challenge calendar streaks: 3, 7, 14, 30 days).
- **Shenron wishes**: earning all 7 dragon balls lets him summon Shenron
  (`POST /api/wish` with a chosen locked fighter slug) — grants any fighter
  regardless of condition, +1000 XP, and consumes the balls (deletes the 7
  `dragon_ball` unlock rows; they can be earned again).

**Quests** — 5 sagas × 4 chapters, seeded in migration 002 with placeholder
titles/requirements; story text is AI-generated (`kind='story'` batches
rewrite `quest_chapters.story`). A chapter's `requirement` counts correct
answers in `quest` sessions at `min_difficulty`+ in the listed skills;
progress updates on every qualifying attempt; completion grants the reward
and unlocks the next chapter. Chapters must be done in order within a saga;
sagas unlock in order.

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
GET    /api/health                → {"ok":true,"ai":<key configured>}

POST   /api/sessions              {mode}                        → Session
POST   /api/sessions/{id}/end                                   → 204

GET    /api/next?skill=&count=&session_id=                      → {questions:[...], bank_low}
POST   /api/attempts              {session_id, question_id, given, elapsed_ms}
                                                                → attempt result (above)

GET    /api/daily?day=YYYY-MM-DD  → {day, questions|results, answered, correct,
                                     completed, streak, calendar:[...last 30 days...]}
       (daily answers go through POST /api/attempts with the daily session)

GET    /api/profile               → {power_level, xp_by_skill, levels, streaks,
                                     fighters_unlocked, fighters_total,
                                     dragon_balls:[1,3,4], daily_streak}
GET    /api/collection            → full catalog + unlocked flags + how-to-unlock hints
POST   /api/wish                  {fighter}                     → unlock result (409 unless 7 balls)

GET    /api/quests                → sagas → chapters with progress/completion/locks
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
PUT    /api/settings              {daily_count, level_override}
GET    /api/export                → full-DB JSON download
```

The parent scorecard lives at the **hidden route `#/parents`** in the PWA —
no tab, no PIN; adults type the URL fragment. Same bearer key.

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
`display: standalone`, DBZ-orange theme color, original SVG icon (four-star
dragon ball). API key in localStorage with a first-run prompt.

**Sound** (`sfx` module): **Web Audio API synthesis only — no audio files**
(keeps the single-file convention; base64 audio would bloat it). Oscillator +
gain-envelope recipes: correct = rising two-note chime; wrong = soft low
thud (never harsh); streak-milestone = ki-charge rising sweep; level-up =
fanfare arpeggio; unlock = gong + shimmer; over-9000 = the works. Master
mute toggle persisted in localStorage. iOS unlocks audio on first user
gesture — resume the AudioContext in the first tap handler.

**Animation**: CSS transforms/keyframes + inline SVG. Streak ≥ 3 shows a
flame aura around the streak counter that intensifies at 6 and 11; correct
answers burst a small energy particle effect; power-level number always
animates count-up on change; unlocks play a full-screen card-reveal
(silhouette → flash → fighter portrait). Respect `prefers-reduced-motion`.

Screens:

- **Home.** Power level front and center (big animated number + aura),
  today's daily-challenge card (state: not started / in progress / results
  grid + streak), TRAIN button (starts a mixed training session), per-skill
  level chips (tap → train just that skill), recent unlocks strip.
- **Play (session).** One question at a time, huge type. Number pad /
  choice buttons / fraction inputs per `kind`; `display` hints render as SVG
  (fraction bars, sequences, grids). Immediate feedback: correct → green
  flash, XP flyup (+75 ⚡), sound, streak flame; wrong → gentle shake, the
  correct answer **with the explanation shown** (this is the teaching
  moment — make it readable, not skippable-in-100ms). Session header: streak
  flame, XP earned this session, end-session ✕. End screen: totals, best
  streak, any unlocks, "train again".
- **Daily.** Entered from the Home card. Same play UI, `daily_count`
  questions, one shot each, timer visible; end screen is the Wordle-style
  grid (🟩🟥 per question + total time) and the calendar streak view.
- **Collection.** Grid of fighter cards — unlocked in full color with
  rarity border and power-level-at-unlock; locked as silhouettes with hint
  text ("Reach power level 25,000"). Dragon-ball tray (1–7, earned ones
  glowing); with all 7 a SUMMON SHENRON button → wish flow (pick any locked
  fighter, dragon animation, balls scatter).
- **Quests.** Saga list (locked sagas greyed with unlock order); saga →
  chapter cards with progress bars; opening a chapter shows the story text,
  then FIGHT starts a quest session serving the chapter's skills at
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
else works), `AI_MODEL` (default `claude-sonnet-5`).

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
