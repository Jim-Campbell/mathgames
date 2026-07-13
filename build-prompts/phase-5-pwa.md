# Phase 5 — PWA: the entire app UI

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first — especially the "PWA"
section, which specifies every screen. This is the biggest phase; budget a
long session. The backend is complete — this phase only consumes it.

## Reference implementations

- `~/projects/finance/pwa/index.html` — `S` state + `render()`, one reusable
  `<dialog>` via `openDialog(html)`, fetch helper with the bearer key,
  first-run key prompt, inline-SVG chart conventions
- `~/projects/food/pwa/sw.js` — network-first service worker with cache
  fallback (network-only for `/api`). Do NOT invent a cache-first scheme.
- `~/projects/food/pwa/manifest.json` — manifest shape

## Ground rules

- Single file `pwa/index.html` + `manifest.json` + `sw.js`. No frameworks,
  no chart libraries, no external assets, no audio files — all art is
  inline SVG, all sound is Web Audio synthesis.
- iPad-first: both orientations, touch targets ≥ 64px, custom on-screen
  number pad for numeric input (never trigger the iOS keyboard in play;
  `text`-kind questions may use a real input). Also usable on a phone.
- Hash routing: `#/home` (default), `#/play`, `#/collection`, `#/quests`,
  `#/parents` (hidden — no tab, reachable only by URL). Bottom tab bar:
  Home / Collection / Quests.
- DBZ look: dark space background, orange/gold energy palette, bold display
  type for numbers. `prefers-reduced-motion` disables the particle/aura
  effects. All fighter art is **original stylized SVG** (silhouette + aura +
  palette per character) — do not copy actual DBZ artwork.

## Build in this order

1. **Shell**: manifest, sw.js, key prompt, fetch helpers, routing, tab bar,
   theme/palette, `sfx` module (Web Audio recipes per ARCHITECTURE.md —
   correct chime, wrong thud, ki-charge, level-up fanfare, unlock gong,
   over-9000; AudioContext resumed on first gesture; mute toggle persisted).
2. **Home**: animated power-level counter with aura, daily card (three
   states), TRAIN button, per-skill level chips (tap → single-skill
   session), recent unlocks strip.
3. **Play**: the session loop per ARCHITECTURE.md → "Play (session)". Every
   payload kind renders: numeric (keypad), numeric2 (two labeled inputs),
   mc (big buttons), fraction (num/den keypad), text (input);
   `display.fraction_bar`, `display.sequence`, `display.grid` render as SVG.
   Feedback moments exactly as specced: XP flyup, streak flame at 3/6/11,
   explanations on wrong answers that can't be dismissed for 2s, level-up
   celebration, silent level-down, unlock reveal (full-screen card:
   silhouette → flash → portrait), the over-9000 moment (once, when power
   level first crosses 9000). End screen with totals + "train again".
4. **Daily**: from the Home card; one shot per question, visible timer,
   Wordle-style results grid (🟩🟥 + total time), 30-day calendar with
   streak count.
5. **Collection**: fighter grid (unlocked full-color with rarity border +
   power-level-at-unlock; locked as silhouettes with hint text), dragon-ball
   tray, SUMMON SHENRON → wish flow (pick a locked fighter, dragon
   animation, balls scatter). ~20 SVG portraits — build a small
   parameterized SVG component (hair shape, palette, aura) rather than 20
   bespoke drawings.
6. **Quests**: saga list with lock order, chapter cards with progress bars,
   story text screen → FIGHT → quest session, completion reward reveal.
7. **Parents** (`#/parents`): per-day activity chart (inline SVG, finance
   conventions), per-skill table (level, attempts, accuracy — render
   basis points as percentages, e.g. 8750 → 87.5%, median time, trend
   arrow), recent misses list, AI bank status + Generate buttons (with
   count feedback), question review with retire toggles, settings
   (daily count, level overrides per skill), export download link,
   mute default, build/version.

## Out of scope

No backend changes beyond trivial fixes (report anything bigger). No
screen-time features. No new dependencies.

## Acceptance checklist

- `go build ./... && go test ./...` still passes; server serves the PWA.
- On an iPad (or Safari responsive mode, both orientations): install to
  home screen works; first run prompts for the key once.
- A full training session plays end to end with sound and animation; every
  question kind and every `display` hint renders correctly (generate AI
  questions in phase 4's smoke DB or rely on template skills).
- Wrong answers show the explanation legibly; ten straight correct answers
  show the streak flame stepping up at 3 and 6.
- Daily: same set on re-open, one-shot enforcement, results grid, calendar.
- Collection reflects real unlocks; wish flow works when 7 balls exist
  (grant them via SQL to test).
- `#/parents` shows real aggregates and the export downloads.
- No `answer` field is ever visible in the network tab before an attempt.
- Reload after a server restart picks up new shell code (network-first sw
  verified).
