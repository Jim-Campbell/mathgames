package game

import (
	"fmt"
	"reflect"
	"testing"
)

func sampleLevels() map[string]int {
	return map[string]int{
		"multiplication": 3, "division": 2, "addsub": 4,
		"fractions": 1, "place_value": 5, "patterns": 2,
	}
}

func TestSeedDaily_Deterministic(t *testing.T) {
	levels := sampleLevels()
	ai := []string{"word_problems", "logic"}

	a := SeedDaily("2026-07-13", levels, ai, 5)
	b := SeedDaily("2026-07-13", levels, ai, 5)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same day+levels produced different picks:\n%+v\n%+v", a, b)
	}
	if len(a) == 0 {
		t.Fatal("expected at least one pick")
	}
}

// TestSeedDaily_DifferentDaysDiffer is probabilistic: over ~30 distinct
// days, no two produce an identical (skills,levels) picks tuple. A false
// failure here would mean a hash collision in the seed, worth investigating
// rather than just rerunning.
func TestSeedDaily_DifferentDaysDiffer(t *testing.T) {
	levels := sampleLevels()
	ai := []string{"word_problems", "logic"}

	seen := map[string]string{}
	for m := 1; m <= 3; m++ {
		for d := 1; d <= 10; d++ {
			day := fmt.Sprintf("2026-%02d-%02d", m, d)
			picks := SeedDaily(day, levels, ai, 5)
			key := fmt.Sprintf("%+v", picks)
			if otherDay, exists := seen[key]; exists {
				t.Fatalf("days %q and %q produced identical picks: %s", day, otherDay, key)
			}
			seen[key] = day
		}
	}
}

func TestSeedDaily_CountAndSkills(t *testing.T) {
	levels := sampleLevels()
	ai := []string{"word_problems"}
	picks := SeedDaily("2026-01-01", levels, ai, 5)
	if len(picks) > 5 {
		t.Fatalf("got %d picks, want at most 5", len(picks))
	}
	seenSkills := map[string]bool{}
	for _, p := range picks {
		if seenSkills[p.Skill] {
			t.Fatalf("skill %q picked more than once: %+v", p.Skill, picks)
		}
		seenSkills[p.Skill] = true
	}
}
