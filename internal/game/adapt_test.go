package game

import "testing"

// TestAdapt_WorkedExample hand-checks ARCHITECTURE.md's example: fresh L3,
// sequence C C C W C C C C C C (9/10 correct) -> promote to L4, window
// resets; next ten W W C W W C W W W C (3/10) -> demote back to L3.
func TestAdapt_WorkedExample(t *testing.T) {
	state := SkillState{Skill: "multiplication", Level: 3}

	round1 := []bool{true, true, true, false, true, true, true, true, true, true} // 9/10
	var changed int
	for _, correct := range round1 {
		state, changed = Adapt(state, correct)
	}
	if state.Level != 4 {
		t.Fatalf("after 9/10 correct: level = %d, want 4", state.Level)
	}
	if changed != 1 {
		t.Fatalf("after promotion: levelChanged = %d, want 1", changed)
	}
	if state.WindowTotal != 0 || state.WindowCorrect != 0 {
		t.Fatalf("window not reset after promotion: total=%d correct=%d", state.WindowTotal, state.WindowCorrect)
	}

	round2 := []bool{false, false, true, false, false, true, false, false, false, true} // 3/10
	for _, correct := range round2 {
		state, changed = Adapt(state, correct)
	}
	if state.Level != 3 {
		t.Fatalf("after 3/10 correct: level = %d, want 3", state.Level)
	}
	if changed != -1 {
		t.Fatalf("after demotion: levelChanged = %d, want -1", changed)
	}
	if state.WindowTotal != 0 || state.WindowCorrect != 0 {
		t.Fatalf("window not reset after demotion: total=%d correct=%d", state.WindowTotal, state.WindowCorrect)
	}
}

func TestAdapt_StayCase(t *testing.T) {
	for _, correctCount := range []int{5, 6, 7} {
		state := SkillState{Skill: "s", Level: 5}
		var changed int
		for i := 0; i < 10; i++ {
			state, changed = Adapt(state, i < correctCount)
		}
		if state.Level != 5 {
			t.Errorf("correctCount=%d: level = %d, want 5 (stay)", correctCount, state.Level)
		}
		if changed != 0 {
			t.Errorf("correctCount=%d: levelChanged = %d, want 0", correctCount, changed)
		}
	}
}

func TestAdapt_CapAtTen(t *testing.T) {
	state := SkillState{Skill: "s", Level: 10}
	var changed int
	for i := 0; i < 10; i++ {
		state, changed = Adapt(state, true) // 10/10 correct
	}
	if state.Level != 10 {
		t.Fatalf("level = %d, want 10 (capped)", state.Level)
	}
	if changed != 0 {
		t.Fatalf("levelChanged = %d, want 0 (already at cap)", changed)
	}
}

func TestAdapt_FloorAtOne(t *testing.T) {
	state := SkillState{Skill: "s", Level: 1}
	var changed int
	for i := 0; i < 10; i++ {
		state, changed = Adapt(state, false) // 0/10 correct
	}
	if state.Level != 1 {
		t.Fatalf("level = %d, want 1 (floored)", state.Level)
	}
	if changed != 0 {
		t.Fatalf("levelChanged = %d, want 0 (already at floor)", changed)
	}
}

func TestAdapt_WindowResetsExactlyAtTen(t *testing.T) {
	state := SkillState{Skill: "s", Level: 5}
	for i := 0; i < 9; i++ {
		state, _ = Adapt(state, true)
	}
	if state.WindowTotal != 9 {
		t.Fatalf("after 9 attempts: window_total = %d, want 9 (no reset yet)", state.WindowTotal)
	}
	if state.Level != 5 {
		t.Fatalf("level changed before window filled: %d", state.Level)
	}
	state, changed := Adapt(state, true) // 10th
	if state.WindowTotal != 0 {
		t.Fatalf("after 10th attempt: window_total = %d, want 0 (reset)", state.WindowTotal)
	}
	if changed != 1 || state.Level != 6 {
		t.Fatalf("10/10 correct should promote: level=%d changed=%d", state.Level, changed)
	}
}

// TestAdapt_OverridePinningIsNotAdaptsJob confirms Adapt has no knowledge of
// settings.level_override — it only ever operates on (and moves) the
// underlying adaptive level regardless of what's being served. Pin
// substitution for serving is a service.go concern (effectiveLevel).
func TestAdapt_OverridePinningIsNotAdaptsJob(t *testing.T) {
	state := SkillState{Skill: "s", Level: 3}
	for i := 0; i < 10; i++ {
		state, _ = Adapt(state, true)
	}
	if state.Level != 4 {
		t.Fatalf("underlying level should keep adapting regardless of any override: got %d, want 4", state.Level)
	}
}
