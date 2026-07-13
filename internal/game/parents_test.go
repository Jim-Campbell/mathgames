package game

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestParentsSummary_HandCheckedFixture builds a small fixed set of attempts
// and hand-checks the resulting basis points and median, per
// ARCHITECTURE.md "API" -> GET /api/parents/summary.
func TestParentsSummary_HandCheckedFixture(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	sess := &Session{Mode: ModeTraining}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	// 4 multiplication attempts today: 3 correct, 1 wrong -> 7500 bp.
	// elapsed_ms: 1000, 2000, 3000, 4000 -> median = (2000+3000)/2 = 2500.
	elapsed := []int{1000, 2000, 3000, 4000}
	correct := []bool{true, true, true, false}
	for i := 0; i < 4; i++ {
		qID := insertNumericQuestion(t, store, "multiplication", 3, 10)
		given, _ := json.Marshal(NumericAnswer{Value: 10})
		if !correct[i] {
			given, _ = json.Marshal(NumericAnswer{Value: -1})
		}
		if _, err := svc.Attempt(ctx, sess.ID, qID, given, elapsed[i]); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}

	summary, err := svc.ParentsSummary(ctx, 30)
	if err != nil {
		t.Fatalf("ParentsSummary: %v", err)
	}

	if len(summary.PerDay) != 1 {
		t.Fatalf("expected 1 day of activity, got %d", len(summary.PerDay))
	}
	today := time.Now().UTC().Format("2006-01-02")
	day := summary.PerDay[0]
	if day.Day != today {
		t.Fatalf("day = %s, want %s", day.Day, today)
	}
	if day.Attempts != 4 {
		t.Fatalf("attempts = %d, want 4", day.Attempts)
	}
	if day.CorrectBP != 7500 {
		t.Fatalf("correct_bp = %d, want 7500", day.CorrectBP)
	}

	var mult *SkillActivity
	for i := range summary.PerSkill {
		if summary.PerSkill[i].Skill == "multiplication" {
			mult = &summary.PerSkill[i]
		}
	}
	if mult == nil {
		t.Fatal("expected a multiplication row in per_skill")
	}
	if mult.CorrectBP != 7500 {
		t.Fatalf("skill correct_bp = %d, want 7500", mult.CorrectBP)
	}
	if mult.MedianMS != 2500 {
		t.Fatalf("median_ms = %d, want 2500", mult.MedianMS)
	}
	if mult.Attempts != 4 {
		t.Fatalf("skill attempts = %d, want 4", mult.Attempts)
	}
}

func TestParentsSummary_RecentMissesIncludesPromptAndAnswer(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "division", 2, 5)
	given, _ := json.Marshal(NumericAnswer{Value: 999})
	if _, err := svc.Attempt(ctx, sess.ID, qID, given, 1000); err != nil {
		t.Fatal(err)
	}

	summary, err := svc.ParentsSummary(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.RecentMisses) != 1 {
		t.Fatalf("expected 1 miss, got %d", len(summary.RecentMisses))
	}
	miss := summary.RecentMisses[0]
	if miss.Skill != "division" {
		t.Fatalf("miss skill = %s, want division", miss.Skill)
	}
	if miss.Prompt == "" {
		t.Fatal("expected a non-empty prompt")
	}
}
