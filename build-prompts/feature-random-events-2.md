# Feature — Random events batch 2: Ultra Instinct, Bulma's capsule, Elder Kai

Read `CLAUDE.md` and `ARCHITECTURE.md` (→ "Random events") first. The event
engine already exists and ships one event (Kaio-ken ×2); this feature adds
three more and the two small engine extensions they need. Everything stays
deterministic, integer-only, unit-tested with injected randomness.

The three events:

> **Ultra Instinct flash** (`ultra_instinct`, ×3, silver) — only eligible
> when the correct answer came in **under the speed-bonus threshold**
> (`fastMS(difficulty)` in `internal/game/score.go`). Name:
> "ULTRA INSTINCT!" Message: "Your body moved on its own — TRIPLE points!"
>
> **Bulma's capsule** (`capsule`, flat +100 XP, Capsule-Corp teal/yellow) —
> no multiplier; a capsule pops open and drops loot. Name:
> "Bulma's Capsule!" Message: "A capsule pops open — +100 bonus points
> inside!"
>
> **Elder Kai's ritual** (`elder_kai`, ×2, mystic purple) — the inverse of
> Ultra Instinct: only eligible when the correct answer was **slow** —
> past the ok-bonus threshold (`elapsedMS > okMS(difficulty)`, score.go).
> Careful work counts too. Name: "Elder Kai's Ritual!" Message: "That took
> forever… but the power-up is REAL. Points DOUBLED!" (Canonically his
> unlock ritual takes 25 hours — the slowness is the joke.)

## Where the engine lives (read before coding)

- `internal/game/events.go` — `Event` struct, `events` registry,
  `RollEvent(rng, attemptsSinceLast)`, `Apply`, `MultiplierString`.
- `internal/game/service.go` — `Service.rollEvent` (test-injectable field,
  default `RollEvent`), the roll site inside `Attempt()` (~line 279: only on
  correct answers, `xpBeforeEvent` captured, `xp = ev.Apply(xp)`).
- `internal/game/types.go` — `EventResult` (~line 156): slug, name, message,
  `multiplier` (display string), `xp_before`.
- `internal/game/events_test.go` — existing seed-based tests to extend.
- `pwa/index.html` — event overlay CSS (~lines 221–227, currently
  kaioken-red generics: `.ov-event-flash`, `.ov-event-name`), overlay push
  (~line 715), queue-drain dispatch (~line 743, currently **hardcodes
  `sfx.kaioken()`** for every event), `sfx` module (~line 441).

## Engine extensions

1. **Eligibility predicate.** Add to `Event`:

   ```go
   // Eligible reports whether this event may fire for this attempt.
   // nil means always eligible.
   Eligible func(elapsedMS, difficulty int) bool
   ```

   `RollEvent` gains the attempt context:
   `RollEvent(rng *rand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event`.
   The 1-in-`eventChance` roll happens first, unchanged; then the weighted
   pick walks **only eligible events** (ineligible ones contribute no
   weight). If no event is eligible, nothing fires. Update the
   `Service.rollEvent` field signature and the call site in `Attempt()`
   (both values are in scope there).

2. **Flat XP bonus.** Add `XPFlat int` to `Event` and change `Apply` to
   `xp*XPNum/XPDen + XPFlat` (multiply before divide, then add). For
   display, `MultiplierString()` returns `"+100 ⚡"`-style when the event is
   flat-only (`XPNum == XPDen`), the existing `"×N"` otherwise —
   `EventResult` needs no shape change; the PWA already renders the
   `multiplier` string and the `xp_before → xp_earned` line generically.

3. **Registry.** Four entries with weights: `kaioken` **4**,
   `capsule` **3**, `elder_kai` **2**, `ultra_instinct` **2** (rarer, and
   further gated by its speed predicate — a triple should feel legendary).
   Ultra Instinct: `XPNum 3, XPDen 1`,
   `Eligible: elapsedMS <= fastMS(difficulty)`. Capsule:
   `XPNum 1, XPDen 1, XPFlat 100`, no predicate. Elder Kai:
   `XPNum 2, XPDen 1`, `Eligible: elapsedMS > okMS(difficulty)`. Kaio-ken
   unchanged apart from its weight. Note the predicates partition speed:
   a fast answer can roll ultra_instinct but never elder_kai, a slow one
   the reverse, a middling one (between fast and ok) neither — only
   kaioken/capsule.

## Server tests (extend `events_test.go` / service tests)

- Worked examples, hand-checked in comments: the ARCHITECTURE.md scoring
  example (75 XP) with ultra_instinct → **225**; with capsule → **175**
  (`75×1/1 + 100`). Elder Kai gets its own worked example since it can't
  co-occur with a speed bonus: difficulty 4, slow (`> okMS = 39000`),
  streak 7 → base 40, no speed bonus, ×125/100 streak = 50, elder_kai ×2 →
  **100**.
- Eligibility: at difficulty 4 (`fastMS = 13000`, `okMS = 39000`),
  `elapsedMS 13000` is eligible for ultra_instinct and not elder_kai;
  `39001` the reverse; `20000` (middling) eligible for neither. Assert an
  ineligible slug is never returned across 10k seeded rolls at each of the
  three speeds.
- Weight walk: over 10k seeded rolls of *fast* answers, kaioken, capsule,
  and ultra_instinct all occur, ordered by weight (kaioken > capsule >
  ultra_instinct in counts) and elder_kai never; over 10k *slow* rolls,
  elder_kai occurs and ultra_instinct never.
- Existing kaioken/cooldown/chance tests updated to the new signature and
  still passing; registry sanity test extended (XPFlat ≥ 0; flat-only
  events have positive XPFlat).
- Service-level (stubbed `rollEvent`): forcing capsule yields
  `xp_earned = xp_before + 100`, attempt row `event = 'capsule'`; forcing
  ultra_instinct yields tripled XP and slug on the row.

## PWA

4. **Per-slug palette.** Add a slug class to the overlay card
   (`ov-event-<slug>`, kaioken keeps current look as the default):
   - `ultra_instinct`: silver/white — cool white radial flash, pale
     silver-blue name glow, a slower, floatier pulse than kaioken's
     aggressive shake (Ultra Instinct is calm, not rage).
   - `capsule`: Capsule Corp teal/yellow — a capsule "pop" (small SVG
     capsule that scales up and splits) instead of the flash, gold XP line.
   - `elder_kai`: mystic purple — deep violet radial glow, slow ceremonial
     pulse (no shake at all; this one is solemn-then-funny, not frantic).
   - Reduced-motion handling identical to the existing card.
5. **Per-slug sound.** Replace the hardcoded `sfx.kaioken()` at the
   queue-drain dispatch with a slug switch: `ultra_instinct` →
   new `sfx.ultrainstinct()` (ethereal high shimmer — layered sine sweeps,
   quieter and airier than kaioken); `capsule` → new `sfx.capsule()` (a
   short pop + coin-like sparkle tones); `elder_kai` → new
   `sfx.elderkai()` (a low slow chant-like rising drone that resolves into
   a bright chord — ritual, then payoff); default → `sfx.kaioken()` so
   future events degrade gracefully.

## Docs

6. Update ARCHITECTURE.md → "Random events": the predicate and flat-bonus
   fields, the new registry table (slug, weight, effect, condition), and a
   one-line note that ineligible events are excluded from the weighted pick.

## Out of scope

- No events on wrong answers, no multi-attempt/stateful events (Vegeta's
  wager, Spirit Bomb, Time Chamber remain in TODO.md).
- No parent-view UI, no tuning knobs beyond the Go constants.
- No schema changes — `attempts.event` already stores any slug.

## Acceptance checklist

- `go build ./... && go test ./...` passes; both worked examples (225, 175)
  present and hand-checked in comments.
- Temporarily set `eventChance = 1` locally (revert before committing):
  fast correct answers eventually produce kaioken, capsule, and
  ultra_instinct overlays — each with its own palette and sound; slow
  correct answers produce elder_kai (and never ultra_instinct); middling
  answers produce neither of the speed-gated pair.
- Capsule overlay shows `+100 ⚡`-style bonus text and
  `xp_before → xp_earned` adds exactly 100; attempt rows in psql carry the
  right slugs.
- Kaio-ken still looks and sounds exactly as before (default class + default
  sound path).
- `grep -i float internal/game/events.go` → nothing.
