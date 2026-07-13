# Build prompts — how to run these

Six phases, **strictly sequential** — each builds on the previous one's code.
Do **not** run them in parallel: they share `go.mod`, the migration files,
and (later) the single-file PWA, so parallel agents would collide constantly.

## Recommended dispatch: one fresh Claude Code session per phase

Sonnet-class models handle these fine because the design decisions are
already made — the prompts are execution work, not architecture work.

```
cd ~/projects/mathgames
claude --model sonnet
> Read CLAUDE.md, ARCHITECTURE.md, and build-prompts/phase-1-scaffold.md, then execute phase 1.
```

Then, **in a new session for each subsequent phase** (fresh context beats a
long polluted one):

```
> Read CLAUDE.md, ARCHITECTURE.md, and build-prompts/phase-N-*.md, then execute phase N.
```

## Gate between phases (do this yourself, don't skip)

1. `go build ./... && go test ./...` must pass.
2. Run the phase's **Acceptance checklist** (bottom of each prompt file).
3. `git add -A && git commit -m "Phase N: <name>"` — a clean commit per phase
   means a botched phase is one `git reset --hard` away from retry.
4. If a phase went sideways, reset and re-run it in a fresh session with a
   note about what went wrong appended to your kickoff message. Don't patch
   a confused session — restart it.

## Phase map

| Phase | File | Delivers |
|---|---|---|
| 1 | phase-1-scaffold.md | Go server skeleton, DB schema, auth, Dockerfile |
| 2 | phase-2-game-core.md | Generators, grading, XP, adaptive ladder, unlocks, daily seeding — pure Go + tests |
| 3 | phase-3-api.md | Sessions, serving, attempts, daily, profile, quests, parents, export |
| 4 | phase-4-ai-content.md | Anthropic client, batch generation + validation, saga stories, seed script |
| 5 | phase-5-pwa.md | The entire PWA: Home, Play, Daily, Collection, Quests, Parents |
| 6 | phase-6-polish-deploy.md | Sound/animation polish, seed content, README, Render deploy |

Nothing in phases 1–4 requires the PWA; test with `curl`. Phase 5 is the
biggest single prompt — budget a long session for it, and drive it
interactively: game feel ("that feedback flash is too fast", "the keypad is
cramped in portrait") benefits from your taste in the loop. Phase 2 is the
soul of the app — eyeball its tests before building on it.
