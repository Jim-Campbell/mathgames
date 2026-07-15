package game

import (
	"math/rand"
	"testing"
)

// TestClipRoll_WorkedExample hand-checks the weighted pick: two eligible
// clips (weights 1 and 3), fixed seeds, cap not hit, chance=1 so every roll
// fires. Over many seeds both clips must appear, and the weight-3 clip
// should be picked roughly 3x as often as the weight-1 clip.
func TestClipRoll_WorkedExample(t *testing.T) {
	clips := []Clip{
		{ID: 1, Weight: 1, Enabled: true, OnCorrect: true},
		{ID: 2, Weight: 3, Enabled: true, OnCorrect: true},
	}
	counts := map[int64]int{}
	for seed := int64(0); seed < 4000; seed++ {
		rng := rand.New(rand.NewSource(seed))
		c := ClipRoll(rng, true, clips, 0, 0, 2, 1)
		if c == nil {
			t.Fatalf("seed %d: expected a clip (chance=1), got nil", seed)
		}
		counts[c.ID]++
	}
	if counts[1] == 0 || counts[2] == 0 {
		t.Fatalf("expected both clips to occur, got %v", counts)
	}
	if counts[2] <= counts[1] {
		t.Fatalf("expected weight-3 clip (id=2) to be picked more often than weight-1 clip (id=1), got %v", counts)
	}
}

// TestClipRoll_AvoidsImmediateRepeat: with two eligible clips and
// lastPlayedID set to the higher-weight one, the lower-weight clip must be
// the only possible pick every time (more than one remains, so the repeat
// candidate is dropped entirely).
func TestClipRoll_AvoidsImmediateRepeat(t *testing.T) {
	clips := []Clip{
		{ID: 1, Weight: 1, Enabled: true, OnCorrect: true},
		{ID: 2, Weight: 3, Enabled: true, OnCorrect: true},
	}
	for seed := int64(0); seed < 500; seed++ {
		rng := rand.New(rand.NewSource(seed))
		c := ClipRoll(rng, true, clips, 2, 0, 2, 1)
		if c == nil {
			t.Fatalf("seed %d: expected a clip, got nil", seed)
		}
		if c.ID != 1 {
			t.Fatalf("seed %d: expected repeat of clip 2 to be avoided, got clip %d", seed, c.ID)
		}
	}
}

// TestClipRoll_SingleEligibleClipCanRepeat: with only one eligible clip,
// repeating it is fine (dropping it would leave nothing to pick).
func TestClipRoll_SingleEligibleClipCanRepeat(t *testing.T) {
	clips := []Clip{{ID: 1, Weight: 1, Enabled: true, OnCorrect: true}}
	rng := rand.New(rand.NewSource(1))
	c := ClipRoll(rng, true, clips, 1, 0, 2, 1)
	if c == nil || c.ID != 1 {
		t.Fatalf("expected the sole eligible clip to repeat, got %v", c)
	}
}

func TestClipRoll_SessionCapHit(t *testing.T) {
	clips := []Clip{{ID: 1, Weight: 1, Enabled: true, OnCorrect: true}}
	rng := rand.New(rand.NewSource(1))
	if got := ClipRoll(rng, true, clips, 0, 2, 2, 1); got != nil {
		t.Fatalf("expected nil at session cap, got %v", got)
	}
	if got := ClipRoll(rng, true, clips, 0, 3, 2, 1); got != nil {
		t.Fatalf("expected nil over session cap, got %v", got)
	}
}

func TestClipRoll_EligibilityFilter(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	// Only fires on correct; a wrong answer must yield nil.
	onCorrectOnly := []Clip{{ID: 1, Weight: 1, Enabled: true, OnCorrect: true, OnWrong: false}}
	if got := ClipRoll(rng, false, onCorrectOnly, 0, 0, 2, 1); got != nil {
		t.Fatalf("expected nil for wrong answer against an on_correct-only clip, got %v", got)
	}

	// Disabled clips never fire.
	disabled := []Clip{{ID: 1, Weight: 1, Enabled: false, OnCorrect: true, OnWrong: true}}
	if got := ClipRoll(rng, true, disabled, 0, 0, 2, 1); got != nil {
		t.Fatalf("expected nil for disabled clip, got %v", got)
	}

	// No clips at all.
	if got := ClipRoll(rng, true, nil, 0, 0, 2, 1); got != nil {
		t.Fatalf("expected nil with no eligible clips, got %v", got)
	}
}

// TestClipRoll_FireRateBand documents the intended 1/chance fire rate with a
// fixed seed over 10,000 rolls (default chance=40 -> ~250 expected fires).
func TestClipRoll_FireRateBand(t *testing.T) {
	clips := []Clip{{ID: 1, Weight: 1, Enabled: true, OnCorrect: true}}
	rng := rand.New(rand.NewSource(42))
	fires := 0
	for i := 0; i < 10000; i++ {
		if ClipRoll(rng, true, clips, 0, 0, 1000000, 40) != nil {
			fires++
		}
	}
	if fires < 150 || fires > 400 {
		t.Fatalf("fire count %d out of expected band [150,400] for 10000 rolls at 1/40", fires)
	}
}
