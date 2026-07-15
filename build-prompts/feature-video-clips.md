# Feature — Video clips: Jim-recorded messages Skyler unlocks on answers

Read `CLAUDE.md` and `ARCHITECTURE.md` in full first. This adds personal
video clips (recorded by Jim/family) that Skyler sees inside the app. v1 has
exactly one way to trigger a clip: **a random roll on any answer — correct or
wrong** — with a hidden manage route to upload clips and set the conditions
under which each plays. More triggers come later, so build the trigger model
to extend.

## Decisions (settled — don't relitigate)

- **Storage: Cloudflare R2.** Copy the client from
  `~/projects/food/internal/storage/r2.go` (same `R2_*` env var names — Jim's
  journal/food credentials are reusable) and the multipart-upload pattern
  from `~/projects/food/internal/api/photos.go`. Clip bytes live in R2; the
  DB holds metadata + the object key + public URL. Video features are **off
  until all five `R2_*` vars are set** (health reports `video:false`, upload
  returns 503) — exactly how food gates photos.
- **Playback: tap-to-play card.** When a clip triggers, the overlay is a
  full-screen "You earned a message from Uncle Jim! ▶" card; Skyler taps to
  play *with sound*. The tap is required — iOS blocks autoplay-with-sound
  after an async load, and Jim's voice is the point.
- **Frequency: rare treat.** Default ~1 in 40 answers, **per-session cap 2**,
  both tunable from the manage page. It must not turn a math session into a
  video-watching loop.
- **Per-clip trigger tags.** Each clip carries `on_correct` / `on_wrong`
  flags + an `enabled` flag + a pick `weight`. Random weighted pick among
  eligible clips, avoiding an immediate repeat.
- **This is a separate roll from the XP event engine** (`internal/game/
  events.go`). The XP events (Kaio-ken etc.) still fire on correct answers
  only; the clip roll is independent and runs on correct **and** wrong
  answers. Do not fold clips into `RollEvent`. (XP events on wrong answers
  remain future work — the Senzu-Bean TODO item — and are out of scope here.)
- **Clips serve directly from their R2 public URL** (unguessable key),
  matching how the sibling apps serve photos. Note in the code comment that
  this makes a clip viewable by anyone with the URL; a bearer-authed
  streaming proxy (range requests) is the future hardening if that ever
  matters. Don't build the proxy now.

## Where this hooks in (read before coding)

- `internal/game/service.go` — `Attempt()` (~line 203): the roll site for XP
  events (~line 279, correct-only) and the `AttemptResult` assembly
  (~line 350). The clip roll goes here too, but on every answer.
- `internal/game/types.go` — `AttemptResult`, `Settings`.
- `internal/api/handler.go` — `healthHandler` (~line 48), currently
  `{"ok":true,"ai":%t}`.
- `internal/api/` — food.go-style handlers; copy food's `photos.go` multipart
  upload shape.
- `pwa/index.html` — the view map + route dispatch (`currentRoute`
  ~line 538, `go()` ~line 540, load dispatch ~line 548, render map
  ~line 586), the `renderParents`/`qreview`/`stlog` hidden-subpage pattern
  to mirror, and the overlay queue (push ~line 715, drain/dispatch
  ~line 743).
- `pwa/sw.js` — the network-first fetch handler (~line 19).
- `go.mod` — add the `aws-sdk-go-v2` requires; copy the exact versions from
  `~/projects/food/go.mod` so the SDK matches the sibling apps.

## Server tasks

1. **Storage.** Copy `internal/storage/r2.go` from food; rename its methods
   generically (`Upload(ctx, key, contentType, r)`,
   `Delete(ctx, key)`) or keep the photo names — your call, but it stores and
   deletes video objects. Constructor reads the five `R2_*` env vars in
   `cmd/server/main.go`; a nil/disabled client when any is missing. Add the
   aws-sdk deps to go.mod (copy food's versions) and `go mod tidy`.

2. **Migration `005_video_clips.sql`:**

   ```sql
   CREATE TABLE clips (
       id           BIGSERIAL PRIMARY KEY,
       title        TEXT NOT NULL,
       r2_key       TEXT NOT NULL,
       url          TEXT NOT NULL,
       content_type TEXT NOT NULL,
       size_bytes   BIGINT NOT NULL,
       duration_ms  INT,                    -- client-reported, nullable
       enabled      BOOLEAN NOT NULL DEFAULT TRUE,
       on_correct   BOOLEAN NOT NULL DEFAULT TRUE,
       on_wrong     BOOLEAN NOT NULL DEFAULT FALSE,
       weight       INT NOT NULL DEFAULT 1 CHECK (weight > 0),
       play_count   INT NOT NULL DEFAULT 0,
       created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   CREATE TABLE clip_plays (
       id         BIGSERIAL PRIMARY KEY,
       clip_id    BIGINT NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
       attempt_id BIGINT REFERENCES attempts(id),
       trigger    TEXT NOT NULL CHECK (trigger IN ('correct','wrong')),
       played_at  TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   ALTER TABLE settings ADD COLUMN clip_chance      INT NOT NULL DEFAULT 40 CHECK (clip_chance > 0);
   ALTER TABLE settings ADD COLUMN clip_session_cap INT NOT NULL DEFAULT 2  CHECK (clip_session_cap >= 0);
   ```

   `clip_plays` is the raw-data record (per the invariant), and it powers
   both the immediate-repeat avoidance and the session-cap count. Include
   both tables in `/api/export`; surface the two settings through the
   existing Settings type/endpoints.

3. **Selection logic** (`internal/game/clips.go`, deterministic + tested,
   rng injected like `RollEvent`):

   ```go
   // ClipRoll decides whether an answer triggers a clip and which one.
   // Pure given its inputs. Returns nil when nothing should play.
   func ClipRoll(rng *rand.Rand, correct bool, eligible []Clip,
       lastPlayedID int64, playsThisSession, sessionCap, chance int) *Clip
   ```

   Rules: return nil if `playsThisSession >= sessionCap`; filter `eligible`
   to enabled clips whose `on_correct`/`on_wrong` matches this answer; return
   nil if none; roll `rng.Intn(chance) == 0` (else nil); among the eligible
   set, drop `lastPlayedID` **if** more than one remains (avoid immediate
   repeat); weighted pick by `weight` (same walk idiom as `weightedPickSkill`
   / `RollEvent`). Worked example hand-checked in `clips_test.go`:
   two eligible clips (weights 1 and 3), fixed seed, cap not hit → asserts
   the pick distribution and that a repeat is skipped when a third roll would
   re-pick the last one.

4. **Wire into `Attempt()`**: after the attempt row is inserted (so
   `attempt.ID` exists) and independent of the XP-event branch, load the
   eligible clips + last-played id + this session's clip-play count, call
   `s.clipRoll(newRand(), correct, ...)` (inject via an unexported
   `clipRoll` field on `Service` defaulting to `ClipRoll`, mirroring
   `rollEvent`). If a clip is chosen: insert a `clip_plays` row (clip_id,
   attempt_id, trigger), bump `clips.play_count`, and attach to the result.
   Keep the queries cheap (they only run per attempt; fine).

5. **AttemptResult** gains:

   ```go
   Clip *ClipPlay `json:"clip,omitempty"`

   type ClipPlay struct {
       ID          int64  `json:"id"`
       Title       string `json:"title"`
       URL         string `json:"url"`
       ContentType string `json:"content_type"`
   }
   ```

6. **Endpoints** (bearer-authed like everything else):

   ```
   GET    /api/clips              → [Clip] metadata for the manage page
   POST   /api/clips              → multipart {file, title, on_correct,
                                     on_wrong, weight, enabled, duration_ms}
                                     → Clip  (503 if R2 disabled)
   PUT    /api/clips/{id}         → {title, enabled, on_correct, on_wrong,
                                     weight}  (conditions only, not the file)
   DELETE /api/clips/{id}         → deletes the R2 object then the row
   GET    /api/clips/plays?limit= → recent clip_plays w/ clip title + trigger
   ```

   Upload: copy food's `photos.go` multipart handling (MaxBytesReader,
   `FormFile`, content-type sniff). Cap at **`maxClipBytes = 64 << 20`**
   (64 MB — a 30-second phone clip fits; note in an error message that Jim
   can trim/compress if a clip is rejected). Accept `video/mp4`,
   `video/quicktime`, `video/webm`; reject others. Key like
   `clips/<random>.<ext>`. On delete, remove the R2 object before the row so
   a failure doesn't orphan the row (log + continue if the object is already
   gone).

7. **Health**: extend to `{"ok":true,"ai":%t,"video":%t}` where `video` is
   whether the R2 client is configured.

## PWA tasks

8. **Viewer card** (the trigger outcome): when `result.clip` is present, push
   a `{type:'clip', clip}` overlay — **last** in the queue (after any XP-event
   overlay), so on a correct answer the XP moment plays first and the message
   is the finale; on a wrong answer it's the sole overlay (gentle
   encouragement). The card: dark backdrop, "🎬 A message from Uncle Jim!"
   heading, a big ▶ tap target. Tapping swaps in a `<video controls playsinline
   src=clip.url>` and calls `.play()` **from the tap handler** (required for
   iOS sound). A ✕ / "Back to training" dismisses and continues the session.
   Do not auto-advance the overlay queue on a timer for this one — let him
   finish watching and dismiss. `prefers-reduced-motion` only affects the
   card's entrance, not playback.

9. **Manage route `#/clips`** (hidden, no tab — mirror `#/parents`; register
   `clips` in `currentRoute`/load dispatch/render map). Sections:
   - **Upload**: file picker (accept `video/*`), title, trigger checkboxes
     (▢ on correct ▢ on wrong), weight, enabled. Before upload, read the
     file into a `<video>` to grab `duration_ms` (loadedmetadata) and show a
     local preview; POST multipart with a progress indication (uploads can
     be tens of MB). If health reports `video:false`, replace the form with
     "Video storage isn't configured (set the R2_* env vars on Render)."
   - **Clip list**: each clip — thumbnail-less row is fine (title, duration,
     size, play count) — with inline toggles for enabled / on_correct /
     on_wrong / weight (PUT on change), a ▶ preview (plays `url`), and a
     delete ✕ (confirm dialog).
   - **Settings**: `clip_chance` (as "1 in N") and `clip_session_cap`,
     saved through the settings endpoint.
   - **Recent plays log**: from `/api/clips/plays` — clip title, trigger
     (✓ correct / ✗ wrong), time.

10. **sw.js fix**: the fetch handler currently network-first-caches every
    same-origin GET and would try to cache cross-origin R2 video (opaque
    responses bloating the shell cache). Add an early return for
    cross-origin requests: `if (url.origin !== location.origin) return;`
    right beside the existing `/api` skip. Video then always goes straight
    to network, never the shell cache.

## Docs

11. ARCHITECTURE.md: a **"Video clips"** section — the R2 storage + gating,
    the `clips`/`clip_plays` schema, the trigger model (per-clip
    correct/wrong tags, weighted pick, immediate-repeat avoidance), the
    decoupled-from-XP-events roll with the 1-in-`clip_chance` +
    session-cap knobs, the tap-to-play viewer, and the endpoints. One line
    that more triggers (streak milestones, daily completion, quest rewards)
    are planned and the per-clip condition model is built to carry them.
    Add the `R2_*` vars to the Environment section (off until all set).

## Out of scope

- No triggers other than the random per-answer roll (streak/quest/daily
  triggers are future — leave the condition model roomy but don't build
  them).
- No XP events on wrong answers (separate future item).
- No transcoding, thumbnails, trimming, or bearer-authed streaming proxy.
- No editing of a clip's file (delete + re-upload); PUT changes conditions
  only.

## Acceptance checklist

- `go build ./... && go test ./...` passes; `ClipRoll` worked example +
  cap/eligibility/immediate-repeat cases present and hand-checked.
- `go mod tidy` clean; aws-sdk versions match food's go.mod.
- Migration 005 applies on a fresh scratch DB and on one that ran 001–004.
- Without the `R2_*` vars: server starts, `/api/health` shows
  `"video":false`, `POST /api/clips` → 503, everything else unaffected.
- With R2 configured, against a scratch DB: upload an mp4 via the manage
  page; it lands in R2 and lists; set it on_correct + on_wrong; temporarily
  set `clip_chance = 1` (revert after) and answer a question → the result
  carries `clip`, the tap-to-play card shows, tapping plays the video with
  sound (test on an actual iPad — this is the iOS-gesture-sensitive part);
  `clip_plays` has a row and `play_count` incremented.
- Session cap holds: with cap 2 and chance 1, a third answer in the same
  session does not attach a clip.
- Immediate-repeat avoided with two eligible clips over many rolls.
- Delete removes the R2 object and the row; `/api/export` includes `clips`
  and `clip_plays`.
- sw.js no longer caches cross-origin responses (check the cache after
  playing a clip — no R2 entry).
- `grep -i float internal/game/clips.go` → nothing.
