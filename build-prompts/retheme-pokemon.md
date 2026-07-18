# Retheme — Dragon Ball Z → Pokémon

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first. Skyler isn't familiar
with Dragon Ball Z; he loves Pokémon. This reskins the entire app's theme —
characters, collectibles, named mechanics, art, sound, copy, and story
generation — from DBZ to Pokémon.

## The one rule: reskin, don't redesign

**Every mechanic, number, and invariant stays identical.** Same scoring math,
same XP formula, same adaptive ladder, same event weights/multipliers/
probabilities, same unlock thresholds, same rarity tiers, same screen-time
math, same API shapes (except the documented renames below), same
no-floats / answers-server-side / derived-screen-time / raw-attempts
invariants. You are changing **names, labels, slugs, art, sound flavor, and
story text** — nothing about how the game plays or computes. When in doubt,
keep the behavior and change only the words.

The single deliberate mechanic change: **7 Dragon Balls → 8 Gym Badges**
(Pokémon canon is 8 gym badges), which bumps the "collect them all" threshold
from 7 to 8. Everything else is pure reskin.

## Master mapping

| DBZ concept | Pokémon replacement |
|---|---|
| Theme "Dragon Ball Z" | "Pokémon" |
| "training" (answering) | keep "training" — Pokémon trainers train |
| Fighters collection (`kind='fighter'`, ~20) | Pokémon / Pokédex (`kind='pokemon'`) |
| 7 Dragon Balls (`kind='dragon_ball'`, ref 1–7) | **8 Gym Badges** (`kind='gym_badge'`, ref 1–8) |
| Streak achievements (`kind='badge'`, e.g. `streak-7`) | Ribbons (`kind='ribbon'`) — Pokémon's achievement awards |
| Collect 7 balls → summon Shenron → wish any fighter | Collect 8 badges → earn the **Master Ball** → catch any Pokémon |
| `/api/wish`, `Wish()`, `WishXP` | `/api/catch`, `Catch()`, `CatchXP` (same +1000 XP, same 8-vs-7 guard) |
| "power level" (100 + total XP) | "XP" / "Trainer XP" (same number, same math) |
| "IT'S OVER 9000!" one-time moment | a one-time legendary/evolution moment at the same threshold (no "over 9000" text) |
| `zenkai` comeback (correct after 3 misses, ×2) | "Comeback!" flavor (a Pokémon digging deep); JSON field `zenkai` → `comeback` |
| 5 sagas (saiyan, namek, android, cell, buu) | 5 Pokémon gym/region arcs (pick 5 Kanto-flavored arc keys) |
| Saga villains join (frieza, cell, majin-buu) | strong Pokémon awarded for beating each arc (e.g. onix, alakazam, lapras) |
| ki-gauge / "MAXIMUM POWER" (screen-time dial) | a **Poké Ball energy meter** / "FULLY CHARGED!" |
| Four-star Dragon Ball app icon, orange theme | **Poké Ball** icon, Poké Ball red (`#EE1515`) theme |

### Random events (`internal/game/events.go`) — keep weight/multiplier/predicate, reskin slug+name+message

| DBZ event | Pokémon event (mechanic unchanged) |
|---|---|
| `kaioken` ×2 | `lucky_egg` — "Lucky Egg! Double XP!" (Lucky Eggs double XP in Pokémon) |
| `capsule` flat +100 | `rare_candy` — "Rare Candy! +100 XP!" |
| `elder_kai` ×2 (slow answers) | `slowpoke` — "Slowpoke's Patience! ×2" |
| `ultra_instinct` ×3 (fast answers) | `critical_hit` — "Critical Hit! ×3" |

Note: `attempts.event` stores slugs; existing historical rows keep the old
slugs — that's fine, leave them (no data migration for history).

### Collectibles (`internal/game/fighters.go` → `pokedex.go`)

Replace the ~20 fighters with ~20 kid-friendly Pokémon, **preserving each
slot's rarity and XP threshold exactly** so balance is unchanged — only slug
and name change. Suggested (finalize as you like, keep thresholds):
pidgey, rattata, caterpie, eevee (commons/rares at the low thresholds) →
growlithe, machop, geodude, psyduck (rares) → charizard (the 9001 slot — the
signature moment), gyarados, gengar, snorlax, blastoise, venusaur (epics) →
dragonite, articuno, zapdos, moltres, mewtwo (legendaries at the top
thresholds). The wish/catch-only entry → **mew** (ultra-rare, only via the
Master Ball). Keep rarity tiers `common/rare/epic/legendary`.

## Identifier renames (mechanical — do them consistently)

- Go: `Fighter`→`Pokemon`, `fighters.go`→`pokedex.go`, `Fighters`→`Pokedex`;
  `UnlockFighter`→`UnlockPokemon`, `UnlockDragonBall`→`UnlockGymBadge`,
  the streak `UnlockBadge`→`UnlockRibbon`; `Wish`→`Catch`, `WishXP`→`CatchXP`;
  reward struct `Fighter`→`Pokemon`, `DragonBall`→`GymBadge` (with matching
  JSON tags `pokemon`, `gym_badge`); `zenkai` bool → `comeback` throughout
  (Score signature param, service var, AttemptResult JSON tag). Keep
  `power_level`'s *math* but rename the JSON/display to `xp` (and the helper
  `powerLevel()`→`totalXP()`); the unlock condition `Type:"power_level"` →
  `Type:"xp"`.
- Do **not** rename neutral DB tables/columns (`attempts`, `skill_state`,
  `questions`, etc.) — they aren't DBZ.

## Migration `006_pokemon_retheme.sql`

(If the daily-reset feature also lands, it takes 006 — use the next free
number.) Reskin the DB to match the new vocabulary:

```sql
-- unlock kinds
ALTER TABLE unlocks DROP CONSTRAINT <the kind check>;
DELETE FROM unlocks WHERE kind = 'fighter';          -- old fighter slugs are gone; Pokédex restarts
UPDATE unlocks SET kind = 'gym_badge' WHERE kind = 'dragon_ball';
UPDATE unlocks SET kind = 'ribbon'    WHERE kind = 'badge';
ALTER TABLE unlocks ADD CONSTRAINT unlocks_kind_check
    CHECK (kind IN ('pokemon','gym_badge','ribbon'));

-- quests: re-seed 5 arcs × 4 chapters with Pokémon saga keys, placeholder
-- titles/stories, and rewards using the new JSON keys (pokemon, gym_badge);
-- scatter 8 gym badges (was 7) across the chapters so all 8 are earnable.
DELETE FROM quest_chapters;
INSERT INTO quest_chapters (saga, chapter, title, story, requirement, reward) VALUES ...
```

Note this resets quest progress and clears the old fighter unlocks (the app
is barely used; acceptable). `fighters.go`'s saga-condition keys must match
the new arc keys. Bump the wish/catch ball-count guard 7 → 8.

## AI content (`internal/ai/generate.go`)

- **Story batches**: rewrite the system prompt so saga stories are a Pokémon
  gym-journey (catching, rivals, gym leaders, evolving) — no DBZ references.
  The chapter hook example changes from "Vegeta blocks the path — land 12
  multiplication hits!" to a Pokémon equivalent ("A wild Onix blocks Route 3
  — land 12 multiplication hits to battle past it!").
- Word-problem / logic prompts: if any DBZ flavor is present, switch to
  Pokémon; a light Pokémon flavor in word problems is a nice touch but keep
  the math rubric and the `check` field exactly.
- Update the phase-4 build prompt's story-hook example line too, for
  consistency (it's docs).

## PWA (`pwa/index.html`, `manifest.json`, `sw.js`)

This is the bulk of the work — ~37 DBZ references plus art and sound.

- **Art**: replace the ~20 fighter SVG portraits with ~20 **original,
  stylized Pokémon** SVGs via the existing parameterized component (body
  shape + palette + type-glow). Original stylized silhouettes evoking each
  Pokémon by color/shape — **do not trace or copy official artwork** (same
  rule the DBZ art followed). The Dragon-Ball tray → an 8-slot **gym badge
  case**; the app/collection iconography → Poké Balls.
- **Sound** (`sfx`): rename and reflavor the recipes to match the new events
  (`kaioken()`→`luckyEgg()`, `capsule()`→`rareCandy()`, `elderkai()`→
  `slowpoke()`, `ultrainstinct()`→`criticalHit()`), keep Web Audio synthesis
  and the per-slug dispatch. The over-9000 fanfare → the legendary/evolution
  moment sound. Keep the mute toggle.
- **Copy & labels**: every DBZ term (power level, ki, training auras, saga
  names, "over 9000", Shenron/wish, dragon balls) → its Pokémon replacement
  per the tables above. The catch flow replaces the wish flow (collect 8
  badges → Master Ball → pick any locked Pokémon → catch animation → badges
  consumed). The screen-time gauge becomes a Poké Ball energy meter.
- **manifest.json**: name/short_name Pokémon-flavored, `theme_color`
  `#EE1515`, the icon a Poké Ball SVG. **sw.js**: if the cache name embeds a
  theme string, bump it so the reskinned shell replaces the old cache.
- Keep every screen, route (incl. hidden `#/parents`, `#/clips`), and
  interaction exactly as-is — only the skin changes.

## Docs

- ARCHITECTURE.md and CLAUDE.md: replace the DBZ theme paragraph and all DBZ
  vocabulary with the Pokémon theme; update the collectibles/quests/events
  descriptions, the `unlocks.kind` values, the 8-badges/Master-Ball flow, the
  `/api/catch` rename, and the `xp`/`comeback` renames. The mechanics
  sections (scoring, adaptive, screen time) keep their numbers — only names
  change.

## Out of scope

- No gameplay, math, threshold, probability, or schema-semantics changes
  beyond 7→8 badges and the documented renames.
- No new features, screens, or content types.
- No migration of historical `attempts.event` slugs.

## Acceptance checklist

- `go build ./... && go test ./...` passes; tests updated for the renamed
  identifiers/slugs (event slugs, unlock kinds, catch, comeback) with the
  same asserted numbers (weights, multipliers, thresholds unchanged —
  e.g. the 75→150 event example still yields 150 under `lucky_egg`).
- Migration applies on a fresh scratch DB and on one that ran the prior
  migrations; unlock kinds, quest arcs, and the 8-badge scatter are correct.
- `grep -riE 'dragon ?ball|saiyan|goku|vegeta|piccolo|frieza|namek|\bcell\b|majin|buu|beerus|kaio|zenkai|shenron|scouter|over 9000|ki-gauge' internal pwa docs *.md`
  returns nothing (allow historical `attempts.event` data only).
- The collection shows Pokémon; earning all 8 badges enables the Master Ball
  catch; a random event fires as e.g. "Lucky Egg! Double XP!" with the same
  ×2 it had as Kaio-ken; the screen-time meter reads as a Poké Ball.
- A generated saga story reads as a Pokémon gym journey with the requirement
  hook, no DBZ references.
- App icon/manifest is a Poké Ball; the reskinned shell loads after a
  reload (sw cache bumped).
- `grep -i float internal/game/*.go` unaffected (still none in logic).
