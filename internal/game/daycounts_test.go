package game

import (
	"testing"
	"time"
)

func TestDayCounts(t *testing.T) {
	mk := func(day string, correct bool) Attempt {
		d, _ := time.Parse("2006-01-02T15:04:05Z", day)
		return Attempt{CreatedAt: d, Correct: correct}
	}
	attempts := []Attempt{
		mk("2026-07-12T09:00:00Z", true),
		mk("2026-07-12T09:05:00Z", false),
		mk("2026-07-12T20:00:00Z", true),
		mk("2026-07-14T08:00:00Z", false),
	}

	got := dayCounts(attempts)
	if len(got) != 2 {
		t.Fatalf("got %d days, want 2 (13th has no activity and is omitted)", len(got))
	}
	if got[0].Day != "2026-07-12" || got[0].Correct != 2 || got[0].Wrong != 1 {
		t.Errorf("day 0 = %+v, want 2026-07-12 correct=2 wrong=1", got[0])
	}
	if got[1].Day != "2026-07-14" || got[1].Correct != 0 || got[1].Wrong != 1 {
		t.Errorf("day 1 = %+v, want 2026-07-14 correct=0 wrong=1", got[1])
	}
}

func TestDayCounts_EmptyIsNonNil(t *testing.T) {
	got := dayCounts(nil)
	if got == nil {
		t.Fatal("dayCounts(nil) returned nil; want empty non-nil slice for clean JSON")
	}
	if len(got) != 0 {
		t.Fatalf("got %d days, want 0", len(got))
	}
}
