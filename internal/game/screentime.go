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

// computeScreenTime does the actual dial math: corrects since the last
// reset (or all-time, if never reset) times settings.minutes_per_correct,
// capped at screenTimeCapMinutes. A rate change picked up mid-period applies
// retroactively to the whole current period -- that's intentional, since the
// dial is derived and there's no per-attempt minute value stored anywhere.
// It does not roll over the day -- callers that need that call
// EnsureDailyReset first.
func (s *Service) computeScreenTime(ctx context.Context) (*ScreenTime, error) {
	settings, err := s.store.GetSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	last, err := s.store.LastScreenTimeReset(ctx)
	if err != nil {
		return nil, fmt.Errorf("last screen time reset: %w", err)
	}
	// A bootstrap marker's resetAt is a sentinel, not a real reset moment --
	// report it as "never reset" (nil), same as before this row existed.
	var since *time.Time
	if last != nil && !last.ResetAt.Equal(screenTimeEpoch) {
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

// screenTimeEpoch backdates a bootstrap daily marker (see EnsureDailyReset)
// so it never excludes attempts that already exist -- it only exists to
// record "localDay is the tracked day" for future rollover comparisons.
var screenTimeEpoch = time.Unix(0, 0)

// EnsureDailyReset rolls the dial over for a new device-local day: if the
// most recent reset predates localDay (or has no day recorded -- a legacy
// manual reset, or no reset has ever happened), it snapshots the current
// dial into a reason='daily' reset row for localDay, so the dial reads 0 for
// the rest of that day. No-op if a reset (of either reason) already covers
// localDay. Race-safe: InsertDailyResetIfNew is the only writer and is
// idempotent per day.
func (s *Service) EnsureDailyReset(ctx context.Context, localDay string) error {
	last, err := s.store.LastScreenTimeReset(ctx)
	if err != nil {
		return fmt.Errorf("last screen time reset: %w", err)
	}

	if last == nil {
		// Nothing has ever reset the dial. There's no prior period to
		// snapshot -- just start tracking localDay from the epoch so it
		// doesn't touch the all-time count, and a later day can detect the
		// rollover.
		if _, err := s.store.InsertDailyResetIfNew(ctx, localDay, screenTimeEpoch, 0, 0); err != nil {
			return fmt.Errorf("insert bootstrap daily reset: %w", err)
		}
		return nil
	}
	if last.Day != nil && *last.Day >= localDay {
		return nil
	}

	current, err := s.computeScreenTime(ctx)
	if err != nil {
		return err
	}
	if _, err := s.store.InsertDailyResetIfNew(ctx, localDay, time.Now(), current.MinutesEarned, current.CorrectsSinceReset); err != nil {
		return fmt.Errorf("insert daily reset: %w", err)
	}
	return nil
}

// ScreenTime returns the current dial for localDay, rolling the day over
// first if this is the first use of the app on a new device-local day.
func (s *Service) ScreenTime(ctx context.Context, localDay string) (*ScreenTime, error) {
	if err := s.EnsureDailyReset(ctx, localDay); err != nil {
		return nil, err
	}
	return s.computeScreenTime(ctx)
}

// ResetScreenTime snapshots the current dial into a screen_time_resets row
// (the parent's redemption event) and returns it. Rejected when the dial is
// at 0 -- nothing to redeem.
func (s *Service) ResetScreenTime(ctx context.Context, localDay string) (*ScreenTimeReset, error) {
	current, err := s.computeScreenTime(ctx)
	if err != nil {
		return nil, err
	}
	if current.MinutesEarned == 0 {
		return nil, fmt.Errorf("invalid: nothing to redeem")
	}

	r := &ScreenTimeReset{
		MinutesRedeemed: current.MinutesEarned,
		CorrectsCounted: current.CorrectsSinceReset,
		Reason:          "manual",
		Day:             &localDay,
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
