package game

// Adapt runs the 10-attempt rolling-window adaptive-difficulty rule
// (ARCHITECTURE.md "Adaptive difficulty") for one attempt. It only touches
// Level/WindowTotal/WindowCorrect on the returned state — Skill/XP/Streak/
// WrongRun/UpdatedAt are the caller's concern (service.go updates them
// alongside this call).
//
// Rule: increment window_total (and window_correct if correct) each
// attempt. When window_total reaches 10: window_correct >= 8 promotes the
// level by 1 (capped at 10); window_correct <= 4 demotes by 1 (floored at
// 1); otherwise the level stays. The window resets to 0/0 in all three
// cases.
//
// Worked example (ARCHITECTURE.md): fresh L3, sequence
// C C C W C C C C C C (9/10 correct) -> promote to L4, window resets;
// next ten W W C W W C W W W C (3/10) -> demote back to L3.
//
// A settings.level_override entry pins the *served* level while this
// window/level state keeps updating underneath — that substitution is a
// service.go concern, not Adapt's.
func Adapt(state SkillState, correct bool) (newState SkillState, levelChanged int) {
	newState = state
	newState.WindowTotal++
	if correct {
		newState.WindowCorrect++
	}

	if newState.WindowTotal < 10 {
		return newState, 0
	}

	switch {
	case newState.WindowCorrect >= 8:
		if newState.Level < 10 {
			newState.Level++
			levelChanged = 1
		}
	case newState.WindowCorrect <= 4:
		if newState.Level > 1 {
			newState.Level--
			levelChanged = -1
		}
	}
	newState.WindowTotal = 0
	newState.WindowCorrect = 0
	return newState, levelChanged
}
