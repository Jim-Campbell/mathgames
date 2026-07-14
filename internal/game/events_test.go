package game

import (
	"math/rand"
	"testing"
)

// mid is a difficulty-4 "middling" elapsedMS: past fastMS(4)=13000 but not
// past okMS(4)=39000, so neither speed-gated event is eligible. Used by
// tests that don't care about speed-gating (cooldown, fire-rate).
const mid = 20000

// TestEvent_Apply_WorkedExample hand-checks the ARCHITECTURE.md scoring
// worked example (difficulty 4, fast, streak 7 -> 75 XP) with kaioken
// applied last:
//
//	75 XP * 2/1 = 150 XP; xp_before = 75.
func TestEvent_Apply_WorkedExample(t *testing.T) {
	base := Score(4, 9200, 7, true, false, false)
	if base != 75 {
		t.Fatalf("worked example base: got %d, want 75", base)
	}
	kaioken := events[0]
	got := kaioken.Apply(base)
	if got != 150 {
		t.Fatalf("kaioken applied to worked example: got %d, want 150", got)
	}
}

// TestEvent_Apply_UltraInstinct hand-checks the ARCHITECTURE.md worked
// example (75 XP) with ultra_instinct (×3): 75 * 3/1 = 225.
func TestEvent_Apply_UltraInstinct(t *testing.T) {
	base := Score(4, 9200, 7, true, false, false)
	if base != 75 {
		t.Fatalf("worked example base: got %d, want 75", base)
	}
	ui := findEvent(t, "ultra_instinct")
	got := ui.Apply(base)
	if got != 225 {
		t.Fatalf("ultra_instinct applied to worked example: got %d, want 225", got)
	}
}

// TestEvent_Apply_Capsule hand-checks the ARCHITECTURE.md worked example
// (75 XP) with capsule (flat +100, no multiplier): 75*1/1 + 100 = 175.
func TestEvent_Apply_Capsule(t *testing.T) {
	base := Score(4, 9200, 7, true, false, false)
	if base != 75 {
		t.Fatalf("worked example base: got %d, want 75", base)
	}
	capsule := findEvent(t, "capsule")
	got := capsule.Apply(base)
	if got != 175 {
		t.Fatalf("capsule applied to worked example: got %d, want 175", got)
	}
}

// TestEvent_Apply_ElderKai hand-checks its own worked example (can't
// co-occur with a speed bonus): difficulty 4, slow (>okMS=39000), streak 7
// -> base 40, no speed bonus, ×125/100 streak = 50, elder_kai ×2 -> 100.
func TestEvent_Apply_ElderKai(t *testing.T) {
	base := Score(4, 39001, 7, true, false, false)
	if base != 50 {
		t.Fatalf("elder kai worked example base: got %d, want 50", base)
	}
	ek := findEvent(t, "elder_kai")
	got := ek.Apply(base)
	if got != 100 {
		t.Fatalf("elder_kai applied to worked example: got %d, want 100", got)
	}
}

func findEvent(t *testing.T, slug string) *Event {
	t.Helper()
	for i := range events {
		if events[i].Slug == slug {
			return &events[i]
		}
	}
	t.Fatalf("no event with slug %q in registry", slug)
	return nil
}

func TestRollEvent_CooldownNeverFiresUnderThreshold(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		rng := rand.New(rand.NewSource(seed))
		for n := 0; n < eventCooldown; n++ {
			if ev := RollEvent(rng, n, mid, 4); ev != nil {
				t.Fatalf("seed %d attemptsSinceLast %d: fired %q, want nil (cooldown)", seed, n, ev.Slug)
			}
		}
	}
}

func TestRollEvent_CanFireAtCooldownThreshold(t *testing.T) {
	fired := false
	for seed := int64(0); seed < 500; seed++ {
		rng := rand.New(rand.NewSource(seed))
		if ev := RollEvent(rng, eventCooldown, mid, 4); ev != nil {
			fired = true
			break
		}
	}
	if !fired {
		t.Fatal("expected at least one fire across 500 seeds at attemptsSinceLast == eventCooldown")
	}
}

// TestRollEvent_FireRateBand documents the intended ~1/25 fire rate with a
// fixed seed over 10,000 rolls, well clear of the cooldown. Deterministic for
// this seed; the band just documents intent (expected ~400 for 10,000/25).
func TestRollEvent_FireRateBand(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	fires := 0
	for i := 0; i < 10000; i++ {
		if RollEvent(rng, 100, mid, 4) != nil {
			fires++
		}
	}
	if fires < 300 || fires > 500 {
		t.Fatalf("fire count %d out of expected band [300,500] for 10000 rolls at 1/%d", fires, eventChance)
	}
}

func TestEventRegistry_Sanity(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range events {
		if seen[e.Slug] {
			t.Fatalf("duplicate slug %q in events registry", e.Slug)
		}
		seen[e.Slug] = true
		if e.Weight <= 0 {
			t.Fatalf("event %q: Weight must be > 0, got %d", e.Slug, e.Weight)
		}
		if e.XPDen <= 0 {
			t.Fatalf("event %q: XPDen must be > 0, got %d", e.Slug, e.XPDen)
		}
		if e.XPFlat < 0 {
			t.Fatalf("event %q: XPFlat must be >= 0, got %d", e.Slug, e.XPFlat)
		}
		if e.XPNum == e.XPDen && e.XPFlat <= 0 {
			t.Fatalf("event %q: flat-only event must have positive XPFlat", e.Slug)
		}
		if got := e.Apply(100); got <= 0 {
			t.Fatalf("event %q: Apply(100) = %d, want positive", e.Slug, got)
		}
	}
}

// TestRollEvent_Eligibility exercises the difficulty-4 thresholds
// (fastMS=13000, okMS=39000): a fast answer (13000) can roll ultra_instinct
// but never elder_kai; a slow answer (39001) the reverse; a middling answer
// (20000) neither.
func TestRollEvent_Eligibility(t *testing.T) {
	const difficulty = 4
	cases := []struct {
		name          string
		elapsedMS     int
		wantNeverSlug string
	}{
		{"fast", 13000, "elder_kai"},
		{"slow", 39001, "ultra_instinct"},
		{"middling", 20000, ""}, // checked separately below
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seenUltra, seenElder := false, false
			for seed := int64(0); seed < 10000; seed++ {
				rng := rand.New(rand.NewSource(seed))
				if ev := RollEvent(rng, eventCooldown, c.elapsedMS, difficulty); ev != nil {
					if ev.Slug == "ultra_instinct" {
						seenUltra = true
					}
					if ev.Slug == "elder_kai" {
						seenElder = true
					}
				}
			}
			if c.name == "middling" && (seenUltra || seenElder) {
				t.Fatalf("middling elapsedMS %d: ultra_instinct=%v elder_kai=%v, want neither", c.elapsedMS, seenUltra, seenElder)
			}
			if c.wantNeverSlug == "ultra_instinct" && seenUltra {
				t.Fatalf("slow elapsedMS %d: ultra_instinct fired, want never", c.elapsedMS)
			}
			if c.wantNeverSlug == "elder_kai" && seenElder {
				t.Fatalf("fast elapsedMS %d: elder_kai fired, want never", c.elapsedMS)
			}
		})
	}
}

// TestRollEvent_WeightWalk_Fast confirms fast answers can roll kaioken,
// capsule, and ultra_instinct (ordered by weight/count), never elder_kai.
func TestRollEvent_WeightWalk_Fast(t *testing.T) {
	const difficulty = 4
	counts := map[string]int{}
	for seed := int64(0); seed < 10000; seed++ {
		rng := rand.New(rand.NewSource(seed))
		if ev := RollEvent(rng, eventCooldown, 13000, difficulty); ev != nil {
			counts[ev.Slug]++
		}
	}
	if counts["elder_kai"] != 0 {
		t.Fatalf("fast rolls produced elder_kai %d times, want 0", counts["elder_kai"])
	}
	if counts["kaioken"] == 0 || counts["capsule"] == 0 || counts["ultra_instinct"] == 0 {
		t.Fatalf("expected kaioken, capsule, and ultra_instinct all to occur, got %v", counts)
	}
	if !(counts["kaioken"] > counts["capsule"] && counts["capsule"] > counts["ultra_instinct"]) {
		t.Fatalf("expected kaioken > capsule > ultra_instinct by count, got %v", counts)
	}
}

// TestRollEvent_WeightWalk_Slow confirms slow answers can roll elder_kai,
// never ultra_instinct.
func TestRollEvent_WeightWalk_Slow(t *testing.T) {
	const difficulty = 4
	counts := map[string]int{}
	for seed := int64(0); seed < 10000; seed++ {
		rng := rand.New(rand.NewSource(seed))
		if ev := RollEvent(rng, eventCooldown, 39001, difficulty); ev != nil {
			counts[ev.Slug]++
		}
	}
	if counts["ultra_instinct"] != 0 {
		t.Fatalf("slow rolls produced ultra_instinct %d times, want 0", counts["ultra_instinct"])
	}
	if counts["elder_kai"] == 0 {
		t.Fatalf("expected elder_kai to occur among slow rolls, got %v", counts)
	}
}
