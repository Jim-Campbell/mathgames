# Feature — Screen-time dial: auto-reset to 0 on first use each day

Read `CLAUDE.md` and `ARCHITECTURE.md` (→ "Screen time") first. The
screen-time dial already exists (`internal/game/screentime.go`, migration
004, `GET/POST /api/screentime*`). Today it only zeroes on a **manual parent
reset**. This adds a second reset: **the dial auto-resets to 0 on the first
use of the app each local day.** The manual parent reset stays exactly as is.

Keep the "derived, never stored" model intact — resets have always been rows;
a daily rollover is just an automatically-inserted reset row.

## How it works

The dial counts correct attempts since the most recent reset row. A daily
rollover inserts a new reset row (reason `daily`) at the first interaction of
a new local day, snapshotting the value that was on the dial, so the dial
then reads 0 for the new day.

- **Local day** comes from the device (Render is UTC; don't infer the day
  server-side). The PWA already computes a device-local `YYYY-MM-DD` for the
  practice calendar — reuse it. Endpoints that can roll the day take it as a
  param; store it on the reset row so rollover detection needs no timezone
  math.
- **Rollover trigger**: on `GET /api/screentime?day=<localDay>` (and,
  defensively, on `POST /api/attempts` when the PWA includes `day`), if the
  most recent reset row's `day` is null or **earlier than** `localDay`,
  perform a daily auto-reset before computing the dial.
- **Idempotent / race-safe**: at most one `daily` row per calendar day,
  enforced by a partial unique index + `ON CONFLICT DO NOTHING`. Repeated
  calls the same day insert nothing.

## Server

1. **Migration `006_screen_time_daily.sql`:**

   ```sql
   ALTER TABLE screen_time_resets
       ADD COLUMN reason TEXT NOT NULL DEFAULT 'manual'
           CHECK (reason IN ('manual','daily')),
       ADD COLUMN day DATE;
   CREATE UNIQUE INDEX screen_time_daily_uniq
       ON screen_time_resets (day) WHERE reason = 'daily';
   ```

   Existing rows are historical manual resets → `reason` defaults to
   `manual`, `day` stays null (fine; they're older than any future day).

2. **Types/store**: add `Reason string` and `Day *time.Time` (or a
   `civil`-style date string — match how the practice calendar carries a
   date) to `ScreenTimeReset`. Add a store method that does the conditional
   insert atomically:

   ```go
   // InsertDailyResetIfNew inserts a reason='daily' row for localDay unless
   // one already exists (partial unique index + ON CONFLICT DO NOTHING).
   // Returns whether a row was inserted.
   InsertDailyResetIfNew(ctx, localDay string, resetAt time.Time,
       minutes, corrects int) (bool, error)
   ```

   `InsertScreenTimeReset` (manual) sets `reason='manual'` and `day=localDay`.

3. **Service** (`screentime.go`):
   - New `EnsureDailyReset(ctx, localDay string) error`: read
     `LastScreenTimeReset`; if it's nil or its `day < localDay`, compute the
     current dial value (the existing corrects-since × rate, capped) and call
     `InsertDailyResetIfNew(localDay, now, value, corrects)`. No-op when the
     latest reset is already for `localDay`.
   - `ScreenTime(ctx, localDay string)`: call `EnsureDailyReset` first, then
     compute as it does today (since = latest reset's `reset_at`). Now that a
     daily row exists at first use, the dial reads 0 at the start of each day.
   - `ResetScreenTime(ctx, localDay string)`: unchanged behavior, but record
     `reason='manual'`, `day=localDay`.
   - Signatures gain `localDay`; update the two API handlers
     (`screenTime`, `resetScreenTime` in `internal/api/game.go`) to read
     `day` from the query string / JSON body. Reject a missing/malformed
     `day` with 400 (the PWA always sends it).

4. **Attempt path**: `POST /api/attempts` gains an optional `day` field; when
   present, call `EnsureDailyReset(day)` before computing the attempt's
   `screen_time_minutes` (so the first answer of a day rolls over even if the
   Home screen didn't fetch `/api/screentime` first). When absent, skip — the
   next screentime GET catches it.

5. **Tests** (`screentime_test.go`), fake store, hand-checked:
   - Monday: 10 corrects at rate 3 → dial 30. Call `ScreenTime(day=Tuesday)`
     → a `daily` reset row is inserted snapshotting 30, and the returned dial
     is **0**. The log then shows the daily row (minutes 30) plus any manual
     rows.
   - Idempotent: a second `ScreenTime(day=Tuesday)` inserts no row and still
     reads 0 (until new corrects land).
   - Same-day corrects after rollover accrue normally (3 corrects → 9).
   - Manual reset still works and records `reason='manual'`.

## PWA

6. Pass the device-local `day` (the practice-calendar helper) on the
   screentime fetch (`GET /api/screentime?day=…`) and on attempt submissions.
   No visible change to the gauge itself — it will simply read 0 at the first
   open each day. In the parents Screen Time Log, label rows by reason:
   `daily` rows as "Daily reset" (rolled over / expired, not redeemed) and
   `manual` rows as "Redeemed by parent", so the history reads honestly.

## Docs

7. ARCHITECTURE.md → "Screen time": add that the dial also auto-resets to 0 on
   the first use each local day (a `reason='daily'` reset row), alongside the
   manual parent reset; note the `day`/`reason` columns and that the manual
   reset is unchanged.

## Out of scope

- No change to the earning rate, cap, or manual-reset behavior.
- No server-side timezone inference — the device supplies the local day.
- No new UI beyond the log's reason labels and passing `day`.

## Acceptance checklist

- `go build ./... && go test ./...` passes; the Monday→Tuesday worked example
  is present and hand-checked.
- Migration 006 applies on a fresh scratch DB and on one that ran 001–005.
- Against a scratch DB: earn some minutes; `GET /api/screentime?day=<today>`
  shows them; `GET /api/screentime?day=<tomorrow>` returns 0 and inserts one
  `daily` row (visible in `/api/screentime/log`); calling it again with
  `<tomorrow>` inserts no second row.
- Manual `POST /api/screentime/reset` still works and logs a `manual` row.
- `/api/export` includes the new columns.
