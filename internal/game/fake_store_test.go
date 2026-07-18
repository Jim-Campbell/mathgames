package game

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// fakeStore is an in-memory game.Store for service-level tests, mirroring
// ~/projects/food/internal/food/fake_store_test.go's style.
type fakeStore struct {
	questions   map[int64]*Question
	nextQID     int64
	attempts    []Attempt
	nextAID     int64
	skillStates map[string]*SkillState
	sessions    map[int64]*Session
	nextSID     int64
	unlocks     map[string]*Unlock // key kind:ref
	nextUID     int64
	chapters    map[int64]*QuestChapter
	nextChID    int64
	daily       map[string]*DailyResult
	settings    Settings
	batches     []AIBatch
	nextBatchID int64
	stResets    []ScreenTimeReset
	nextSTRID   int64
	clips       map[int64]*Clip
	nextClipID  int64
	clipPlays   []clipPlayRow
	nextCPID    int64
	messages    []Message
	nextMsgID   int64
}

// clipPlayRow mirrors a clip_plays DB row for the fake store.
type clipPlayRow struct {
	ID        int64
	ClipID    int64
	AttemptID int64
	Trigger   string
	PlayedAt  time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		questions:   map[int64]*Question{},
		skillStates: map[string]*SkillState{},
		sessions:    map[int64]*Session{},
		unlocks:     map[string]*Unlock{},
		chapters:    map[int64]*QuestChapter{},
		daily:       map[string]*DailyResult{},
		settings:    Settings{ID: 1, DailyCount: 5, LevelOverride: map[string]int{}, MinutesPerCorrect: 3, ClipChance: 40, ClipSessionCap: 2},
		clips:       map[int64]*Clip{},
	}
}

func (f *fakeStore) InsertQuestion(ctx context.Context, q *Question) error {
	f.nextQID++
	q.ID = f.nextQID
	q.CreatedAt = time.Now()
	cp := *q
	f.questions[q.ID] = &cp
	return nil
}

func (f *fakeStore) GetQuestion(ctx context.Context, id int64) (*Question, error) {
	q, ok := f.questions[id]
	if !ok {
		return nil, nil
	}
	cp := *q
	return &cp, nil
}

func (f *fakeStore) PickAIQuestions(ctx context.Context, skill string, level, n int) ([]Question, bool, error) {
	var out []Question
	for _, q := range f.questions {
		if q.Skill == skill && q.Difficulty == level && q.Source == string(SourceAI) && !q.Retired {
			out = append(out, *q)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TimesServed < out[j].TimesServed })
	bankLow := len(out) < n
	if len(out) > n {
		out = out[:n]
	}
	return out, bankLow, nil
}

func (f *fakeStore) BumpTimesServed(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		if q, ok := f.questions[id]; ok {
			q.TimesServed++
		}
	}
	return nil
}

func (f *fakeStore) ListQuestions(ctx context.Context, skill, source string, retired *bool) ([]Question, error) {
	var out []Question
	for _, q := range f.questions {
		if skill != "" && q.Skill != skill {
			continue
		}
		if source != "" && q.Source != source {
			continue
		}
		if retired != nil && q.Retired != *retired {
			continue
		}
		out = append(out, *q)
	}
	return out, nil
}

func (f *fakeStore) SetQuestionRetired(ctx context.Context, id int64, retired bool) error {
	q, ok := f.questions[id]
	if !ok {
		return fmt.Errorf("not found: question")
	}
	q.Retired = retired
	return nil
}

func (f *fakeStore) InsertAttempt(ctx context.Context, a *Attempt) error {
	f.nextAID++
	a.ID = f.nextAID
	a.CreatedAt = time.Now()
	f.attempts = append(f.attempts, *a)
	return nil
}

func (f *fakeStore) HasAttempt(ctx context.Context, sessionID, questionID int64) (bool, error) {
	for _, a := range f.attempts {
		if a.SessionID == sessionID && a.QuestionID == questionID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) ListAttempts(ctx context.Context, since time.Time) ([]Attempt, error) {
	var out []Attempt
	for _, a := range f.attempts {
		if !a.CreatedAt.Before(since) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeStore) AttemptsSinceLastEvent(ctx context.Context) (int, error) {
	var lastEventID int64
	for _, a := range f.attempts {
		if a.Event != "" && a.ID > lastEventID {
			lastEventID = a.ID
		}
	}
	count := 0
	for _, a := range f.attempts {
		if a.ID > lastEventID {
			count++
		}
	}
	return count, nil
}

func (f *fakeStore) GetSkillState(ctx context.Context, skill string) (*SkillState, error) {
	s, ok := f.skillStates[skill]
	if !ok {
		return &SkillState{Skill: skill, Level: 1}, nil
	}
	cp := *s
	return &cp, nil
}

func (f *fakeStore) ListSkillStates(ctx context.Context) ([]SkillState, error) {
	var out []SkillState
	for _, s := range f.skillStates {
		out = append(out, *s)
	}
	return out, nil
}

func (f *fakeStore) UpdateSkillState(ctx context.Context, s *SkillState) error {
	cp := *s
	cp.UpdatedAt = time.Now()
	f.skillStates[s.Skill] = &cp
	return nil
}

func (f *fakeStore) CreateSession(ctx context.Context, s *Session) error {
	f.nextSID++
	s.ID = f.nextSID
	s.StartedAt = time.Now()
	cp := *s
	f.sessions[s.ID] = &cp
	return nil
}

func (f *fakeStore) EndSession(ctx context.Context, id int64) error {
	if s, ok := f.sessions[id]; ok {
		now := time.Now()
		s.EndedAt = &now
	}
	return nil
}

func (f *fakeStore) GetSession(ctx context.Context, id int64) (*Session, error) {
	s, ok := f.sessions[id]
	if !ok {
		return nil, nil
	}
	cp := *s
	return &cp, nil
}

func (f *fakeStore) RecentAttemptCounts(ctx context.Context, since time.Time) (map[string]int, error) {
	out := map[string]int{}
	for _, a := range f.attempts {
		if !a.CreatedAt.Before(since) {
			out[a.Skill]++
		}
	}
	return out, nil
}

func (f *fakeStore) ListUnlocks(ctx context.Context) ([]Unlock, error) {
	var out []Unlock
	for _, u := range f.unlocks {
		out = append(out, *u)
	}
	return out, nil
}

func (f *fakeStore) InsertUnlock(ctx context.Context, u *Unlock) (bool, error) {
	key := u.Kind + ":" + u.Ref
	if _, exists := f.unlocks[key]; exists {
		return false, nil
	}
	f.nextUID++
	u.ID = f.nextUID
	u.CreatedAt = time.Now()
	cp := *u
	f.unlocks[key] = &cp
	return true, nil
}

func (f *fakeStore) DeleteUnlocks(ctx context.Context, kind string, refs []string) error {
	for _, ref := range refs {
		delete(f.unlocks, kind+":"+ref)
	}
	return nil
}

func (f *fakeStore) Catch(ctx context.Context, pokemonRef, bonusSkill string, bonusXP int64) (int, bool, error) {
	badgeCount := 0
	for _, u := range f.unlocks {
		if u.Kind == UnlockGymBadge {
			badgeCount++
		}
	}
	if badgeCount != 8 {
		return badgeCount, false, nil
	}
	if _, exists := f.unlocks[UnlockPokemon+":"+pokemonRef]; exists {
		return badgeCount, true, nil
	}

	f.nextUID++
	f.unlocks[UnlockPokemon+":"+pokemonRef] = &Unlock{
		ID: f.nextUID, Kind: UnlockPokemon, Ref: pokemonRef, Source: "catch", CreatedAt: time.Now(),
	}
	for ref := range f.unlocks {
		if f.unlocks[ref].Kind == UnlockGymBadge {
			delete(f.unlocks, ref)
		}
	}
	bonus, ok := f.skillStates[bonusSkill]
	if !ok {
		bonus = &SkillState{Skill: bonusSkill, Level: 1}
	}
	cp := *bonus
	cp.XP += bonusXP
	cp.UpdatedAt = time.Now()
	f.skillStates[bonusSkill] = &cp
	return badgeCount, false, nil
}

func (f *fakeStore) ListQuestChapters(ctx context.Context) ([]QuestChapter, error) {
	var out []QuestChapter
	for _, ch := range f.chapters {
		out = append(out, *ch)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Saga != out[j].Saga {
			return out[i].Saga < out[j].Saga
		}
		return out[i].Chapter < out[j].Chapter
	})
	return out, nil
}

func (f *fakeStore) GetQuestChapter(ctx context.Context, saga string, chapter int) (*QuestChapter, error) {
	for _, ch := range f.chapters {
		if ch.Saga == saga && ch.Chapter == chapter {
			cp := *ch
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) UpdateQuestProgress(ctx context.Context, id int64, progress int, completedAt *time.Time) error {
	if ch, ok := f.chapters[id]; ok {
		ch.Progress = progress
		ch.CompletedAt = completedAt
	}
	return nil
}

func (f *fakeStore) GetDailyResult(ctx context.Context, day string) (*DailyResult, error) {
	d, ok := f.daily[day]
	if !ok {
		return nil, nil
	}
	cp := *d
	return &cp, nil
}

func (f *fakeStore) CreateDailyResult(ctx context.Context, d *DailyResult) error {
	cp := *d
	f.daily[d.Day] = &cp
	return nil
}

func (f *fakeStore) UpdateDailyResult(ctx context.Context, d *DailyResult) error {
	cp := *d
	f.daily[d.Day] = &cp
	return nil
}

func (f *fakeStore) ListDailyResults(ctx context.Context, sinceDay string) ([]DailyResult, error) {
	var out []DailyResult
	for day, d := range f.daily {
		if day >= sinceDay {
			out = append(out, *d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out, nil
}

func (f *fakeStore) GetSettings(ctx context.Context) (*Settings, error) {
	cp := f.settings
	cp.LevelOverride = map[string]int{}
	for k, v := range f.settings.LevelOverride {
		cp.LevelOverride[k] = v
	}
	return &cp, nil
}

func (f *fakeStore) UpdateSettings(ctx context.Context, s *Settings) error {
	f.settings = *s
	return nil
}

func (f *fakeStore) ExportAll(ctx context.Context) (map[string]any, error) {
	return map[string]any{
		"questions":          f.questions,
		"attempts":           f.attempts,
		"skill_state":        f.skillStates,
		"unlocks":            f.unlocks,
		"chapters":           f.chapters,
		"daily":              f.daily,
		"settings":           f.settings,
		"screen_time_resets": f.stResets,
	}, nil
}

func (f *fakeStore) InsertAIBatch(ctx context.Context, b *AIBatch) error {
	f.nextBatchID++
	b.ID = f.nextBatchID
	b.CreatedAt = time.Now()
	f.batches = append(f.batches, *b)
	return nil
}

func (f *fakeStore) UpdateAIBatchCounts(ctx context.Context, id int64, accepted, rejected int) error {
	for i := range f.batches {
		if f.batches[i].ID == id {
			f.batches[i].Accepted = accepted
			f.batches[i].Rejected = rejected
		}
	}
	return nil
}

func (f *fakeStore) UpdateQuestChapterStory(ctx context.Context, id int64, title, story string, aiBatchID int64) error {
	if ch, ok := f.chapters[id]; ok {
		ch.Title = title
		ch.Story = story
		ch.AIBatchID = &aiBatchID
	}
	return nil
}

func (f *fakeStore) ResetProgress(ctx context.Context) error {
	f.attempts = nil
	f.sessions = map[int64]*Session{}
	f.unlocks = map[string]*Unlock{}
	f.daily = map[string]*DailyResult{}
	for skill, s := range f.skillStates {
		f.skillStates[skill] = &SkillState{Skill: s.Skill, Level: 1, UpdatedAt: time.Now()}
	}
	for _, ch := range f.chapters {
		ch.Progress = 0
		ch.CompletedAt = nil
	}
	for _, q := range f.questions {
		q.TimesServed = 0
	}
	f.settings.LevelOverride = map[string]int{}
	return nil
}

func (f *fakeStore) CountCorrectsSince(ctx context.Context, since *time.Time) (int, error) {
	count := 0
	for _, a := range f.attempts {
		if !a.Correct {
			continue
		}
		if since != nil && a.CreatedAt.Before(*since) {
			continue
		}
		count++
	}
	return count, nil
}

func (f *fakeStore) InsertScreenTimeReset(ctx context.Context, r *ScreenTimeReset) error {
	f.nextSTRID++
	r.ID = f.nextSTRID
	r.ResetAt = time.Now()
	f.stResets = append(f.stResets, *r)
	return nil
}

func (f *fakeStore) InsertDailyResetIfNew(ctx context.Context, localDay string, resetAt time.Time, minutesRedeemed, correctsCounted int) (bool, error) {
	for _, r := range f.stResets {
		if r.Reason == "daily" && r.Day != nil && *r.Day == localDay {
			return false, nil
		}
	}
	f.nextSTRID++
	day := localDay
	f.stResets = append(f.stResets, ScreenTimeReset{
		ID:              f.nextSTRID,
		ResetAt:         resetAt,
		MinutesRedeemed: minutesRedeemed,
		CorrectsCounted: correctsCounted,
		Reason:          "daily",
		Day:             &day,
	})
	return true, nil
}

func (f *fakeStore) LastScreenTimeReset(ctx context.Context) (*ScreenTimeReset, error) {
	if len(f.stResets) == 0 {
		return nil, nil
	}
	latest := f.stResets[0]
	for _, r := range f.stResets[1:] {
		if r.ResetAt.After(latest.ResetAt) {
			latest = r
		}
	}
	return &latest, nil
}

func (f *fakeStore) ListScreenTimeResets(ctx context.Context) ([]ScreenTimeReset, error) {
	out := make([]ScreenTimeReset, len(f.stResets))
	copy(out, f.stResets)
	sort.Slice(out, func(i, j int) bool { return out[i].ResetAt.After(out[j].ResetAt) })
	return out, nil
}

// ---- clips ----

func (f *fakeStore) ListClips(ctx context.Context) ([]Clip, error) {
	var out []Clip
	for _, c := range f.clips {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) GetClip(ctx context.Context, id int64) (*Clip, error) {
	c, ok := f.clips[id]
	if !ok {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}

func (f *fakeStore) InsertClip(ctx context.Context, c *Clip) error {
	f.nextClipID++
	c.ID = f.nextClipID
	c.CreatedAt = time.Now()
	cp := *c
	f.clips[c.ID] = &cp
	return nil
}

func (f *fakeStore) UpdateClipConditions(ctx context.Context, id int64, title string, enabled, onCorrect, onWrong bool, weight int) error {
	c, ok := f.clips[id]
	if !ok {
		return fmt.Errorf("not found: clip")
	}
	c.Title = title
	c.Enabled = enabled
	c.OnCorrect = onCorrect
	c.OnWrong = onWrong
	c.Weight = weight
	return nil
}

func (f *fakeStore) DeleteClip(ctx context.Context, id int64) error {
	delete(f.clips, id)
	return nil
}

func (f *fakeStore) CountClipPlaysInSession(ctx context.Context, sessionID int64) (int, error) {
	count := 0
	for _, cp := range f.clipPlays {
		for _, a := range f.attempts {
			if a.ID == cp.AttemptID && a.SessionID == sessionID {
				count++
				break
			}
		}
	}
	return count, nil
}

func (f *fakeStore) LastPlayedClipID(ctx context.Context) (int64, error) {
	if len(f.clipPlays) == 0 {
		return 0, nil
	}
	latest := f.clipPlays[0]
	for _, cp := range f.clipPlays[1:] {
		if cp.ID > latest.ID {
			latest = cp
		}
	}
	return latest.ClipID, nil
}

func (f *fakeStore) InsertClipPlay(ctx context.Context, clipID, attemptID int64, trigger string) error {
	f.nextCPID++
	f.clipPlays = append(f.clipPlays, clipPlayRow{
		ID: f.nextCPID, ClipID: clipID, AttemptID: attemptID, Trigger: trigger, PlayedAt: time.Now(),
	})
	if c, ok := f.clips[clipID]; ok {
		c.PlayCount++
	}
	return nil
}

func (f *fakeStore) ListClipPlays(ctx context.Context, limit int) ([]ClipPlayLog, error) {
	out := make([]ClipPlayLog, 0, len(f.clipPlays))
	for _, cp := range f.clipPlays {
		title := ""
		if c, ok := f.clips[cp.ClipID]; ok {
			title = c.Title
		}
		out = append(out, ClipPlayLog{
			ID: cp.ID, ClipID: cp.ClipID, ClipTitle: title,
			AttemptID: &cp.AttemptID, Trigger: cp.Trigger, PlayedAt: cp.PlayedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PlayedAt.After(out[j].PlayedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---- messages ----

func (f *fakeStore) InsertMessage(ctx context.Context, m *Message) error {
	f.nextMsgID++
	m.ID = f.nextMsgID
	m.CreatedAt = time.Now()
	f.messages = append(f.messages, *m)
	return nil
}

func (f *fakeStore) ListMessages(ctx context.Context) ([]Message, error) {
	out := make([]Message, len(f.messages))
	copy(out, f.messages)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (f *fakeStore) UpdateMessageEmailStatus(ctx context.Context, id int64, emailed bool, emailError string) error {
	for i := range f.messages {
		if f.messages[i].ID == id {
			f.messages[i].Emailed = emailed
			f.messages[i].EmailError = emailError
		}
	}
	return nil
}

func (f *fakeStore) MarkMessageRead(ctx context.Context, id int64) error {
	for i := range f.messages {
		if f.messages[i].ID == id {
			if f.messages[i].ReadAt == nil {
				now := time.Now()
				f.messages[i].ReadAt = &now
			}
			return nil
		}
	}
	return fmt.Errorf("not found: message")
}

func (f *fakeStore) CountUnreadMessages(ctx context.Context) (int, error) {
	count := 0
	for _, m := range f.messages {
		if m.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

func (f *fakeStore) CountMessagesSince(ctx context.Context, t time.Time) (int, error) {
	count := 0
	for _, m := range f.messages {
		if !m.CreatedAt.Before(t) {
			count++
		}
	}
	return count, nil
}

var _ Store = (*fakeStore)(nil)

// addChapter is a test helper for seeding a quest chapter directly (bypassing
// any store method, since chapters are seeded by migration 002 in the real
// app, not created through the Store interface in phase 2).
func (f *fakeStore) addChapter(ch QuestChapter) *QuestChapter {
	f.nextChID++
	ch.ID = f.nextChID
	cp := ch
	f.chapters[ch.ID] = &cp
	return &cp
}
