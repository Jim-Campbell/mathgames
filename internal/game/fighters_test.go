package game

import "testing"

func unlockKey(kind, ref string) string { return kind + ":" + ref }

func TestDetectUnlocks_ThresholdCrossedOnce(t *testing.T) {
	already := map[string]bool{}

	first := DetectUnlocks(0, 600, 0, nil, already)
	if len(first) != 1 || first[0].Slug != "krillin" {
		t.Fatalf("expected exactly [krillin] crossing 500, got %+v", first)
	}

	// Mark it unlocked (as the service would after persisting), then call
	// again with power still above the threshold — must not re-report it.
	for _, f := range first {
		already[unlockKey(UnlockFighter, f.Slug)] = true
	}
	second := DetectUnlocks(600, 700, 0, nil, already)
	if len(second) != 0 {
		t.Fatalf("threshold already crossed should not be reported again, got %+v", second)
	}
}

func TestDetectUnlocks_SingleJumpCrossesMultipleThresholds(t *testing.T) {
	already := map[string]bool{}
	got := DetectUnlocks(400, 3000, 0, nil, already)

	wantSlugs := map[string]bool{"krillin": false, "yamcha": false}
	for _, f := range got {
		if _, ok := wantSlugs[f.Slug]; ok {
			wantSlugs[f.Slug] = true
		}
	}
	for slug, found := range wantSlugs {
		if !found {
			t.Errorf("expected %q to unlock when power jumps 400 -> 3000, got %+v", slug, got)
		}
	}
}

func TestDetectUnlocks_StreakBadges(t *testing.T) {
	already := map[string]bool{}
	got := DetectUnlocks(0, 0, 3, nil, already)

	found := false
	for _, f := range got {
		if f.Slug == "streak-3" {
			found = true
		}
		if f.Slug == "streak-7" {
			t.Errorf("streak 3 should not unlock the 7-day badge: %+v", got)
		}
	}
	if !found {
		t.Fatalf("expected streak-3 badge unlock, got %+v", got)
	}
}

func TestDetectUnlocks_SagaCompletion(t *testing.T) {
	already := map[string]bool{}
	completions := map[string]bool{"namek": true}
	got := DetectUnlocks(0, 0, 0, completions, already)

	found := false
	for _, f := range got {
		if f.Slug == "frieza" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected frieza to unlock on namek saga completion, got %+v", got)
	}
}

func TestDetectUnlocks_WishOnlyNeverAutoUnlocks(t *testing.T) {
	already := map[string]bool{}
	// Push power absurdly high and mark every saga complete; shenron
	// (wish_only) must never appear.
	completions := map[string]bool{}
	for _, f := range Fighters {
		if f.Condition.Type == "saga" {
			completions[f.Condition.Saga] = true
		}
	}
	got := DetectUnlocks(0, 10_000_000, 999, completions, already)
	for _, f := range got {
		if f.Slug == "shenron" {
			t.Fatalf("shenron must never auto-unlock, got %+v", got)
		}
	}
}

func TestFighterBySlug(t *testing.T) {
	f, ok := FighterBySlug("goku")
	if !ok || f.Name != "Goku" {
		t.Fatalf("expected to find goku, got %+v ok=%v", f, ok)
	}
	if _, ok := FighterBySlug("does-not-exist"); ok {
		t.Fatal("expected not found for bogus slug")
	}
}
