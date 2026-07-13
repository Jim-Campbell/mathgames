package game

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

// genForTest re-marshals a generator's payload/answer through JSON, exactly
// as service.go does before Grade() ever sees it, so the round-trip (e.g.
// [2]int -> JSON array -> [2]int) is exercised too.
func genForTest(t *testing.T, skill string, level int, rng *rand.Rand) (Payload, json.RawMessage, string) {
	t.Helper()
	gen, ok := templateGenerators[skill]
	if !ok {
		t.Fatalf("no generator registered for skill %q", skill)
	}
	payload, answer, explanation := gen(level, rng)
	answerJSON, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("marshal answer: %v", err)
	}
	return payload, answerJSON, explanation
}

// TestGenerators_SelfConsistent generates 200 questions per skill x level
// (1-10) with a fixed seed and checks: the stored answer grades correct
// against itself, the payload's kind matches what the level dictates, and
// level-appropriate bounds hold.
func TestGenerators_SelfConsistent(t *testing.T) {
	skills := []string{"multiplication", "division", "addsub", "fractions", "place_value", "patterns"}
	for _, skill := range skills {
		skill := skill
		t.Run(skill, func(t *testing.T) {
			for level := 1; level <= 10; level++ {
				level := level
				t.Run(levelName(level), func(t *testing.T) {
					rng := rand.New(rand.NewSource(int64(level)*1000 + hashString(skill)))
					for i := 0; i < 200; i++ {
						payload, answerJSON, explanation := genForTest(t, skill, level, rng)

						if payload.Prompt == "" {
							t.Fatalf("iter %d: empty prompt", i)
						}
						if explanation == "" {
							t.Fatalf("iter %d: empty explanation", i)
						}
						if strings.Contains(explanation, "%!") {
							t.Fatalf("iter %d: malformed explanation (fmt error): %q", i, explanation)
						}

						// (a) the stored answer grades correct against itself.
						ok, err := Grade(payload.Kind, answerJSON, answerJSON)
						if err != nil {
							t.Fatalf("iter %d: grade self: %v", i, err)
						}
						if !ok {
							t.Fatalf("iter %d: answer does not grade correct against itself: payload=%+v answer=%s", i, payload, answerJSON)
						}

						// (c) kind matches the spec for this skill/level.
						wantKind := expectedKind(skill, level)
						if payload.Kind != wantKind {
							t.Fatalf("iter %d: kind = %q, want %q", i, payload.Kind, wantKind)
						}

						// (b) bounds check.
						checkBounds(t, skill, level, payload, answerJSON, i)
					}
				})
			}
		})
	}
}

func levelName(level int) string { return "L" + itoa(level) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func hashString(s string) int64 {
	var h int64 = 5381
	for _, c := range s {
		h = h*33 + int64(c)
	}
	return h
}

func expectedKind(skill string, level int) string {
	switch skill {
	case "multiplication":
		return KindNumeric
	case "division":
		switch level {
		case 4, 6, 9, 10:
			return KindNumeric2
		default:
			return KindNumeric
		}
	case "addsub":
		return KindNumeric
	case "fractions":
		switch level {
		case 1, 4:
			return KindMC
		case 3, 9:
			return KindNumeric
		default:
			return KindFraction
		}
	case "place_value":
		switch level {
		case 2, 4, 6, 10:
			return KindMC
		default:
			return KindNumeric
		}
	case "patterns":
		if level == 9 {
			return KindMC
		}
		return KindNumeric
	default:
		t := skill
		_ = t
		return KindNumeric
	}
}

// checkBounds asserts operands/results stay within the level's documented
// magnitude (ARCHITECTURE.md "Skills and difficulty"). It's intentionally
// loose (order-of-magnitude, not exact digit-count enforcement in every
// case) — the point is to catch a generator drifting wildly out of its
// level, not to pin every generator constant.
func checkBounds(t *testing.T, skill string, level int, payload Payload, answerJSON json.RawMessage, iter int) {
	t.Helper()
	within := func(v, lo, hi int) {
		t.Helper()
		if v < lo || v > hi {
			t.Fatalf("iter %d: value %d out of bounds [%d,%d] for %s L%d", iter, v, lo, hi, skill, level)
		}
	}

	switch skill {
	case "multiplication":
		var a NumericAnswer
		_ = json.Unmarshal(answerJSON, &a)
		maxByLevel := map[int]int{
			1: 5 * 5, 2: 9 * 9, 3: 12 * 12, 4: 99 * 9, 5: 90 * 90,
			6: 99 * 99, 7: 999 * 9, 8: 999 * 99, 9: 9 * 9 * 250, 10: 9999 * 99,
		}
		within(a.Value, 0, maxByLevel[level])

	case "division":
		if level == 4 || level == 6 || level == 9 || level == 10 {
			var a Numeric2Answer
			_ = json.Unmarshal(answerJSON, &a)
			if a.Values[1] < 1 {
				t.Fatalf("iter %d: remainder %d should be >= 1", iter, a.Values[1])
			}
		} else {
			var a NumericAnswer
			_ = json.Unmarshal(answerJSON, &a)
			if a.Value < 1 {
				t.Fatalf("iter %d: quotient %d should be >= 1", iter, a.Value)
			}
		}

	case "addsub":
		var a NumericAnswer
		_ = json.Unmarshal(answerJSON, &a)
		maxByLevel := map[int]int{
			1: 198, 2: 198, 3: 1998, 4: 999 * 3, 5: 17998,
			6: 9999, 7: 199998, 8: 899*2 - 10, 9: 8999 + 8099, 10: 899999 + 89999,
		}
		if a.Value < -maxByLevel[level] || a.Value > maxByLevel[level] {
			t.Fatalf("iter %d: value %d out of bounds for %s L%d", iter, a.Value, skill, level)
		}

	case "fractions":
		if payload.Kind == KindFraction {
			var a FractionAnswer
			_ = json.Unmarshal(answerJSON, &a)
			if a.Den == 0 {
				t.Fatalf("iter %d: zero denominator", iter)
			}
			if a.Den > 300 || a.Num > 300 || a.Num < -300 {
				t.Fatalf("iter %d: fraction %d/%d implausibly large", iter, a.Num, a.Den)
			}
		}
		if level == 2 && payload.Display == nil {
			t.Fatalf("iter %d: L2 fraction-of-a-shape must set display.fraction_bar", iter)
		}

	case "place_value":
		if payload.Kind == KindNumeric {
			var a NumericAnswer
			_ = json.Unmarshal(answerJSON, &a)
			if a.Value < 0 || a.Value > 10_100_000 {
				t.Fatalf("iter %d: place_value numeric answer %d out of range", iter, a.Value)
			}
		}

	case "patterns":
		if level >= 1 && level != 9 {
			if payload.Display == nil || payload.Display.Sequence == nil {
				t.Fatalf("iter %d: patterns L%d must set display.sequence", iter, level)
			}
			blanks := 0
			for _, v := range payload.Display.Sequence {
				if v == nil {
					blanks++
				}
			}
			if blanks != 1 {
				t.Fatalf("iter %d: expected exactly one blank slot, got %d", iter, blanks)
			}
		}
	}
}
