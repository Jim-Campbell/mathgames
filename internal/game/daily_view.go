package game

import (
	"context"
	"fmt"
	"time"
)

// aiSkillSlugs is the pool SeedDaily picks the 5th (AI) slot from.
var aiSkillSlugs = func() []string {
	var out []string
	for _, sk := range Skills {
		if sk.Source == SourceAI {
			out = append(out, sk.Slug)
		}
	}
	return out
}()

// CalendarDay is one entry in the daily-challenge calendar view.
type CalendarDay struct {
	Day       string `json:"day"`
	Completed bool   `json:"completed"`
	Correct   int    `json:"correct"`
	Total     int    `json:"total"`
}

// DailyView is the response shape for GET /api/daily.
type DailyView struct {
	Day       string        `json:"day"`
	Questions []Question    `json:"questions,omitempty"` // only while incomplete; stripping answers is an HTTP-layer concern
	Answered  int           `json:"answered"`
	Correct   int           `json:"correct"`
	ElapsedMS int           `json:"elapsed_ms"`
	XPEarned  int           `json:"xp_earned"`
	Completed bool          `json:"completed"`
	Streak    int           `json:"streak"`
	Calendar  []CalendarDay `json:"calendar"`
}

// Daily fetches (creating + pinning on first fetch) the day's challenge set
// per ARCHITECTURE.md "Collection, quests, daily".
func (s *Service) Daily(ctx context.Context, day string) (*DailyView, error) {
	res, err := s.store.GetDailyResult(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("get daily result: %w", err)
	}
	if res == nil {
		res, err = s.seedDailyResult(ctx, day)
		if err != nil {
			return nil, err
		}
	}

	view := &DailyView{
		Day:       res.Day,
		Answered:  res.Answered,
		Correct:   res.Correct,
		ElapsedMS: res.ElapsedMS,
		XPEarned:  res.XPEarned,
		Completed: res.CompletedAt != nil,
	}
	if !view.Completed {
		for _, id := range res.QuestionIDs {
			q, err := s.store.GetQuestion(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("get daily question %d: %w", id, err)
			}
			if q != nil {
				view.Questions = append(view.Questions, *q)
			}
		}
	}

	results, err := s.store.ListDailyResults(ctx, time.Now().AddDate(0, 0, -60).Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list daily results: %w", err)
	}
	view.Streak = dailyStreakFrom(results)

	calSince := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	calResults, err := s.store.ListDailyResults(ctx, calSince)
	if err != nil {
		return nil, fmt.Errorf("list calendar results: %w", err)
	}
	for _, r := range calResults {
		view.Calendar = append(view.Calendar, CalendarDay{
			Day: r.Day, Completed: r.CompletedAt != nil, Correct: r.Correct, Total: len(r.QuestionIDs),
		})
	}

	return view, nil
}

// seedDailyResult deterministically picks and generates the day's question
// set (FNV-1a(day) seeded, per daily.go) and pins it in daily_results on
// first fetch.
func (s *Service) seedDailyResult(ctx context.Context, day string) (*DailyResult, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	levels := map[string]int{}
	for _, sk := range Skills {
		lvl, err := s.effectiveLevel(ctx, sk.Slug)
		if err != nil {
			return nil, err
		}
		levels[sk.Slug] = lvl
	}

	picks := SeedDaily(day, levels, aiSkillSlugs, settings.DailyCount)

	rng := newRand()
	var ids []int64
	for _, pick := range picks {
		qs, _, err := s.serveSkill(ctx, pick.Skill, 1, rng)
		if err != nil {
			return nil, fmt.Errorf("seed daily question for %s: %w", pick.Skill, err)
		}
		for _, q := range qs {
			ids = append(ids, q.ID)
		}
	}

	res := &DailyResult{Day: day, QuestionIDs: ids}
	if err := s.store.CreateDailyResult(ctx, res); err != nil {
		return nil, fmt.Errorf("create daily result: %w", err)
	}
	return res, nil
}
