# Phase 2 — Game core: generators, grading, scoring, adaptive ladder, unlocks

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first. This phase is **pure
domain logic in `internal/game/` with heavy unit tests** — no HTTP handlers,
no AI. It is the soul of the app; everything later trusts it.

All specs referenced below are in ARCHITECTURE.md — implement them exactly:
"Skills and difficulty" (level→parameter tables), "Question payloads",
"Scoring", "Adaptive difficulty", "Collection, quests, daily".

## Tasks

1. **Types** (`types.go`): Question, Payload, Answer, Attempt, AttemptResult,
   SkillState, Session, Fighter, QuestChapter, DailyState, Settings — JSON
   tags snake_case, matching the schema and API shapes in ARCHITECTURE.md.
2. **Store interface** (`store.go`) + pgx implementation in `internal/db/`:
   the operations the service needs (insert/pick questions, record attempts,
   read/update skill_state, unlocks, quest progress, daily_results, settings,
   export dump). Keep it one interface so tests can use a fake.
3. **Generators** (`gen_*.go`, one file per template skill): for each of
   `multiplication`, `division`, `addsub`, `fractions`, `place_value`,
   `patterns`, a `Generate(level int, rng *rand.Rand) (payload, answer,
   explanation)` implementing the level table exactly. Explanations show the
   working (e.g. long multiplication split into partial products), one short
   line. Fraction visual levels emit `display.fraction_bar`; patterns emit
   `display.sequence` with the blank as null.
4. **Grading** (`grade.go`): pure function per ARCHITECTURE.md → "Question
   payloads" — numeric exact, numeric2 both values, mc by index, fraction by
   integer cross-multiplication, text lowercase/trim against value+accept.
5. **Scoring** (`score.go`): base/speed/streak/zenkai exactly per
   ARCHITECTURE.md → "Scoring", integer math, multiply-before-divide. Daily
   ×2 and perfect-day +100 included.
6. **Adaptive ladder** (`adapt.go`): the 10-attempt window rule per
   ARCHITECTURE.md → "Adaptive difficulty", including override behavior
   (pin serves the override level; window/level still update underneath).
7. **Fighters catalog** (`fighters.go`): ~20 fighters with slug, name,
   rarity, unlock condition (power-level thresholds per the examples in
   ARCHITECTURE.md — fill in a sensible full table; saga rewards; streak
   badges; wish-only fighters like Shenron himself as legendary). Unlock
   detection: given old/new power level, streaks, and quest completions,
   return newly earned unlocks.
8. **Daily seeding** (`daily.go`): FNV-1a(dateString) → `math/rand` source →
   pick 4 rotating template skills + 1 AI skill per ARCHITECTURE.md →
   "Collection, quests, daily". Deterministic for a given date + levels.
9. **Service** (`service.go`): orchestrates an attempt end-to-end — grade,
   score, update skill_state (streak, wrong_run, window, level, xp), update
   quest progress and daily state when the session mode applies, detect
   unlocks, persist, return AttemptResult. Serving: template generation +
   insert, AI-bank pick per ARCHITECTURE.md → "Question serving" including
   the mixed-mode weighting and `bank_low`.

## Tests (the point of this phase)

- `score_test.go`: the worked example from ARCHITECTURE.md **hand-checked in
  a comment** (40 → 60 → 75; zenkai → 150), plus boundary cases (streak 2 vs
  3, 5 vs 6, 10 vs 11; fast/ok/slow thresholds; wrong = 1 XP).
- `adapt_test.go`: the worked example (9/10 → promote; 3/10 → demote), stay
  case (5–7 correct), cap at 10, floor at 1, window reset on change,
  override pinning.
- `grade_test.go`: every kind; fraction equivalents (6/8 ≡ 3/4 accepted,
  2/5 ≢ 3/4 rejected); text case/space insensitivity.
- `gen_test.go`: for each template skill × each level 1–10, generate 200
  questions with a fixed seed and assert (a) the stored answer grades
  correct against itself, (b) operands/parameters stay inside the level's
  bounds, (c) payload kind matches the spec.
- `daily_test.go`: same date+levels → identical set; different dates differ.
- `fighters_test.go`: threshold crossings unlock exactly once; wish
  consumes 7 balls.
- Use a fake in-memory Store for service tests (see
  `~/projects/food/internal/food/fake_store_test.go` for the pattern).

## Out of scope

No HTTP handlers (phase 3), no Anthropic client (phase 4), no PWA changes.

## Acceptance checklist

- `go build ./... && go test ./...` passes with all the tests above present
  and meaningful (spot-check: temporarily break the streak multiplier and
  confirm tests fail).
- `go test ./internal/game/ -run TestScore -v` shows the worked example.
- Generators produce no floats anywhere (grep the package for `float`).
- The server still starts against a scratch DB as in phase 1.
