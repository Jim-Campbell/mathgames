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

// TestScreenTime_WorkedExamples hand-checks ARCHITECTURE.md's screen-time
// worked examples: 7 corrects x 3 = 21 min; 25 corrects x 3 = 75 -> capped
// 60, Full: true; after a reset, back to 0.
func TestScreenTime_WorkedExamples(t *testing.T) {
	ctx := context.Background()
	svc, store := testService()

	addCorrectAttempts(store, 7)
	st, err := svc.ScreenTime(ctx)
	if err != nil {
		t.Fatalf("ScreenTime: %v", err)
	}
	if st.MinutesEarned != 21 || st.Full {
		t.Fatalf("7 corrects x 3/correct: got %d min (full=%v), want 21 min, not full", st.MinutesEarned, st.Full)
	}

	// Add corrects up to a total of 25 (18 more) -> 75 min raw, capped 60.
	addCorrectAttempts(store, 18)
	st, err = svc.ScreenTime(ctx)
	if err != nil {
		t.Fatalf("ScreenTime: %v", err)
	}
	if st.MinutesEarned != 60 || !st.Full {
		t.Fatalf("25 corrects x 3/correct capped: got %d min (full=%v), want 60 min, full", st.MinutesEarned, st.Full)
	}
	if st.CorrectsSinceReset != 25 {
		t.Fatalf("corrects since reset: got %d, want 25", st.CorrectsSinceReset)
	}

	reset, err := svc.ResetScreenTime(ctx)
	if err != nil {
		t.Fatalf("ResetScreenTime: %v", err)
	}
	if reset.MinutesRedeemed != 60 || reset.CorrectsCounted != 25 {
		t.Fatalf("reset row: got minutes=%d corrects=%d, want 60/25", reset.MinutesRedeemed, reset.CorrectsCounted)
	}

	st, err = svc.ScreenTime(ctx)
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

	_, err := svc.ResetScreenTime(ctx)
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

	st, err := svc.ScreenTime(ctx)
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
	st, _ := svc.ScreenTime(ctx)
	if st.MinutesEarned != 15 {
		t.Fatalf("before rate change: got %d min, want 15", st.MinutesEarned)
	}

	settings, _ := store.GetSettings(ctx)
	settings.MinutesPerCorrect = 5
	store.UpdateSettings(ctx, settings)

	st, _ = svc.ScreenTime(ctx)
	if st.MinutesEarned != 25 {
		t.Fatalf("after rate change: got %d min, want 25 (5 corrects x 5/correct)", st.MinutesEarned)
	}
}
