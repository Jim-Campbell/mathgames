package game

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// ---- profile ----

// Profile is the response shape for GET /api/profile.
type Profile struct {
	PowerLevel       int64            `json:"power_level"`
	XPBySkill        map[string]int64 `json:"xp_by_skill"`
	Levels           map[string]int   `json:"levels"`
	Streaks          map[string]int   `json:"streaks"`
	FightersUnlocked int              `json:"fighters_unlocked"`
	FightersTotal    int              `json:"fighters_total"`
	DragonBalls      []int            `json:"dragon_balls"`
	DailyStreak      int              `json:"daily_streak"`
	Days             []DayCount       `json:"days"` // per-day answer tally, last 30 days, for the Home activity bar
}

// DayCount is one calendar day's correct/wrong answer tally across all
// practice modes, for the Home activity bar.
type DayCount struct {
	Day     string `json:"day"`
	Correct int    `json:"correct"`
	Wrong   int    `json:"wrong"`
}

// dayCounts tallies attempts into per-day correct/wrong buckets, ascending by
// date. Days with no attempts are omitted (the client lays them out on a
// continuous axis). Never returns nil, so the JSON is always an array.
func dayCounts(attempts []Attempt) []DayCount {
	byDay := map[string]*DayCount{}
	for _, a := range attempts {
		day := a.CreatedAt.UTC().Format("2006-01-02")
		d, ok := byDay[day]
		if !ok {
			d = &DayCount{Day: day}
			byDay[day] = d
		}
		if a.Correct {
			d.Correct++
		} else {
			d.Wrong++
		}
	}
	out := make([]DayCount, 0, len(byDay))
	for _, d := range byDay {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}

func (s *Service) Profile(ctx context.Context) (*Profile, error) {
	states, err := s.store.ListSkillStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("list skill states: %w", err)
	}
	bySkill := make(map[string]SkillState, len(states))
	for _, st := range states {
		bySkill[st.Skill] = st
	}

	p := &Profile{
		PowerLevel: powerLevel(states),
		XPBySkill:  map[string]int64{},
		Levels:     map[string]int{},
		Streaks:    map[string]int{},
	}
	for _, sk := range Skills {
		st, ok := bySkill[sk.Slug]
		if !ok {
			st = SkillState{Skill: sk.Slug, Level: 1}
		}
		p.XPBySkill[sk.Slug] = st.XP
		p.Levels[sk.Slug] = st.Level
		p.Streaks[sk.Slug] = st.Streak
	}

	unlocks, err := s.store.ListUnlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list unlocks: %w", err)
	}
	p.FightersTotal = len(Fighters)
	for _, u := range unlocks {
		switch u.Kind {
		case UnlockFighter:
			p.FightersUnlocked++
		case UnlockDragonBall:
			if n, err := strconv.Atoi(u.Ref); err == nil {
				p.DragonBalls = append(p.DragonBalls, n)
			}
		}
	}
	sort.Ints(p.DragonBalls)

	results, err := s.store.ListDailyResults(ctx, time.Now().AddDate(0, 0, -60).Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list daily results: %w", err)
	}
	p.DailyStreak = dailyStreakFrom(results)

	attempts, err := s.store.ListAttempts(ctx, time.Now().AddDate(0, 0, -30))
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}
	p.Days = dayCounts(attempts)

	return p, nil
}

// ---- collection ----

// CollectionFighter is one card in GET /api/collection's fighter grid.
type CollectionFighter struct {
	Slug       string     `json:"slug"`
	Name       string     `json:"name"`
	Rarity     string     `json:"rarity"`
	Unlocked   bool       `json:"unlocked"`
	Hint       string     `json:"hint"`
	UnlockedAt *time.Time `json:"unlocked_at,omitempty"`
}

// Collection is the response shape for GET /api/collection.
type Collection struct {
	Fighters    []CollectionFighter `json:"fighters"`
	DragonBalls []int               `json:"dragon_balls"`
	ReadyToWish bool                `json:"ready_to_wish"`
}

func (s *Service) Collection(ctx context.Context) (*Collection, error) {
	unlocks, err := s.store.ListUnlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list unlocks: %w", err)
	}
	byKey := make(map[string]Unlock, len(unlocks))
	for _, u := range unlocks {
		byKey[u.Kind+":"+u.Ref] = u
	}

	c := &Collection{}
	for _, f := range Fighters {
		cf := CollectionFighter{Slug: f.Slug, Name: f.Name, Rarity: string(f.Rarity), Hint: unlockHint(f)}
		if u, ok := byKey[UnlockFighter+":"+f.Slug]; ok {
			cf.Unlocked = true
			t := u.CreatedAt
			cf.UnlockedAt = &t
		}
		c.Fighters = append(c.Fighters, cf)
	}

	for _, u := range unlocks {
		if u.Kind == UnlockDragonBall {
			if n, err := strconv.Atoi(u.Ref); err == nil {
				c.DragonBalls = append(c.DragonBalls, n)
			}
		}
	}
	sort.Ints(c.DragonBalls)
	c.ReadyToWish = len(c.DragonBalls) == 7

	return c, nil
}

func unlockHint(f Fighter) string {
	switch f.Condition.Type {
	case "power_level":
		return fmt.Sprintf("Reach power level %d", f.Condition.PowerLevel)
	case "saga":
		return fmt.Sprintf("Complete the %s saga", f.Condition.Saga)
	case "wish_only":
		return "Summon Shenron with all 7 dragon balls"
	default:
		return ""
	}
}

// ---- quests ----

// QuestChapterView adds saga-position lock state to a QuestChapter.
type QuestChapterView struct {
	QuestChapter
	Locked bool `json:"locked"`
}

// QuestSaga groups a saga's chapters with its own lock state.
type QuestSaga struct {
	Saga     string             `json:"saga"`
	Locked   bool               `json:"locked"`
	Chapters []QuestChapterView `json:"chapters"`
}

// SagaOrder is the fixed story order sagas unlock in (ARCHITECTURE.md
// "Quests": "5 sagas x 4 chapters" — saiyan, namek, android, cell, buu).
// The DB has no ordinal column for this (quest_chapters is keyed by
// (saga, chapter), which sorts alphabetically), so this table is
// authoritative for saga sequencing.
var SagaOrder = []string{"saiyan", "namek", "android", "cell", "buu"}

func (s *Service) Quests(ctx context.Context) ([]QuestSaga, error) {
	chapters, err := s.store.ListQuestChapters(ctx)
	if err != nil {
		return nil, fmt.Errorf("list quest chapters: %w", err)
	}

	bySaga := map[string][]QuestChapter{}
	for _, ch := range chapters {
		bySaga[ch.Saga] = append(bySaga[ch.Saga], ch)
	}

	var out []QuestSaga
	sagaUnlocked := true
	for _, saga := range SagaOrder {
		if _, ok := bySaga[saga]; !ok {
			continue
		}
		chs := bySaga[saga]
		sort.Slice(chs, func(i, j int) bool { return chs[i].Chapter < chs[j].Chapter })

		view := QuestSaga{Saga: saga, Locked: !sagaUnlocked}
		prevDone := sagaUnlocked
		for _, ch := range chs {
			locked := !sagaUnlocked || !prevDone
			view.Chapters = append(view.Chapters, QuestChapterView{QuestChapter: ch, Locked: locked})
			prevDone = ch.CompletedAt != nil
		}
		out = append(out, view)
		sagaUnlocked = prevDone
	}
	return out, nil
}

// QuestChapterByID looks up one chapter (with its lock state) by DB id, for
// GET /api/quests/{id}. Returns nil, nil if not found.
func (s *Service) QuestChapterByID(ctx context.Context, id int64) (*QuestChapterView, error) {
	sagas, err := s.Quests(ctx)
	if err != nil {
		return nil, err
	}
	for _, saga := range sagas {
		for _, ch := range saga.Chapters {
			if ch.ID == id {
				cp := ch
				return &cp, nil
			}
		}
	}
	return nil, nil
}
