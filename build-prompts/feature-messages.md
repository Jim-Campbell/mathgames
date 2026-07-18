# Feature — Messages: Skyler sends bug reports & notes to Uncle Jim by email

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first. This lets Skyler send a
short message (bug report, idea, or just a note) from inside the app; the
server emails it to Jim **and** always saves it, so it doubles as Jim's
feedback channel for co-designing new games.

## Shape (settled)

- **Delivery: Gmail SMTP with an app password**, via Go's stdlib `net/smtp`
  (STARTTLS on 587) — **no new dependency, no third-party service**. Gated on
  env config exactly like the AI and R2 features: unset → the kid flow still
  works and messages are saved, email just doesn't send, and health reports
  `messaging:false`.
- **Save always, email best-effort.** Every message is inserted into a
  `messages` table before the send is attempted; a send failure records the
  error on the row but never fails the request. The kid always sees "Sent!";
  Jim sees every message in the hidden parents inbox even if email is down or
  unconfigured. (Matches the app's "rich raw data" invariant.)
- **Recipient is server-controlled, never from the request.** The "To"
  address comes only from env (`MESSAGE_TO`, default = the sending account) so
  the endpoint can't be turned into a spam relay. The kid never sees or sets
  an address.
- **Rate-limited** to avoid an 8-year-old flooding the inbox.

## Where this hooks in (read before coding)

- `cmd/server/main.go` — env/config block (~line 48; `AnthropicKey`, `R2*`
  pattern to mirror) and the `configured/not-configured` startup logging.
- `internal/api/handler.go` — `Config` struct (~line 15, has `AI`, `Video`
  bools) and `healthHandler` (~line 50, currently
  `{"ok":true,"ai":%t,"video":%t}`).
- `internal/api/game.go` — route registration + JSON handler style.
- `internal/game/` — `Service`, `Store` interface, types (add message ops).
- `pwa/index.html` — `renderHome` (~line 679) for the entry button, the one
  `openDialog` dialog for compose, `renderParents` (~line 1250) + the
  `qreview`/`stlog` subpage pattern for the inbox, the `api()` fetch helper
  (~line 433), and `S.build`/version if present for context.

## Server

1. **Config/env** (`main.go`): read
   `SMTP_HOST` (default `smtp.gmail.com`), `SMTP_PORT` (default `587`),
   `SMTP_USER` (Jim's Gmail address), `SMTP_PASS` (the 16-char app password),
   `MESSAGE_TO` (default `SMTP_USER`). Messaging is **enabled when `SMTP_USER`
   and `SMTP_PASS` are both set**; log configured/not-configured like the
   other integrations. Add all five to `.env.example` with a one-line note
   that `SMTP_PASS` is a Google **App Password** (needs 2FA on the account),
   not the Google account password.

2. **Mailer** (`internal/mailer/mailer.go`): a small type wrapping
   `net/smtp` — `smtp.PlainAuth` + `smtp.SendMail` (which negotiates STARTTLS
   on 587 automatically). Compose a plain RFC 5322 message: `From: SMTP_USER`
   (Gmail requires From = the authenticated user — do not make From differ),
   `To: MESSAGE_TO`, a `Subject` like `[MathGames] 🐛 Bug from Skyler`
   (kind-dependent), and a body = the message text + a context footer
   (app version, screen/route, time, user agent). A disabled mailer (missing
   creds) is a no-op that reports "not configured" so the service marks the
   row unsent. Define a tiny `Mailer` interface so the service takes it as a
   dependency and tests can inject a fake.

3. **Migration `008_messages.sql`:**

   ```sql
   CREATE TABLE messages (
       id          BIGSERIAL PRIMARY KEY,
       kind        TEXT NOT NULL DEFAULT 'message'
                   CHECK (kind IN ('bug','idea','message')),
       body        TEXT NOT NULL,
       context     JSONB,                 -- {version, route, user_agent} auto-attached
       emailed     BOOLEAN NOT NULL DEFAULT FALSE,
       email_error TEXT,                  -- last send error, if any
       read_at     TIMESTAMPTZ,           -- parent marked read
       created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   CREATE INDEX messages_created_idx ON messages (created_at DESC);
   ```

4. **Store**: `InsertMessage`, `ListMessages`, `MarkMessageRead`,
   `CountUnreadMessages`, and `CountMessagesSince(ctx, t)` for the rate limit.
   Add `messages` to the `/api/export` dump.

5. **Service** (`internal/game/messages.go`): `SendMessage(ctx, kind, body,
   context)`:
   - Validate: `kind` in the enum; `body` trimmed, non-empty, **≤ 2000
     chars** (reject 400 otherwise).
   - Rate limit: **max 10 messages per rolling hour** (`CountMessagesSince`);
     over → a typed error the handler maps to **429**.
   - Insert the row (emailed=false). Then, if the mailer is enabled, send;
     on success set `emailed=true`, on failure set `email_error` — **either
     way return the saved message with no request error**.

6. **Endpoints** (bearer-authed like everything):

   ```
   POST /api/messages            {kind, body, context}  → saved Message (429 if rate-limited)
   GET  /api/messages            → [Message] newest first  (parent inbox)
   GET  /api/messages/unread     → {count}                 (parent badge)
   POST /api/messages/{id}/read  → 204
   ```

7. **Health**: extend to `{"ok":true,"ai":%t,"video":%t,"messaging":%t}`
   (`messaging` = mailer configured). Thread a `Messaging bool` through
   `api.Config` and `healthHandler`.

## PWA

8. **Entry point** (kid-facing, on Home via `renderHome`): a friendly button —
   "📮 Message Uncle Jim". Tapping opens the compose dialog (`openDialog`):
   - Type chips: 🐛 Bug · 💡 Idea · 💬 Message (→ `kind`), default Message.
   - A large, friendly `<textarea>` (placeholder e.g. "Tell Uncle Jim what
     happened…"). No email address shown anywhere.
   - Send → `POST /api/messages` with `{kind, body, context}` where `context`
     is auto-filled `{version: <build>, route: <current hash>, user_agent}` —
     invisibly, so bug reports carry the screen he was on. Success → a
     cheerful confirmation ("Sent to Uncle Jim! 📨") then close. `429` → a
     gentle "You've sent lots of notes — try again in a little while!" Keep it
     kid-simple; never surface raw errors.
9. **Parents inbox** (`#/parents`): a Messages section (make it a subpage like
   `qreview`/`stlog` if it gets long) — newest first, each row: kind icon,
   body, the context (version · screen · time), email status (✉️ sent, or ⚠️
   "saved, not emailed" with the error), and a mark-read toggle; show an
   unread count. If health reports `messaging:false`, add a line: "Email
   delivery isn't configured — messages are still saved here."

## Docs

10. ARCHITECTURE.md: a **"Messages"** section — the save-always/email-best-
    effort model, server-controlled recipient, the 10/hour rate limit, the
    schema + endpoints, and the parent inbox. Add the five `SMTP_*` /
    `MESSAGE_TO` vars to the Environment section (gated: off until
    `SMTP_USER`+`SMTP_PASS` are set), with the app-password note.

## Out of scope

- No two-way replies (Jim → Skyler), no attachments/photos in messages, no
  moderation/filtering.
- No user-settable recipient — `MESSAGE_TO` is env-only.
- No new dependencies — `net/smtp` is stdlib.

## Acceptance checklist

- `go build ./... && go test ./...` passes. Service tests (fake mailer):
  success sets `emailed=true`; a mailer error keeps the row, records
  `email_error`, and **still returns the saved message with no request
  error**; the 11th message in an hour returns the rate-limit error; a
  disabled mailer saves with `emailed=false`.
- Migration 008 applies on a fresh scratch DB and on one that ran 001–007.
- Without `SMTP_*`: server starts, health `messaging:false`,
  `POST /api/messages` still 200 and saves; `GET /api/messages` shows it as
  not-emailed; nothing else affected.
- With a real Gmail app password (manual test): sending a message from the
  PWA delivers an email to `MESSAGE_TO` with the kind in the subject and the
  context footer in the body; the parents inbox shows it as ✉️ sent.
- Rate limit: the 11th send within an hour returns 429 and the PWA shows the
  gentle message; the message is not saved beyond the cap.
- The recipient cannot be influenced by the request body (grep the handler —
  `To` comes only from config).
- `/api/export` includes `messages`.
