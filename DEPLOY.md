# Deploying Math Games to Render

Click-through instructions — nothing here is scripted, since cloud resources
shouldn't be created by an agent. Follow in order.

## 1. Database

Reuse the existing Postgres instance shared by food/finance/journal rather
than provisioning a new one — it's a single-user, low-traffic app, and
sharing the instance saves a second Postgres bill. Give it its own
**database** on that instance (not the same database another app uses) so
table names never collide.

1. Render dashboard → open the shared Postgres instance → **Connect** →
   copy the **PSQL Command** and run it locally.
2. At the `psql` prompt: `CREATE DATABASE mathgames;`
3. Take the instance's existing **Internal Database URL** and swap just the
   database name at the end (e.g. `.../food` → `.../mathgames`) — same
   host/port/user/password. That's `DATABASE_URL` for step 4 below.

(If you'd rather keep mathgames fully isolated — its own maintenance
window, no shared blast radius — **New → PostgreSQL** works the same as the
sibling apps' original setup; just skip straight to copying its Internal
Database URL.)

## 2. Generate the API key

```sh
openssl rand -hex 24
```

Save this value — it's `MATHGAMES_API_KEY` below, and the same value you
paste into the PWA's first-run prompt on Skylar's iPad.

## 3. Anthropic API key (optional, for AI content generation)

Only needed if you want `word_problems`/`logic` question generation and saga
story text. The app runs fully without it — those two skills just serve
from whatever's already in the bank (empty until generated), and `/api/generate`
returns 503.

Get a key from the [Anthropic Console](https://console.anthropic.com/) if
you don't already have one for the sibling apps.

## 4. Web service

1. Render dashboard → **New → Web Service** → connect this repo
   (`jimgcampbell/mathgames` or wherever it's hosted).
2. Environment: **Docker** (Render auto-detects the `Dockerfile`).
3. Region: same as the Postgres instance from step 1.
4. Plan: starter is fine to begin with.
5. Environment variables:

   | Key | Value |
   |---|---|
   | `DATABASE_URL` | the Internal Database URL from step 1 |
   | `MATHGAMES_API_KEY` | the value generated in step 2 |
   | `ANTHROPIC_API_KEY` | your Anthropic API key (optional — see step 3) |

   `AI_MODEL` and `PORT` are optional — leave unset for defaults
   (`claude-sonnet-5`, `8083`; Render sets its own `$PORT` and the app reads
   it, so `PORT` usually doesn't need setting on Render at all — confirm the
   service's assigned port matches what's exposed).

6. Deploy. **Migrations run automatically at startup** — first boot creates
   the whole schema from `internal/db/migrations/` (including the 5-saga,
   4-chapter quest seed), nothing to run by hand.
7. Once live, hit `https://<your-service>.onrender.com/api/health` and
   confirm `{"ok":true,"ai":true}` (`ai:false` is fine and expected if you
   skipped step 3 — everything except `/api/generate` still works).

## 5. Seed the AI question bank (optional, only if step 3 was done)

The two AI skills (`word_problems`, `logic`) start with an empty bank —
`GET /api/next?skill=word_problems` will report `bank_low: true` until it's
filled. Run the seed script once, pointed at the deployed service:

```sh
MATHGAMES_BASE_URL=https://<your-service>.onrender.com \
MATHGAMES_API_KEY=<the key from step 2> \
scripts/seed-content.sh
```

This fills both AI skills to ~40 questions per level (1–10) and rewrites the
story text for all 5 sagas. It's idempotent — safe to re-run later to top
up the bank as questions get retired from the parent view.

## 6. Install on iPad

1. Open the Render URL in **Safari** (must be Safari, not another browser,
   for PWA install on iOS/iPadOS).
2. Share button → **Add to Home Screen**.
3. Open the new Home Screen icon — first run prompts for the API key; paste
   the value from step 2.
4. Play a round to confirm everything's wired up end to end.

## Notes

- Render's disk is ephemeral — there's no persistent volume in this setup.
  The backup story is `GET /api/export`: download it periodically, or set up
  your own recurring job that hits the endpoint and stores the result
  somewhere durable.
- If `ANTHROPIC_API_KEY` is missing/wrong, the app still runs — content
  generation is just disabled (`/api/health` reports `ai:false`, and
  `word_problems`/`logic` training serves from whatever bank already exists,
  possibly falling back to nearby levels via `bank_low`). Nothing crashes on
  missing optional config.
- The parent scorecard is the hidden route `#/parents` on the deployed
  URL — no separate login, same bearer key (stored in the PWA's
  localStorage from step 6).
