package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"time"
)

// BonusSkillSlug is a pseudo-skill row in skill_state that holds XP not
// earned by practicing a specific skill (quest/saga rewards, catch grants).
// It's never in the Skills registry, so it's excluded from per-skill views
// (profile, home chips) automatically while still counting toward
// totalXP(), which sums every skill_state row.
const BonusSkillSlug = "_bonus"

// CatchXP is the flat XP bonus a granted catch adds (ARCHITECTURE.md
// "Collection, quests, daily").
const CatchXP = 1000

// Service orchestrates attempts and question serving on top of a Store.
type Service struct {
	store Store
	log   *slog.Logger

	// mailer delivers kid messages by email, best-effort (see SendMessage).
	// nil or disabled means messages are saved but never emailed.
	mailer Mailer

	// rollEvent is RollEvent by default; overridable in tests to force/deny
	// an event without depending on the RNG.
	rollEvent func(rng *mrand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event

	// clipRoll is ClipRoll by default; overridable in tests to force/deny a
	// clip without depending on the RNG. Separate from rollEvent: it runs on
	// every answer, correct or wrong.
	clipRoll func(rng *mrand.Rand, correct bool, eligible []Clip, lastPlayedID int64, playsThisSession, sessionCap, chance int) *Clip
}

func NewService(store Store, mailer Mailer, log *slog.Logger) *Service {
	return &Service{store: store, mailer: mailer, log: log, rollEvent: RollEvent, clipRoll: ClipRoll}
}

// templateGenerators maps a template skill slug to its Generate function.
var templateGenerators = map[string]func(int, *mrand.Rand) (Payload, any, string){
	"multiplication": genMultiplication,
	"division":       genDivision,
	"addsub":         genAddSub,
	"fractions":      genFractions,
	"place_value":    genPlaceValue,
	"patterns":       genPatterns,
}

// newRand seeds a math/rand source from crypto/rand, per
// ARCHITECTURE.md -> "Question serving".
func newRand() *mrand.Rand {
	var seed int64
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		seed = int64(binary.LittleEndian.Uint64(buf[:]))
	} else {
		seed = time.Now().UnixNano()
	}
	return mrand.New(mrand.NewSource(seed))
}

// ---- serving ----

// NextQuestions serves count questions for skill (or, for skill=="mixed", a
// weighted round-robin across every registered skill). Template skills
// generate fresh questions and insert them; AI skills pick from the bank.
// The returned Questions still carry Answer/Explanation — stripping those
// before they reach the client is an HTTP-layer concern (phase 3).
func (s *Service) NextQuestions(ctx context.Context, skill string, count int, sessionID int64) ([]Question, bool, error) {
	if count <= 0 {
		count = 1
	}
	rng := newRand()

	if skill != "mixed" {
		return s.serveSkill(ctx, skill, count, rng)
	}

	counts, err := s.store.RecentAttemptCounts(ctx, time.Now().AddDate(0, 0, -7))
	if err != nil {
		return nil, false, fmt.Errorf("recent attempt counts: %w", err)
	}

	var out []Question
	var bankLow bool
	for i := 0; i < count; i++ {
		picked := weightedPickSkill(rng, counts)
		qs, low, err := s.serveSkill(ctx, picked, 1, rng)
		if err != nil {
			return nil, false, err
		}
		out = append(out, qs...)
		bankLow = bankLow || low
		counts[picked]++ // discourage repeats within this same batch
	}
	return out, bankLow, nil
}

// weightedPickSkill favors the skill with the fewest recent attempts.
// Integer-weighted (no floats): weight_i = (max(counts)+1) - counts[i].
func weightedPickSkill(rng *mrand.Rand, counts map[string]int) string {
	max := 0
	for _, s := range Skills {
		if c := counts[s.Slug]; c > max {
			max = c
		}
	}
	total := 0
	weights := make([]int, len(Skills))
	for i, s := range Skills {
		w := (max + 1) - counts[s.Slug]
		if w < 1 {
			w = 1
		}
		weights[i] = w
		total += w
	}
	r := rng.Intn(total)
	for i, w := range weights {
		if r < w {
			return Skills[i].Slug
		}
		r -= w
	}
	return Skills[len(Skills)-1].Slug
}

func (s *Service) serveSkill(ctx context.Context, skill string, n int, rng *mrand.Rand) ([]Question, bool, error) {
	level, err := s.effectiveLevel(ctx, skill)
	if err != nil {
		return nil, false, err
	}

	if gen, ok := templateGenerators[skill]; ok {
		out := make([]Question, 0, n)
		for i := 0; i < n; i++ {
			payload, answer, explanation := gen(level, rng)
			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return nil, false, fmt.Errorf("marshal payload: %w", err)
			}
			answerJSON, err := json.Marshal(answer)
			if err != nil {
				return nil, false, fmt.Errorf("marshal answer: %w", err)
			}
			q := &Question{
				Skill:       skill,
				Difficulty:  level,
				Source:      string(SourceTemplate),
				Payload:     payloadJSON,
				Answer:      answerJSON,
				Explanation: explanation,
			}
			if err := s.store.InsertQuestion(ctx, q); err != nil {
				return nil, false, fmt.Errorf("insert generated question: %w", err)
			}
			out = append(out, *q)
		}
		return out, false, nil
	}

	// AI skill: pick from the bank.
	qs, bankLow, err := s.store.PickAIQuestions(ctx, skill, level, n)
	if err != nil {
		return nil, false, fmt.Errorf("pick AI questions: %w", err)
	}
	if len(qs) < n {
		bankLow = true
	}
	if len(qs) > 0 {
		ids := make([]int64, len(qs))
		for i, q := range qs {
			ids[i] = q.ID
		}
		if err := s.store.BumpTimesServed(ctx, ids); err != nil {
			return nil, false, fmt.Errorf("bump times served: %w", err)
		}
	}
	return qs, bankLow, nil
}

// effectiveLevel is settings.level_override[skill] if present, else the
// skill's current adaptive level.
func (s *Service) effectiveLevel(ctx context.Context, skill string) (int, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return 0, fmt.Errorf("get settings: %w", err)
	}
	if lvl, ok := settings.LevelOverride[skill]; ok {
		return lvl, nil
	}
	state, err := s.store.GetSkillState(ctx, skill)
	if err != nil {
		return 0, fmt.Errorf("get skill state for %s: %w", skill, err)
	}
	if state == nil {
		return 1, nil
	}
	return state.Level, nil
}

// ---- attempts ----

// Attempt grades a submission, scores it, updates skill/quest/daily state,
// detects new unlocks, persists everything, and returns the result the PWA
// needs for the moment of feedback. localDay is an optional device-local
// YYYY-MM-DD (the PWA always sends it); when present, it's used to roll the
// screen-time dial over to a new day if this is the first interaction of
// that day. When absent, that rollover is skipped -- the next screentime GET
// catches it instead.
func (s *Service) Attempt(ctx context.Context, sessionID, questionID int64, given json.RawMessage, elapsedMS int, localDay ...string) (*AttemptResult, error) {
	var day string
	if len(localDay) > 0 {
		day = localDay[0]
	}

	q, err := s.store.GetQuestion(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("get question: %w", err)
	}
	if q == nil {
		return nil, fmt.Errorf("not found: question")
	}

	var payload Payload
	if err := json.Unmarshal(q.Payload, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	correct, err := Grade(payload.Kind, given, q.Answer)
	if err != nil {
		return nil, fmt.Errorf("grade: %w", err)
	}

	session, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("not found: session")
	}

	// Total XP before this attempt's XP lands, for the response and for
	// unlock threshold detection.
	statesBefore, err := s.store.ListSkillStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list skill states: %w", err)
	}
	xpBefore := totalXP(statesBefore)

	state, err := s.store.GetSkillState(ctx, q.Skill)
	if err != nil {
		return nil, fmt.Errorf("get skill state: %w", err)
	}
	if state == nil {
		state = &SkillState{Skill: q.Skill, Level: 1}
	}

	comeback := correct && state.WrongRun >= 3
	daily := session.Mode == ModeDaily

	if daily {
		answered, err := s.store.HasAttempt(ctx, sessionID, questionID)
		if err != nil {
			return nil, fmt.Errorf("check daily attempt: %w", err)
		}
		if answered {
			return nil, fmt.Errorf("conflict: question already answered today")
		}
	}

	var streakAfter int
	if correct {
		streakAfter = state.Streak + 1
	}
	xp := Score(q.Difficulty, elapsedMS, streakAfter, correct, comeback, daily)

	// Random events only ever fire on a correct answer, applied after
	// comeback/daily are already baked into xp -- a daily lucky-egg answer is
	// (base×speed×streak×2 daily)×2 event, stacking intentionally.
	var ev *Event
	var xpBeforeEvent int
	if correct {
		attemptsSinceLast, err := s.store.AttemptsSinceLastEvent(ctx)
		if err != nil {
			return nil, fmt.Errorf("attempts since last event: %w", err)
		}
		ev = s.rollEvent(newRand(), attemptsSinceLast, elapsedMS, q.Difficulty)
		if ev != nil {
			xpBeforeEvent = xp
			xp = ev.Apply(xp)
		}
	}

	newState, levelChanged := Adapt(*state, correct)
	newState.XP += int64(xp)
	newState.Streak = streakAfter
	if correct {
		newState.WrongRun = 0
	} else {
		newState.WrongRun = state.WrongRun + 1
	}
	if err := s.store.UpdateSkillState(ctx, &newState); err != nil {
		return nil, fmt.Errorf("update skill state: %w", err)
	}

	attempt := &Attempt{
		SessionID:   sessionID,
		QuestionID:  questionID,
		Skill:       q.Skill,
		Difficulty:  q.Difficulty,
		Given:       given,
		Correct:     correct,
		ElapsedMS:   elapsedMS,
		XPEarned:    xp,
		StreakAfter: newState.Streak,
		LevelAfter:  newState.Level,
	}
	if ev != nil {
		attempt.Event = ev.Slug
	}
	if err := s.store.InsertAttempt(ctx, attempt); err != nil {
		return nil, fmt.Errorf("insert attempt: %w", err)
	}

	clipResult, err := s.rollClip(ctx, sessionID, attempt.ID, correct)
	if err != nil {
		return nil, err
	}

	var rewardXP int
	var questUnlocks []Unlock
	if session.Mode == ModeQuest && correct {
		granted, qu, err := s.applyQuestProgress(ctx, q.Skill, q.Difficulty)
		if err != nil {
			return nil, err
		}
		rewardXP += granted
		questUnlocks = qu
		// Quest rewards aren't tied to a single practiced skill, so they
		// land in the bonus pseudo-skill row (see Catch, which does the
		// same for its +1000 XP) rather than q.Skill's own state -- keeps
		// per-skill XP an honest reflection of practice in that skill.
		if rewardXP > 0 {
			if err := s.addBonusXP(ctx, rewardXP); err != nil {
				return nil, err
			}
		}
	}

	if session.Mode == ModeDaily {
		if err := s.applyDailyProgress(ctx, questionID, correct, elapsedMS, xp); err != nil {
			return nil, err
		}
	}

	xpAfter := xpBefore + int64(xp) + int64(rewardXP)

	thresholdUnlocks, err := s.detectAndPersistUnlocks(ctx, xpBefore, xpAfter)
	if err != nil {
		return nil, err
	}
	unlocks := append(questUnlocks, thresholdUnlocks...)

	var eventResult *EventResult
	if ev != nil {
		eventResult = &EventResult{
			Slug:       ev.Slug,
			Name:       ev.Name,
			Message:    ev.Message,
			Multiplier: ev.MultiplierString(),
			XPBefore:   xpBeforeEvent,
		}
	}

	if day != "" {
		if err := s.EnsureDailyReset(ctx, day); err != nil {
			return nil, fmt.Errorf("ensure daily reset: %w", err)
		}
	}
	var screenTimeMinutes int
	if correct {
		st, err := s.computeScreenTime(ctx)
		if err != nil {
			return nil, fmt.Errorf("screen time: %w", err)
		}
		screenTimeMinutes = st.MinutesEarned
	}

	return &AttemptResult{
		Correct:           correct,
		Answer:            q.Answer,
		Explanation:       q.Explanation,
		XPEarned:          xp,
		Comeback:          comeback,
		Streak:            newState.Streak,
		SkillLevel:        newState.Level,
		LevelChanged:      levelChanged,
		XP:                xpAfter,
		XPBefore:          xpBefore,
		Unlocks:           unlocks,
		Event:             eventResult,
		ScreenTimeMinutes: screenTimeMinutes,
		Clip:              clipResult,
	}, nil
}

// rollClip is the video-clip trigger: a separate roll from rollEvent, run on
// every answer (correct or wrong), independent of the XP-event branch. On a
// pick, it records the play (bumping clips.play_count) and returns the
// API-facing ClipPlay to attach to the result.
func (s *Service) rollClip(ctx context.Context, sessionID, attemptID int64, correct bool) (*ClipPlay, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get settings for clip roll: %w", err)
	}
	clips, err := s.store.ListClips(ctx)
	if err != nil {
		return nil, fmt.Errorf("list clips: %w", err)
	}
	if len(clips) == 0 {
		return nil, nil
	}
	lastPlayedID, err := s.store.LastPlayedClipID(ctx)
	if err != nil {
		return nil, fmt.Errorf("last played clip id: %w", err)
	}
	playsThisSession, err := s.store.CountClipPlaysInSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("count clip plays in session: %w", err)
	}

	clip := s.clipRoll(newRand(), correct, clips, lastPlayedID, playsThisSession, settings.ClipSessionCap, settings.ClipChance)
	if clip == nil {
		return nil, nil
	}

	trigger := "wrong"
	if correct {
		trigger = "correct"
	}
	if err := s.store.InsertClipPlay(ctx, clip.ID, attemptID, trigger); err != nil {
		return nil, fmt.Errorf("insert clip play: %w", err)
	}

	return &ClipPlay{ID: clip.ID, Title: clip.Title, URL: clip.URL, ContentType: clip.ContentType}, nil
}

// ---- sessions ----

func (s *Service) CreateSession(ctx context.Context, mode string) (*Session, error) {
	sess := &Session{Mode: mode}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

func (s *Service) EndSession(ctx context.Context, id int64) error {
	if err := s.store.EndSession(ctx, id); err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	return nil
}

// ---- settings ----

func (s *Service) GetSettings(ctx context.Context) (*Settings, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	return settings, nil
}

func (s *Service) UpdateSettings(ctx context.Context, in *Settings) (*Settings, error) {
	in.ID = 1
	if in.DailyCount <= 0 {
		return nil, fmt.Errorf("invalid: daily_count must be positive")
	}
	if in.MinutesPerCorrect <= 0 {
		return nil, fmt.Errorf("invalid: minutes_per_correct must be positive")
	}
	if in.ClipChance <= 0 {
		return nil, fmt.Errorf("invalid: clip_chance must be positive")
	}
	if in.ClipSessionCap < 0 {
		return nil, fmt.Errorf("invalid: clip_session_cap must be non-negative")
	}
	if in.LevelOverride == nil {
		in.LevelOverride = map[string]int{}
	}
	for skill, level := range in.LevelOverride {
		if level < 1 || level > 10 {
			return nil, fmt.Errorf("invalid: level_override[%s] must be between 1 and 10", skill)
		}
	}
	if err := s.store.UpdateSettings(ctx, in); err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}
	return s.GetSettings(ctx)
}

// ---- question review (parent view) ----

// ListQuestions supports GET /api/questions; skill/source/retired are
// optional filters (empty/nil means "any").
func (s *Service) ListQuestions(ctx context.Context, skill, source string, retired *bool) ([]Question, error) {
	qs, err := s.store.ListQuestions(ctx, skill, source, retired)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	return qs, nil
}

// SetQuestionRetired backs POST /api/questions/{id}/retire|unretire.
// Retired questions never get served (PickAIQuestions already filters).
// Store's "not found: question" sentinel is passed through unwrapped so
// GameHandler.fail's prefix match still routes it to 404.
func (s *Service) SetQuestionRetired(ctx context.Context, id int64, retired bool) error {
	return s.store.SetQuestionRetired(ctx, id, retired)
}

// ---- clips (parent manage view) ----

func (s *Service) ListClips(ctx context.Context) ([]Clip, error) {
	clips, err := s.store.ListClips(ctx)
	if err != nil {
		return nil, fmt.Errorf("list clips: %w", err)
	}
	return clips, nil
}

func (s *Service) GetClip(ctx context.Context, id int64) (*Clip, error) {
	c, err := s.store.GetClip(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get clip: %w", err)
	}
	return c, nil
}

// CreateClip records a clip row after its bytes have already been uploaded
// to R2 (the handler does the upload; this just persists the metadata).
func (s *Service) CreateClip(ctx context.Context, c *Clip) error {
	if c.Title == "" {
		return fmt.Errorf("invalid: title is required")
	}
	if c.Weight <= 0 {
		c.Weight = 1
	}
	if err := s.store.InsertClip(ctx, c); err != nil {
		return fmt.Errorf("insert clip: %w", err)
	}
	return nil
}

// UpdateClip changes a clip's conditions only (title, enabled, on_correct,
// on_wrong, weight) -- never the file. Delete + re-upload replaces bytes.
func (s *Service) UpdateClip(ctx context.Context, id int64, title string, enabled, onCorrect, onWrong bool, weight int) error {
	if title == "" {
		return fmt.Errorf("invalid: title is required")
	}
	if weight <= 0 {
		return fmt.Errorf("invalid: weight must be positive")
	}
	return s.store.UpdateClipConditions(ctx, id, title, enabled, onCorrect, onWrong, weight)
}

// DeleteClip removes only the DB row; the caller (handler) deletes the R2
// object first so a failure there can't orphan the object.
func (s *Service) DeleteClip(ctx context.Context, id int64) error {
	return s.store.DeleteClip(ctx, id)
}

func (s *Service) ListClipPlays(ctx context.Context, limit int) ([]ClipPlayLog, error) {
	plays, err := s.store.ListClipPlays(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list clip plays: %w", err)
	}
	return plays, nil
}

// ---- admin ----

// ResetProgress wipes all player progress back to a fresh-start state (see
// Store.ResetProgress for exactly what's cleared vs preserved).
func (s *Service) ResetProgress(ctx context.Context) error {
	if err := s.store.ResetProgress(ctx); err != nil {
		return fmt.Errorf("reset progress: %w", err)
	}
	s.log.Warn("progress reset to zero")
	return nil
}

// ---- export ----

func (s *Service) Export(ctx context.Context) (map[string]any, error) {
	doc, err := s.store.ExportAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("export all: %w", err)
	}
	return doc, nil
}

// totalXP = 100 + lifetime XP summed across every skill. It only goes up.
func totalXP(states []SkillState) int64 {
	var total int64 = 100
	for _, s := range states {
		total += s.XP
	}
	return total
}

// applyQuestProgress advances every incomplete quest chapter whose
// requirement is satisfied by this correct attempt (skill listed, min
// difficulty met), granting the reward and returning any XP it carries plus
// the unlocks it directly granted (pokemon/gym-badge rewards — these are
// separate from the xp/streak/saga threshold unlocks detectAndPersistUnlocks
// finds, so the caller must merge both into the response).
func (s *Service) applyQuestProgress(ctx context.Context, skill string, difficulty int) (int, []Unlock, error) {
	chapters, err := s.store.ListQuestChapters(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("list quest chapters: %w", err)
	}

	var rewardXP int
	var unlocks []Unlock
	for _, ch := range chapters {
		if ch.CompletedAt != nil {
			continue
		}
		if difficulty < ch.Requirement.MinDifficulty {
			continue
		}
		if !containsString(ch.Requirement.Skills, skill) {
			continue
		}

		progress := ch.Progress + 1
		var completedAt *time.Time
		if progress >= ch.Requirement.Correct {
			now := time.Now().UTC()
			completedAt = &now
			rewardXP += ch.Reward.XP
			source := fmt.Sprintf("saga %s ch%d", ch.Saga, ch.Chapter)
			if ch.Reward.Pokemon != "" {
				u := &Unlock{Kind: UnlockPokemon, Ref: ch.Reward.Pokemon, Source: source}
				inserted, err := s.store.InsertUnlock(ctx, u)
				if err != nil {
					return 0, nil, fmt.Errorf("insert quest pokemon unlock: %w", err)
				}
				if inserted {
					if p, ok := PokemonBySlug(ch.Reward.Pokemon); ok {
						u.Name = p.Name
						u.Rarity = string(p.Rarity)
					}
					unlocks = append(unlocks, *u)
				}
			}
			if ch.Reward.GymBadge > 0 {
				u := &Unlock{Kind: UnlockGymBadge, Ref: fmt.Sprintf("%d", ch.Reward.GymBadge), Source: source}
				inserted, err := s.store.InsertUnlock(ctx, u)
				if err != nil {
					return 0, nil, fmt.Errorf("insert gym badge unlock: %w", err)
				}
				if inserted {
					u.Name = fmt.Sprintf("Gym Badge %d", ch.Reward.GymBadge)
					unlocks = append(unlocks, *u)
				}
			}
		}
		if err := s.store.UpdateQuestProgress(ctx, ch.ID, progress, completedAt); err != nil {
			return 0, nil, fmt.Errorf("update quest progress: %w", err)
		}
	}
	return rewardXP, unlocks, nil
}

// applyDailyProgress updates the daily_results row containing questionID
// (found by scanning recent days rather than assuming "today", so a session
// spanning midnight still lands on the right day), applying the perfect-day
// bonus once the set completes with everything correct.
func (s *Service) applyDailyProgress(ctx context.Context, questionID int64, correct bool, elapsedMS, xp int) error {
	results, err := s.store.ListDailyResults(ctx, time.Now().AddDate(0, 0, -2).Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("list daily results: %w", err)
	}
	var d *DailyResult
	for i := range results {
		if containsInt64(results[i].QuestionIDs, questionID) {
			d = &results[i]
			break
		}
	}
	if d == nil {
		return fmt.Errorf("not found: daily result for question %d", questionID)
	}

	d.Answered++
	if correct {
		d.Correct++
	}
	d.ElapsedMS += elapsedMS
	d.XPEarned += xp

	if d.Answered >= len(d.QuestionIDs) && d.CompletedAt == nil {
		now := time.Now().UTC()
		d.CompletedAt = &now
		if d.Correct == len(d.QuestionIDs) {
			d.XPEarned += PerfectDayBonus()
		}
	}

	return s.store.UpdateDailyResult(ctx, d)
}

// detectAndPersistUnlocks runs DetectUnlocks against the total-XP delta
// (plus current daily streak / saga completions), inserts any newly-earned
// rows, and fills in Name/Rarity for the API response.
func (s *Service) detectAndPersistUnlocks(ctx context.Context, xpBefore, xpAfter int64) ([]Unlock, error) {
	existing, err := s.store.ListUnlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list unlocks: %w", err)
	}
	already := make(map[string]bool, len(existing))
	for _, u := range existing {
		already[u.Kind+":"+u.Ref] = true
	}

	dailyResults, err := s.store.ListDailyResults(ctx, time.Now().AddDate(0, 0, -60).Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list daily results for streak: %w", err)
	}
	streak := dailyStreakFrom(dailyResults)

	chapters, err := s.store.ListQuestChapters(ctx)
	if err != nil {
		return nil, fmt.Errorf("list quest chapters for sagas: %w", err)
	}
	sagaCompletions := sagaCompletionsFrom(chapters)

	newlyEarned := DetectUnlocks(xpBefore, xpAfter, streak, sagaCompletions, already)

	var out []Unlock
	for _, p := range newlyEarned {
		kind := UnlockPokemon
		source := fmt.Sprintf("xp %d", p.Condition.XP)
		switch p.Condition.Type {
		case "streak":
			kind = UnlockRibbon
			source = fmt.Sprintf("daily streak %d days", p.Condition.StreakDays)
		case "saga":
			source = fmt.Sprintf("saga %s ch%d", p.Condition.Saga, p.Condition.Chapter)
		}
		u := &Unlock{Kind: kind, Ref: p.Slug, Source: source}
		inserted, err := s.store.InsertUnlock(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("insert unlock %s: %w", p.Slug, err)
		}
		if !inserted {
			continue
		}
		u.Name = p.Name
		u.Rarity = string(p.Rarity)
		out = append(out, *u)
	}
	return out, nil
}

// dailyStreakFrom computes the current consecutive-day completed streak
// from a set of daily_results, ending at the most recent completed day.
func dailyStreakFrom(results []DailyResult) int {
	byDay := make(map[string]bool, len(results))
	for _, r := range results {
		if r.CompletedAt != nil {
			byDay[r.Day] = true
		}
	}
	day := time.Now().UTC()
	if !byDay[day.Format("2006-01-02")] {
		// Today not completed yet (or no attempt today) — that shouldn't
		// zero out an otherwise-current streak, so start counting from
		// yesterday.
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for byDay[day.Format("2006-01-02")] {
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak
}

// sagaCompletionsFrom marks a saga complete when its highest-numbered
// chapter has CompletedAt set.
func sagaCompletionsFrom(chapters []QuestChapter) map[string]bool {
	highest := map[string]int{}
	for _, ch := range chapters {
		if ch.Chapter > highest[ch.Saga] {
			highest[ch.Saga] = ch.Chapter
		}
	}
	completions := map[string]bool{}
	for _, ch := range chapters {
		if ch.Chapter == highest[ch.Saga] && ch.CompletedAt != nil {
			completions[ch.Saga] = true
		}
	}
	return completions
}

// addBonusXP credits xp to the bonus pseudo-skill row, creating it on first
// use.
func (s *Service) addBonusXP(ctx context.Context, xp int) error {
	bonus, err := s.store.GetSkillState(ctx, BonusSkillSlug)
	if err != nil {
		return fmt.Errorf("get bonus skill state: %w", err)
	}
	if bonus == nil {
		bonus = &SkillState{Skill: BonusSkillSlug, Level: 1}
	}
	bonus.XP += int64(xp)
	if err := s.store.UpdateSkillState(ctx, bonus); err != nil {
		return fmt.Errorf("update bonus skill state: %w", err)
	}
	return nil
}

// Catch grants pokemonSlug in exchange for the 8 gym badges (the Master
// Ball), per ARCHITECTURE.md "Collection, quests, daily": +1000 XP,
// consumes the badges. Returns "conflict: ..." if fewer than 8 badges are
// held, "invalid: ..." for an unknown or already-unlocked Pokémon.
func (s *Service) Catch(ctx context.Context, pokemonSlug string) (*Unlock, error) {
	pokemon, ok := PokemonBySlug(pokemonSlug)
	if !ok {
		return nil, fmt.Errorf("invalid: unknown pokemon %q", pokemonSlug)
	}

	badgeCount, already, err := s.store.Catch(ctx, pokemonSlug, BonusSkillSlug, CatchXP)
	if err != nil {
		return nil, fmt.Errorf("catch: %w", err)
	}
	if badgeCount != 8 {
		return nil, fmt.Errorf("conflict: need all 8 gym badges (have %d)", badgeCount)
	}
	if already {
		return nil, fmt.Errorf("invalid: %s is already unlocked", pokemon.Name)
	}

	return &Unlock{
		Kind: UnlockPokemon, Ref: pokemon.Slug, Source: "catch",
		Name: pokemon.Name, Rarity: string(pokemon.Rarity),
	}, nil
}

func containsString(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func containsInt64(ss []int64, v int64) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
