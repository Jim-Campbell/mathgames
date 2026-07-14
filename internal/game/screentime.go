package game

import (
	"context"
	"fmt"
	"time"
)

// screenTimeCapMinutes is the maximum the dial can show; earning pauses once
// reached, until a parent resets it. See ARCHITECTURE.md "Screen time".
const screenTimeCapMinutes = 60

// ScreenTime is the current dial state, derived from attempts rows -- never
// stored, so there's nothing to drift.
type ScreenTime struct {
	MinutesEarned      int        `json:"minutes_earned"` // capped at MinutesCap
	MinutesCap         int        `json:"minutes_cap"`
	CorrectsSinceReset int        `json:"corrects_since_reset"`
	MinutesPerCorrect  int        `json:"minutes_per_correct"`
	Full               bool       `json:"full"`
	SinceReset         *time.Time `json:"since_reset"` // nil = never reset
}

// ScreenTime computes the current dial: corrects since the last reset (or
// all-time, if never reset) times settings.minutes_per_correct, capped at
// screenTimeCapMinutes. A rate change picked up mid-period applies
// retroactively to the whole current period -- that's intentional, since the
// dial is derived and there's no per-attempt minute value stored anywhere.
func (s *Service) ScreenTime(ctx context.Context) (*ScreenTime, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	last, err := s.store.LastScreenTimeReset(ctx)
	if err != nil {
		return nil, fmt.Errorf("last screen time reset: %w", err)
	}
	var since *time.Time
	if last != nil {
		since = &last.ResetAt
	}

	corrects, err := s.store.CountCorrectsSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("count corrects since: %w", err)
	}

	minutes := corrects * settings.MinutesPerCorrect
	full := minutes >= screenTimeCapMinutes
	if full {
		minutes = screenTimeCapMinutes
	}

	return &ScreenTime{
		MinutesEarned:      minutes,
		MinutesCap:         screenTimeCapMinutes,
		CorrectsSinceReset: corrects,
		MinutesPerCorrect:  settings.MinutesPerCorrect,
		Full:               full,
		SinceReset:         since,
	}, nil
}

// ResetScreenTime snapshots the current dial into a screen_time_resets row
// (the parent's redemption event) and returns it. Rejected when the dial is
// at 0 -- nothing to redeem.
func (s *Service) ResetScreenTime(ctx context.Context) (*ScreenTimeReset, error) {
	current, err := s.ScreenTime(ctx)
	if err != nil {
		return nil, err
	}
	if current.MinutesEarned == 0 {
		return nil, fmt.Errorf("invalid: nothing to redeem")
	}

	r := &ScreenTimeReset{
		MinutesRedeemed: current.MinutesEarned,
		CorrectsCounted: current.CorrectsSinceReset,
	}
	if err := s.store.InsertScreenTimeReset(ctx, r); err != nil {
		return nil, fmt.Errorf("insert screen time reset: %w", err)
	}
	return r, nil
}

// ListScreenTimeLog returns every reset, newest first, for the parents log
// subpage.
func (s *Service) ListScreenTimeLog(ctx context.Context) ([]ScreenTimeReset, error) {
	resets, err := s.store.ListScreenTimeResets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list screen time resets: %w", err)
	}
	return resets, nil
}
