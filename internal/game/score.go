package game

// fastMS and okMS are the speed-bonus thresholds for a given difficulty
// (1-10). fast_ms = 5000 + 2000*difficulty; ok_ms = 3*fast_ms.
func fastMS(difficulty int) int { return 5000 + 2000*difficulty }
func okMS(difficulty int) int   { return 3 * fastMS(difficulty) }

// Score computes the XP for one attempt. All integer math, multiply before
// divide, matching every other derived amount in this app.
//
// Worked example (ARCHITECTURE.md "Scoring"): difficulty 4, answered
// correctly in 9.2s with streak reaching 7, no zenkai.
//
//	base = 10*4 = 40
//	fast_ms = 5000+2000*4 = 13000; 9200 <= 13000 -> speed x150/100 -> 60
//	streak 7 >= 6 -> x125/100 -> 75
//
// 75 XP. The same answer arriving after 3 straight misses (zenkai) doubles
// to 150.
//
// A wrong answer always earns a flat 1 XP, bypassing speed/streak/zenkai/
// daily entirely ("showing up counts").
func Score(difficulty, elapsedMS, streakAfter int, correct, zenkai, daily bool) int {
	if !correct {
		return 1
	}

	base := 10 * difficulty

	var speedMult int
	switch {
	case elapsedMS <= fastMS(difficulty):
		speedMult = 150
	case elapsedMS <= okMS(difficulty):
		speedMult = 120
	default:
		speedMult = 100
	}
	xp := base * speedMult / 100

	var streakMult int
	switch {
	case streakAfter >= 11:
		streakMult = 150
	case streakAfter >= 6:
		streakMult = 125
	case streakAfter >= 3:
		streakMult = 110
	default:
		streakMult = 100
	}
	xp = xp * streakMult / 100

	if zenkai {
		xp *= 2
	}
	if daily {
		xp *= 2
	}
	return xp
}

// PerfectDayBonus is a flat XP bonus applied once, by the caller, when a
// daily set is completed with every question correct. It is a session-level
// bonus, never folded into per-answer Score().
func PerfectDayBonus() int { return 100 }
