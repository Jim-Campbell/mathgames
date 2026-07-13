package game

import "testing"

// TestScore hand-checks the ARCHITECTURE.md worked example:
//
//	difficulty 4, elapsed 9200ms, streak reaching 7, no zenkai
//	base = 10*4 = 40
//	fast_ms = 5000+2000*4 = 13000; 9200 <= 13000 -> x150/100 -> 60
//	streak 7 >= 6 -> x125/100 -> 75
//
// 75 XP. The same answer after 3 straight misses (zenkai) doubles to 150.
func TestScore(t *testing.T) {
	got := Score(4, 9200, 7, true, false, false)
	if got != 75 {
		t.Fatalf("worked example: got %d, want 75", got)
	}
	t.Logf("worked example: 40 base -> 60 speed -> 75 streak = %d XP", got)

	gotZenkai := Score(4, 9200, 7, true, true, false)
	if gotZenkai != 150 {
		t.Fatalf("worked example with zenkai: got %d, want 150", gotZenkai)
	}
	t.Logf("with zenkai: 75 XP doubles to %d XP", gotZenkai)
}

func TestScore_StreakBoundaries(t *testing.T) {
	// difficulty 1, fast_ms = 7000, answered instantly so speed x150 always.
	cases := []struct {
		streak int
		want   int
	}{
		{2, 15},  // base 10 -> speed 15 -> streak x100/100 -> 15
		{3, 16},  // streak >= 3 -> x110/100 -> 16 (15*110/100=16)
		{5, 16},  // still x110
		{6, 18},  // streak >= 6 -> x125/100 -> 18 (15*125/100=18)
		{10, 18}, // still x125
		{11, 22}, // streak >= 11 -> x150/100 -> 22 (15*150/100=22)
	}
	for _, c := range cases {
		got := Score(1, 0, c.streak, true, false, false)
		if got != c.want {
			t.Errorf("streak %d: got %d, want %d", c.streak, got, c.want)
		}
	}
}

func TestScore_SpeedBoundaries(t *testing.T) {
	difficulty := 2
	fast := fastMS(difficulty) // 9000
	ok := okMS(difficulty)     // 27000
	base := 10 * difficulty    // 20

	// streak 1 so streak multiplier is x100/100 (no interference).
	atFast := Score(difficulty, fast, 1, true, false, false)
	if want := base * 150 / 100; atFast != want {
		t.Errorf("elapsed == fast_ms: got %d, want %d", atFast, want)
	}
	justOverFast := Score(difficulty, fast+1, 1, true, false, false)
	if want := base * 120 / 100; justOverFast != want {
		t.Errorf("elapsed just over fast_ms: got %d, want %d", justOverFast, want)
	}
	atOK := Score(difficulty, ok, 1, true, false, false)
	if want := base * 120 / 100; atOK != want {
		t.Errorf("elapsed == ok_ms: got %d, want %d", atOK, want)
	}
	justOverOK := Score(difficulty, ok+1, 1, true, false, false)
	if want := base * 100 / 100; justOverOK != want {
		t.Errorf("elapsed just over ok_ms: got %d, want %d", justOverOK, want)
	}
}

func TestScore_WrongAlwaysOneXP(t *testing.T) {
	cases := []struct {
		difficulty, elapsed, streak int
		zenkai, daily               bool
	}{
		{10, 0, 20, true, true},
		{1, 999999, 0, false, false},
		{5, 5000, 11, false, true},
	}
	for _, c := range cases {
		got := Score(c.difficulty, c.elapsed, c.streak, false, c.zenkai, c.daily)
		if got != 1 {
			t.Errorf("wrong answer %+v: got %d XP, want 1", c, got)
		}
	}
}

func TestScore_DailyDoubles(t *testing.T) {
	base := Score(4, 9200, 7, true, false, false)
	doubled := Score(4, 9200, 7, true, false, true)
	if doubled != base*2 {
		t.Errorf("daily doubling: got %d, want %d", doubled, base*2)
	}
}

func TestPerfectDayBonus(t *testing.T) {
	if PerfectDayBonus() != 100 {
		t.Errorf("got %d, want 100", PerfectDayBonus())
	}
}
