package game

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// DayActivity is one row of ParentsSummary.PerDay.
type DayActivity struct {
	Day       string `json:"day"`
	Attempts  int    `json:"attempts"`
	CorrectBP int    `json:"correct_bp"` // basis points, 8750 = 87.50%
	Minutes   int    `json:"minutes"`
	XP        int64  `json:"xp"`
}

// SkillActivity is one row of ParentsSummary.PerSkill.
type SkillActivity struct {
	Skill     string `json:"skill"`
	Level     int    `json:"level"`
	Attempts  int    `json:"attempts"`
	CorrectBP int    `json:"correct_bp"`
	MedianMS  int    `json:"median_ms"`
	Trend     int    `json:"trend"` // -1/0/+1 vs the prior 7 days
}

// Miss is one recent wrong attempt, for the "what should we practice" list.
type Miss struct {
	Prompt string          `json:"prompt"`
	Given  json.RawMessage `json:"given"`
	Answer json.RawMessage `json:"answer"`
	Skill  string          `json:"skill"`
	Day    string          `json:"day"`
}

// BankStatus is the AI question bank count for one skill x level.
type BankStatus struct {
	Skill     string `json:"skill"`
	Level     int    `json:"level"`
	Available int    `json:"available"`
}

// ParentsSummary is the response shape for GET /api/parents/summary.
type ParentsSummary struct {
	PerDay       []DayActivity   `json:"per_day"`
	PerSkill     []SkillActivity `json:"per_skill"`
	RecentMisses []Miss          `json:"recent_misses"`
	Bank         []BankStatus    `json:"bank"`
}

// ParentsSummary aggregates activity over the last `days` days per
// ARCHITECTURE.md "API" -> GET /api/parents/summary. Minutes use the
// documented elapsed_ms-sum fallback uniformly (simpler and always
// available, since a kid closing the tab mid-session leaves sessions
// without a reliable ended_at).
func (s *Service) ParentsSummary(ctx context.Context, days int) (*ParentsSummary, error) {
	if days <= 0 {
		days = 30
	}
	// Fetch far enough back to cover both the requested window and the
	// 14-day trend comparison (last 7 days vs the 7 before that).
	lookback := days
	if lookback < 14 {
		lookback = 14
	}
	since := time.Now().AddDate(0, 0, -lookback)

	attempts, err := s.store.ListAttempts(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}

	windowStart := time.Now().AddDate(0, 0, -days)
	var windowed []Attempt
	for _, a := range attempts {
		if !a.CreatedAt.Before(windowStart) {
			windowed = append(windowed, a)
		}
	}

	summary := &ParentsSummary{
		PerDay:   buildPerDay(windowed),
		PerSkill: buildPerSkill(attempts, days),
	}

	summary.RecentMisses, err = s.recentMisses(ctx, windowed, 20)
	if err != nil {
		return nil, err
	}

	summary.Bank, err = s.bankStatus(ctx)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func buildPerDay(attempts []Attempt) []DayActivity {
	type acc struct {
		total, correct int
		elapsedMS      int
		xp             int64
	}
	byDay := map[string]*acc{}
	for _, a := range attempts {
		day := a.CreatedAt.UTC().Format("2006-01-02")
		d, ok := byDay[day]
		if !ok {
			d = &acc{}
			byDay[day] = d
		}
		d.total++
		if a.Correct {
			d.correct++
		}
		d.elapsedMS += a.ElapsedMS
		d.xp += int64(a.XPEarned)
	}

	var out []DayActivity
	for day, d := range byDay {
		out = append(out, DayActivity{
			Day:       day,
			Attempts:  d.total,
			CorrectBP: d.correct * 10000 / d.total,
			Minutes:   d.elapsedMS / 60000,
			XP:        d.xp,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}

func buildPerSkill(attempts []Attempt, days int) []SkillActivity {
	windowStart := time.Now().AddDate(0, 0, -days)
	last7 := time.Now().AddDate(0, 0, -7)
	prior7 := time.Now().AddDate(0, 0, -14)

	type bucket struct {
		total, correct   int
		last7T, last7C   int
		prior7T, prior7C int
		elapsed          []int
		level            int
	}
	bySkill := map[string]*bucket{}
	for _, a := range attempts {
		b, ok := bySkill[a.Skill]
		if !ok {
			b = &bucket{}
			bySkill[a.Skill] = b
		}
		b.level = a.Difficulty // last-seen difficulty as a proxy for current level

		if !a.CreatedAt.Before(windowStart) {
			b.total++
			if a.Correct {
				b.correct++
			}
			b.elapsed = append(b.elapsed, a.ElapsedMS)
		}
		if !a.CreatedAt.Before(last7) {
			b.last7T++
			if a.Correct {
				b.last7C++
			}
		} else if !a.CreatedAt.Before(prior7) {
			b.prior7T++
			if a.Correct {
				b.prior7C++
			}
		}
	}

	var out []SkillActivity
	for skill, b := range bySkill {
		sa := SkillActivity{Skill: skill, Level: b.level, Attempts: b.total}
		if b.total > 0 {
			sa.CorrectBP = b.correct * 10000 / b.total
		}
		sa.MedianMS = medianInt(b.elapsed)

		last7BP, prior7BP := -1, -1
		if b.last7T > 0 {
			last7BP = b.last7C * 10000 / b.last7T
		}
		if b.prior7T > 0 {
			prior7BP = b.prior7C * 10000 / b.prior7T
		}
		switch {
		case last7BP < 0 || prior7BP < 0:
			sa.Trend = 0
		case last7BP > prior7BP:
			sa.Trend = 1
		case last7BP < prior7BP:
			sa.Trend = -1
		}
		out = append(out, sa)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Skill < out[j].Skill })
	return out
}

func medianInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]int{}, vals...)
	sort.Ints(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func (s *Service) recentMisses(ctx context.Context, attempts []Attempt, limit int) ([]Miss, error) {
	var wrong []Attempt
	for _, a := range attempts {
		if !a.Correct {
			wrong = append(wrong, a)
		}
	}
	sort.Slice(wrong, func(i, j int) bool { return wrong[i].CreatedAt.After(wrong[j].CreatedAt) })
	if len(wrong) > limit {
		wrong = wrong[:limit]
	}

	var out []Miss
	for _, a := range wrong {
		q, err := s.store.GetQuestion(ctx, a.QuestionID)
		if err != nil {
			return nil, fmt.Errorf("get question %d for miss: %w", a.QuestionID, err)
		}
		if q == nil {
			continue
		}
		var payload Payload
		if err := json.Unmarshal(q.Payload, &payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload for miss: %w", err)
		}
		out = append(out, Miss{
			Prompt: payload.Prompt,
			Given:  a.Given,
			Answer: q.Answer,
			Skill:  a.Skill,
			Day:    a.CreatedAt.UTC().Format("2006-01-02"),
		})
	}
	return out, nil
}

func (s *Service) bankStatus(ctx context.Context) ([]BankStatus, error) {
	falseVal := false
	questions, err := s.store.ListQuestions(ctx, "", string(SourceAI), &falseVal)
	if err != nil {
		return nil, fmt.Errorf("list ai questions: %w", err)
	}
	counts := map[string]int{}
	for _, q := range questions {
		counts[fmt.Sprintf("%s|%d", q.Skill, q.Difficulty)]++
	}

	var out []BankStatus
	for _, sk := range Skills {
		if sk.Source != SourceAI {
			continue
		}
		for lvl := 1; lvl <= 10; lvl++ {
			key := fmt.Sprintf("%s|%d", sk.Slug, lvl)
			out = append(out, BankStatus{Skill: sk.Slug, Level: lvl, Available: counts[key]})
		}
	}
	return out, nil
}
