# Feature — Random events: rare fun surprises on correct answers

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first. This feature adds a
**random event engine**: every so often, a correct answer triggers something
fun. The engine is a registry so more event kinds can be added later with a
single struct literal; rev 1 ships exactly one event:

> **Kaio-ken ×2** — a big DBZ moment: "WOW — you just DOUBLED your points!"
> The answer's final XP is doubled. (Kaio-ken is canonically the
> power-multiplier technique, so play it straight: crimson aura, ×2.)

All the domain invariants apply: no floats, deterministic logic unit-tested
with injected randomness, raw data preserved, answers stay server-side, the
AI is not involved anywhere in this feature.

## Where this hooks in (read these before coding)

- `internal/game/score.go` — `Score(...)` stays **untouched**; the event
  multiplier is applied after it.
- `internal/game/service.go` — `Attempt(...)` (~line 203) is the single
  place XP is computed (`xp := Score(...)`), the attempt row is inserted,
  and `AttemptResult` is assembled. All server changes land here plus a new
  `events.go`.
- `internal/game/types.go` — `Attempt` and `AttemptResult` structs.
- `internal/game/service.go` `newRand()` — the existing crypto-seeded
  `math/rand` helper; reuse it.
- `pwa/index.html` — the overlay queue (`S.overlayQueue`, overlay types
  `levelup` / `unlock` / `over9000`, rendered via `#overlayRoot`, sounds
  dispatched where the queue is drained, ~line 728) and the `sfx` module
  (~line 395). The event overlay is a new queue type alongside these.

## Server tasks

1. **Engine** (`internal/game/events.go`):

   ```go
   type Event struct {
       Slug    string // "kaioken"
       Name    string // "Kaio-ken ×2!"
       Message string // "WOW — you just DOUBLED your points!"
       Weight  int    // relative pick weight among events (>0)
       XPNum   int    // multiplier numerator   (kaioken: 2)
       XPDen   int    // multiplier denominator (kaioken: 1)
   }
   ```

   A package-level `events = []Event{...}` registry with just kaioken, and:

   ```go
   // RollEvent decides whether a correct answer triggers an event.
   // Pure given its inputs: rng injected, cooldown passed in.
   func RollEvent(rng *rand.Rand, attemptsSinceLast int) *Event
   ```

   Rules: fires with probability **1 in 25** (`rng.Intn(eventChance) == 0`,
   `const eventChance = 25`); never fires when `attemptsSinceLast <
   eventCooldown` (`const eventCooldown = 10` — attempts, correct or not,
   since the last attempt whose `event` is non-null; if no event has ever
   fired, the cooldown is satisfied). When multiple events exist, pick by
   weight (`rng.Intn(totalWeight)` walk — same idiom as
   `weightedPickSkill`). Multiplier application is a method:
   `func (e *Event) Apply(xp int) int { return xp * e.XPNum / e.XPDen }` —
   integer, multiply before divide.

2. **Store**: add `AttemptsSinceLastEvent(ctx) (int, error)` to the Store
   interface + pgx implementation (count attempts newer than the max-id
   attempt with `event IS NOT NULL`; total attempt count when none) + the
   fake store used by tests. Extend `InsertAttempt` to persist the new
   field.

3. **Migration** `internal/db/migrations/003_attempt_events.sql`:

   ```sql
   ALTER TABLE attempts ADD COLUMN event TEXT;
   ```

   Nullable, no default; the slug of the event that fired on this attempt.

4. **Wire into `Attempt()`** (service.go): after `xp := Score(...)` and
   only when `correct`, query the cooldown count, call
   `RollEvent(newRand(), n)`; if an event fires, `xp = ev.Apply(xp)` —
   **after** zenkai and daily are already baked into `Score`'s result, so a
   daily kaio-ken answer is (base×speed×streak×2 daily)×2 event; that
   stacking is intentional. The doubled `xp` then flows everywhere the old
   one did (skill state, attempt row, daily progress, power level) with no
   further special-casing. Set `attempt.Event = ev.Slug` (new `Event string`
   field on the struct, `json:"event,omitempty"`).

5. **API shape**: `AttemptResult` gains

   ```go
   Event *EventResult `json:"event,omitempty"`

   type EventResult struct {
       Slug       string `json:"slug"`
       Name       string `json:"name"`
       Message    string `json:"message"`
       Multiplier string `json:"multiplier"` // "×2" — display string
       XPBefore   int    `json:"xp_before"`  // xp before the event multiplier
   }
   ```

   `xp_earned` stays the final (post-event) value. No handler changes
   beyond the type flowing through.

6. **Tests** (extend `internal/game/` tests, fixed seeds throughout):
   - Worked example, hand-checked in a comment: the ARCHITECTURE.md scoring
     example (difficulty 4, fast, streak 7 → 75 XP) with kaioken →
     **150 XP**; `xp_before` 75.
   - `RollEvent` with a fixed seed over 10,000 rolls at
     `attemptsSinceLast=100`: fire count lands in a sane band around 400
     (assert e.g. 300–500 — deterministic for the seed, the band just
     documents intent).
   - Cooldown: `attemptsSinceLast` 0..9 never fires regardless of seed;
     10 can.
   - Registry sanity: unique slugs, `Weight > 0`, `XPDen > 0`, applying
     each event to 100 XP yields a positive integer.
   - Service-level: stub the roll (inject via an unexported
     `rollEvent func(...)` field on `Service`, defaulting to `RollEvent`)
     to force an event → assert attempt row has the slug, result carries
     `event`, xp doubled, power level reflects the doubled value; and a
     no-event attempt leaves `event` empty/omitted.

## PWA tasks

7. **Overlay**: when `result.event` is present, push
   `{type:'event', event: result.event}` **first** in the overlay list
   (before levelup/unlock — the surprise is the headline). Full-screen
   card in the existing `#overlayRoot` system:
   - Crimson/red palette (this is Kaio-ken — reuse the over-9000 red,
     `var(--red)`), pulsing aura burst behind big shaking display text:
     the event `name` ("KAIO-KEN ×2!"), then `message`, then
     `+{xp_before} ⚡ → +{xp_earned} ⚡` so the doubling is visible.
   - Render generically from `EventResult` fields (name/message/multiplier)
     so a future event needs zero PWA changes unless it wants custom art;
     allow a per-slug CSS class (`ov-event-kaioken`) for palette overrides.
   - Tap to dismiss, auto-dismiss ~2.5s, `prefers-reduced-motion` gets a
     static card (no shake/pulse), same dismissal.
   - The regular XP flyup shows the final `xp_earned` (it already does —
     verify nothing recomputes XP client-side from thresholds).
8. **Sound**: new `sfx.kaioken()` — an aggressive rising crimson-feeling
   recipe (e.g. sawtooth sweep up + a hard two-tone hit, louder than
   `streak()`, shorter than `over9000()`), dispatched when the event
   overlay shows, following the existing pattern at the queue-drain switch.
   Respect the existing mute toggle (all `sfx` recipes already do).

## Docs

9. Add a short **"Random events"** subsection to ARCHITECTURE.md under
   "Scoring" documenting the registry, the 1-in-25 chance, the 10-attempt
   cooldown, the multiplier-applied-last rule (and that daily stacking is
   intentional), and the `attempts.event` column. One paragraph + the
   constants.

## Out of scope

- No parent-view UI for events (tuning knobs stay Go constants for now).
- No non-XP effect kinds (item drops, fighter encounters) — the registry
  shape leaves room; don't build the machinery.
- No changes to `Score()`, the adaptive ladder, streaks, or unlock logic.
- No new dependencies.

## Acceptance checklist

- `go build ./... && go test ./...` passes; the worked example (75 → 150)
  is present and hand-checked in a comment.
- Migration 003 applies on a fresh scratch DB **and** on one that already
  ran 001–002.
- Against a scratch DB with the roll stubbed/forced (or by temporarily
  setting `eventChance = 1` locally — revert before committing):
  `POST /api/attempts` on a correct answer returns `event` with
  slug/name/message/multiplier/xp_before, `xp_earned` doubled, and
  `psql`: the attempt row's `event = 'kaioken'`.
- Two forced events within 10 attempts: the second does **not** fire
  (cooldown holds).
- In the PWA (eventChance=1 trick): correct answer → Kaio-ken takeover
  renders with sound before any levelup/unlock overlay, dismisses by tap
  and by timeout, and the session continues normally. Reduced-motion mode
  shows the static card.
- Wrong answers can never fire an event (assert in a test, verify once by
  hand).
- `grep -i float internal/game/events.go` → nothing.
