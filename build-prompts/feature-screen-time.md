# Feature — Screen-time dial: earn iPad time by answering questions

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first. This builds the
(previously parked) earned-screen-time feature in its agreed v1 shape:

> **Correct answers fill a big ki-gauge dial on Home, 3 minutes per correct,
> capped at 60. A parent resets it from the parents view when the time is
> spent.** The app never locks anything — it is the trusted meter; parents
> are the enforcers (iOS gives a PWA no way to control other apps, so
> redemption is handled by a human). One flat rate, no other ways to earn.

Design rules (settled, don't relitigate):

- **Corrects only** — wrong answers don't move the dial (attempt-count
  rules are gameable by mashing wrong answers). All modes count: training,
  quest, daily.
- **The dial is derived, never stored.** Earned minutes = (count of correct
  attempts since the last reset) × `minutes_per_correct`, capped at 60.
  Computed from `attempts` rows on demand — no per-question state, nothing
  to drift, immune to the daily double-counting bug noted in TODO.md.
- **Progress persists across days until a parent resets** — it's a bank,
  not a daily allowance.
- **Cap at 60 pauses earning** until reset; the pinned dial is the cue to
  go claim the hour.
- Rate is a settings knob: `minutes_per_correct`, default **3**
  (20 corrects = 1 hour), editable in the parents settings.
- Integer math throughout (minutes are integers; multiply before dividing
  if any derived display needs it).

## Server

1. **Migration `004_screen_time.sql`**:

   ```sql
   CREATE TABLE screen_time_resets (
       id               BIGSERIAL PRIMARY KEY,
       reset_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
       minutes_redeemed INT NOT NULL,   -- dial value at the moment of reset
       corrects_counted INT NOT NULL    -- corrects that produced it
   );
   ALTER TABLE settings ADD COLUMN minutes_per_correct INT NOT NULL DEFAULT 3;
   ```

2. **Store**: `CountCorrectsSince(ctx, since *time.Time) (int, error)`
   (all correct attempts when `since` is nil), `InsertScreenTimeReset`,
   `LastScreenTimeReset`, `ListScreenTimeResets` (newest first). Extend the
   fake store. Include `screen_time_resets` in `/api/export` and surface
   `minutes_per_correct` through the existing Settings type/endpoints.

3. **Service** (`internal/game/screentime.go`), deterministic + tested:

   ```go
   const screenTimeCapMinutes = 60

   type ScreenTime struct {
       MinutesEarned      int        `json:"minutes_earned"`       // capped
       MinutesCap         int        `json:"minutes_cap"`          // 60
       CorrectsSinceReset int        `json:"corrects_since_reset"`
       MinutesPerCorrect  int        `json:"minutes_per_correct"`
       Full               bool       `json:"full"`
       SinceReset         *time.Time `json:"since_reset"`          // nil = never reset
   }
   ```

   `ScreenTime()` computes the above; `ResetScreenTime()` snapshots the
   current values into a `screen_time_resets` row and returns it (reject
   with a validation error when `MinutesEarned == 0` — nothing to redeem).
   Worked examples in `screentime_test.go`, hand-checked in comments:
   7 corrects × 3 = **21 min**; 25 corrects × 3 = 75 → capped **60**,
   `Full: true`; after reset, 0. Also: rate change mid-period applies to
   the whole current period (it's derived — document that in a comment,
   it's intended); corrects landing while full don't bank past 60.

4. **API**:

   ```
   GET  /api/screentime        → ScreenTime
   POST /api/screentime/reset  → the created reset row (400 when dial is 0)
   GET  /api/screentime/log    → [{id, reset_at, minutes_redeemed,
                                   corrects_counted}], newest first
   ```

5. **AttemptResult** gains `screen_time_minutes int` — the post-attempt
   capped dial value — so the play screen can react without an extra
   fetch. (Compute it in `Attempt()` after the attempt row is inserted;
   one count query, only when the answer was correct — on wrong answers
   carry the value forward or omit; keep it cheap.)

## PWA

6. **Home: the dial.** A big inline-SVG **ki gauge** at the top of Home
   (scouter-style arc, not a car speedometer): 0→60 sweep with tick marks
   every 10, needle + filled arc in the energy palette, big minute readout
   in the center ("47 min"), "SCREEN TIME" label. Needle animates on
   change (CSS transition on the rotation). **Full state**: pinned at 60,
   gauge glows gold, label flips to "MAXIMUM POWER — 1 HOUR EARNED!", play
   `sfx.unlockFanfare()` once on the transition to full (not on every
   Home render — track the last-seen value in `S`). Respect
   `prefers-reduced-motion` (no glow pulse, needle jumps without sweep).
7. **Play: the tick.** When an attempt result's `screen_time_minutes`
   increased vs the previous value, show a small "+3 min ⏱" chip near the
   XP flyup (skip when full). Subtle — the XP/event feedback stays the
   star; this is a quiet reminder the dial is filling.
8. **Parents: reset + log.** In the parents view: a Screen Time block
   showing the current dial value and a **Reset** button — confirmation
   dialog via `openDialog` ("Reset the dial? 47 min will be marked
   redeemed."), then `POST /api/screentime/reset` and refresh. Next to it,
   a link to a **Screen Time Log** subpage — follow the existing
   question-review subpage pattern (`qreview` view registered in the
   render map, ~line 549): a table of resets, newest first — date+time,
   minutes redeemed, corrects counted — with the in-progress period as a
   highlighted top row ("current: 21 min, 7 corrects, since Jul 12").
   Also add the `minutes_per_correct` knob to the parents settings form.

## Docs & cleanup

9. ARCHITECTURE.md: replace the "**Parked (rev 2+…)**" paragraph with a
   short **"Screen time"** section: the deal (corrects × rate, cap 60,
   parent reset, derived-never-stored, app-as-meter / parents-as-enforcers),
   the table, the endpoints. Keep a one-line note that richer earning rules
   (bonuses, streak multipliers) are deliberately future work.
10. **Name fix: the nephew is "Skyler", not "Skylar".** Repo-wide
    case-preserving replace (Skylar→Skyler, skylar→skyler) — it appears in
    ARCHITECTURE.md, CLAUDE.md, README.md, DEPLOY.md, TODO.md,
    build-prompts/phase-4-ai-content.md, **`internal/ai/generate.go`** (the
    generation system prompts address him by name — this is the one that
    matters most, it flows into future word problems), and `pwa/index.html`.
    Note: **already-generated** `questions` rows in any live DB may contain
    the old spelling inside word-problem text; don't chase those with SQL —
    they'll wash out as batches regenerate, and a kid won't parse
    Skylar-vs-Skyler in a story problem anyway. New content must use
    Skyler.

## Out of scope

- No in-app locking, session limits, or enforcement of any kind.
- No varying earn rates (event bonuses, streak multipliers, daily bonuses)
  — one flat per-correct rate.
- No banking past the 60-minute cap, no multi-hour carryover.
- No editing or deleting log rows (the log is append-only history).

## Acceptance checklist

- `go build ./... && go test ./...` passes; worked examples (21, capped 60,
  0 after reset) hand-checked in comments.
- Migration 004 applies on a fresh scratch DB and on one that ran 001–003.
- Against a scratch DB: answer 3 questions correctly and 2 wrong →
  `GET /api/screentime` shows 9 minutes, 3 corrects; 22 total corrects →
  60, `full: true`; `POST /api/screentime/reset` → row with
  `minutes_redeemed: 60`, dial back to 0; reset at 0 → 400. Log lists the
  reset.
- In the PWA: the gauge renders on Home and animates as corrects land;
  the full state glows with the one-time fanfare; the "+3 min" chip
  appears during play; parents reset flow works with confirmation and the
  log subpage shows the history plus the current period row.
- `settings.minutes_per_correct` edit in parents view changes the dial
  immediately (derived math picks it up).
- `/api/export` includes `screen_time_resets`.
- `grep -ri skylar . --exclude-dir=.git` returns nothing.
