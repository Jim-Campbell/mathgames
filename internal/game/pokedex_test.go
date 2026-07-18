package game

import "testing"

func unlockKey(kind, ref string) string { return kind + ":" + ref }

func TestDetectUnlocks_ThresholdCrossedOnce(t *testing.T) {
	already := map[string]bool{}

	first := DetectUnlocks(0, 600, 0, nil, already)
	if len(first) != 1 || first[0].Slug != "pidgey" {
		t.Fatalf("expected exactly [pidgey] crossing 500, got %+v", first)
	}

	// Mark it unlocked (as the service would after persisting), then call
	// again with XP still above the threshold — must not re-report it.
	for _, p := range first {
		already[unlockKey(UnlockPokemon, p.Slug)] = true
	}
	second := DetectUnlocks(600, 700, 0, nil, already)
	if len(second) != 0 {
		t.Fatalf("threshold already crossed should not be reported again, got %+v", second)
	}
}

func TestDetectUnlocks_SingleJumpCrossesMultipleThresholds(t *testing.T) {
	already := map[string]bool{}
	got := DetectUnlocks(400, 3000, 0, nil, already)

	wantSlugs := map[string]bool{"pidgey": false, "rattata": false}
	for _, p := range got {
		if _, ok := wantSlugs[p.Slug]; ok {
			wantSlugs[p.Slug] = true
		}
	}
	for slug, found := range wantSlugs {
		if !found {
			t.Errorf("expected %q to unlock when xp jumps 400 -> 3000, got %+v", slug, got)
		}
	}
}

func TestDetectUnlocks_StreakRibbons(t *testing.T) {
	already := map[string]bool{}
	got := DetectUnlocks(0, 0, 3, nil, already)

	found := false
	for _, p := range got {
		if p.Slug == "streak-3" {
			found = true
		}
		if p.Slug == "streak-7" {
			t.Errorf("streak 3 should not unlock the 7-day ribbon: %+v", got)
		}
	}
	if !found {
		t.Fatalf("expected streak-3 ribbon unlock, got %+v", got)
	}
}

func TestDetectUnlocks_SagaCompletion(t *testing.T) {
	already := map[string]bool{}
	completions := map[string]bool{"cerulean": true}
	got := DetectUnlocks(0, 0, 0, completions, already)

	found := false
	for _, p := range got {
		if p.Slug == "onix" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected onix to unlock on cerulean saga completion, got %+v", got)
	}
}

func TestDetectUnlocks_CatchOnlyNeverAutoUnlocks(t *testing.T) {
	already := map[string]bool{}
	// Push XP absurdly high and mark every saga complete; mew (catch_only)
	// must never appear.
	completions := map[string]bool{}
	for _, p := range Pokedex {
		if p.Condition.Type == "saga" {
			completions[p.Condition.Saga] = true
		}
	}
	got := DetectUnlocks(0, 10_000_000, 999, completions, already)
	for _, p := range got {
		if p.Slug == "mew" {
			t.Fatalf("mew must never auto-unlock, got %+v", got)
		}
	}
}

func TestPokemonBySlug(t *testing.T) {
	p, ok := PokemonBySlug("charizard")
	if !ok || p.Name != "Charizard" {
		t.Fatalf("expected to find charizard, got %+v ok=%v", p, ok)
	}
	if _, ok := PokemonBySlug("does-not-exist"); ok {
		t.Fatal("expected not found for bogus slug")
	}
}
