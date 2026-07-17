package game

import (
	"context"
	"testing"
	"time"
)

func addCorrectAttempts(store *fakeStore, n int) {
	for i := 0; i < n; i++ {
		store.attempts = append(store.attempts, Attempt{Correct: true, CreatedAt: time.Now()})
	}
}

const (
	monday  = "2026-07-13"
	tuesday = "2026-07-14"
)

// TestScreenTime_WorkedExamples hand-checks ARCHITECTURE.md's screen-time
// worked examples: 7 corrects x 3 = 21 min; 25 corrects x 3 = 75 -> capped
// 60, Full: true; after a reset, back to 0.
func TestScreenTime_WorkedExamples(t *testing.T) {
	ctx := context.Background()
	svc, store := testService()

	addCorrectAttempts(store, 7)
	st, err := svc.ScreenTime(ctx, monday)
	if err != nil {
		t.Fatalf("ScreenTime: %v", err)
	}
	if st.MinutesEarned != 21 || st.Full {
		t.Fatalf("7 corrects x 3/correct: got %d min (full=%v), want 21 min, not full", st.MinutesEarned, st.Full)
	}

	// Add corrects up to a total of 25 (18 more) -> 75 min raw, capped 60.
	addCorrectAttempts(store, 18)
	st, err = svc.ScreenTime(ctx, monday)
	if err != nil {
		t.Fatalf("ScreenTime: %v", err)
	}
	if st.MinutesEarned != 60 || !st.Full {
		t.Fatalf("25 corrects x 3/correct capped: got %d min (full=%v), want 60 min, full", st.MinutesEarned, st.Full)
	}
	if st.CorrectsSinceReset != 25 {
		t.Fatalf("corrects since reset: got %d, want 25", st.CorrectsSinceReset)
	}

	reset, err := svc.ResetScreenTime(ctx, monday)
	if err != nil {
		t.Fatalf("ResetScreenTime: %v", err)
	}
	if reset.MinutesRedeemed != 60 || reset.CorrectsCounted != 25 {
		t.Fatalf("reset row: got minutes=%d corrects=%d, want 60/25", reset.MinutesRedeemed, reset.CorrectsCounted)
	}

	st, err = svc.ScreenTime(ctx, monday)
	if err != nil {
		t.Fatalf("ScreenTime after reset: %v", err)
	}
	if st.MinutesEarned != 0 || st.Full {
		t.Fatalf("after reset: got %d min (full=%v), want 0, not full", st.MinutesEarned, st.Full)
	}
}

// TestScreenTime_ResetAtZeroRejected: nothing to redeem at 0 minutes.
func TestScreenTime_ResetAtZeroRejected(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService()

	_, err := svc.ResetScreenTime(ctx, monday)
	if err == nil {
		t.Fatal("expected error resetting at 0 minutes, got nil")
	}
}

// TestScreenTime_WrongAnswersDontCount: only corrects move the dial.
func TestScreenTime_WrongAnswersDontCount(t *testing.T) {
	ctx := context.Background()
	svc, store := testService()

	now := time.Now()
	store.attempts = append(store.attempts,
		Attempt{Correct: true, CreatedAt: now},
		Attempt{Correct: false, CreatedAt: now},
		Attempt{Correct: false, CreatedAt: now},
	)

	st, err := svc.ScreenTime(ctx, monday)
	if err != nil {
		t.Fatalf("ScreenTime: %v", err)
	}
	if st.MinutesEarned != 3 || st.CorrectsSinceReset != 1 {
		t.Fatalf("got %d min, %d corrects; want 3 min, 1 correct", st.MinutesEarned, st.CorrectsSinceReset)
	}
}

// TestScreenTime_RateChangeAppliesToCurrentPeriod: the dial is derived, so a
// mid-period settings.minutes_per_correct edit is picked up immediately for
// the whole current period -- intended, not a bug.
func TestScreenTime_RateChangeAppliesToCurrentPeriod(t *testing.T) {
	ctx := context.Background()
	svc, store := testService()

	addCorrectAttempts(store, 5)
	st, _ := svc.ScreenTime(ctx, monday)
	if st.MinutesEarned != 15 {
		t.Fatalf("before rate change: got %d min, want 15", st.MinutesEarned)
	}

	settings, _ := store.GetSettings(ctx)
	settings.MinutesPerCorrect = 5
	store.UpdateSettings(ctx, settings)

	st, _ = svc.ScreenTime(ctx, monday)
	if st.MinutesEarned != 25 {
		t.Fatalf("after rate change: got %d min, want 25 (5 corrects x 5/correct)", st.MinutesEarned)
	}
}

// TestScreenTime_DailyRollover hand-checks the Monday->Tuesday worked
// example from the feature spec: 10 corrects at rate 3 -> dial 30 on
// Monday. Calling ScreenTime(day=Tuesday) rolls the day over: it inserts a
// daily reset row snapshotting 30, and the dial reads 0. A second call the
// same day is a no-op (idempotent). New corrects on Tuesday accrue normally
// from 0.
func TestScreenTime_DailyRollover(t *testing.T) {
	ctx := context.Background()
	svc, store := testService()

	addCorrectAttempts(store, 10)
	st, err := svc.ScreenTime(ctx, monday)
	if err != nil {
		t.Fatalf("ScreenTime monday: %v", err)
	}
	if st.MinutesEarned != 30 {
		t.Fatalf("monday: got %d min, want 30", st.MinutesEarned)
	}

	st, err = svc.ScreenTime(ctx, tuesday)
	if err != nil {
		t.Fatalf("ScreenTime tuesday: %v", err)
	}
	if st.MinutesEarned != 0 {
		t.Fatalf("tuesday after rollover: got %d min, want 0", st.MinutesEarned)
	}
	// One bootstrap row (from the monday call, tracking that day without
	// touching the dial) plus the tuesday rollover row.
	if len(store.stResets) != 2 {
		t.Fatalf("expected 2 reset rows (bootstrap + tuesday rollover), got %d", len(store.stResets))
	}
	var daily *ScreenTimeReset
	for i := range store.stResets {
		if store.stResets[i].Day != nil && *store.stResets[i].Day == tuesday {
			daily = &store.stResets[i]
		}
	}
	if daily == nil {
		t.Fatal("expected a reset row for tuesday")
	}
	if daily.Reason != "daily" || daily.MinutesRedeemed != 30 || daily.CorrectsCounted != 10 {
		t.Fatalf("daily reset row: got reason=%s minutes=%d corrects=%d, want daily/30/10",
			daily.Reason, daily.MinutesRedeemed, daily.CorrectsCounted)
	}

	// Idempotent: calling again the same day inserts no second row.
	st, err = svc.ScreenTime(ctx, tuesday)
	if err != nil {
		t.Fatalf("ScreenTime tuesday again: %v", err)
	}
	if st.MinutesEarned != 0 {
		t.Fatalf("tuesday again: got %d min, want 0", st.MinutesEarned)
	}
	if len(store.stResets) != 2 {
		t.Fatalf("expected still 2 reset rows, got %d", len(store.stResets))
	}

	// Same-day corrects after rollover accrue normally.
	addCorrectAttempts(store, 3)
	st, err = svc.ScreenTime(ctx, tuesday)
	if err != nil {
		t.Fatalf("ScreenTime tuesday after corrects: %v", err)
	}
	if st.MinutesEarned != 9 {
		t.Fatalf("tuesday after 3 more corrects: got %d min, want 9", st.MinutesEarned)
	}
}

// TestScreenTime_ManualResetStillWorks: the parent reset path is unchanged
// and records reason='manual'.
func TestScreenTime_ManualResetStillWorks(t *testing.T) {
	ctx := context.Background()
	svc, store := testService()

	addCorrectAttempts(store, 4)
	reset, err := svc.ResetScreenTime(ctx, monday)
	if err != nil {
		t.Fatalf("ResetScreenTime: %v", err)
	}
	if reset.Reason != "manual" {
		t.Fatalf("got reason %q, want manual", reset.Reason)
	}
	if reset.Day == nil || *reset.Day != monday {
		t.Fatalf("got day %v, want %s", reset.Day, monday)
	}
}
