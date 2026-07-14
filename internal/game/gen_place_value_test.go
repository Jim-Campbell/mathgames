package game

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestDigitCount(t *testing.T) {
	cases := []struct {
		n, d, want int
	}{
		{566, 6, 2}, {566, 5, 1}, {305, 0, 1}, {111, 1, 3}, {789, 0, 0}, {0, 0, 1}, {0, 5, 0},
	}
	for _, c := range cases {
		if got := digitCount(c.n, c.d); got != c.want {
			t.Errorf("digitCount(%d,%d)=%d, want %d", c.n, c.d, got, c.want)
		}
	}
}

// TestPlaceValueL1_Unambiguous guards the fix for the "value of the digit 6
// in 566" bug: the queried digit must appear exactly once, and its value must
// match the answer.
func TestPlaceValueL1_Unambiguous(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		payload, answer, _ := genPlaceValue(1, rng)
		var digit, n int
		if _, err := fmt.Sscanf(payload.Prompt, "What is the value of the digit %d in %d?", &digit, &n); err != nil {
			t.Fatalf("unexpected prompt %q: %v", payload.Prompt, err)
		}
		if c := digitCount(n, digit); c != 1 {
			t.Fatalf("digit %d appears %d times in %d (ambiguous): %q", digit, c, n, payload.Prompt)
		}
		a := answer.(NumericAnswer)
		// The value must be the digit times the power of ten at its position.
		if digit != 0 && a.Value%digit != 0 {
			t.Fatalf("value %d not a multiple of digit %d for %q", a.Value, digit, payload.Prompt)
		}
	}
}
