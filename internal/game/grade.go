package game

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Grade is a pure function: given a question kind and the canonical answer,
// decide whether the given answer is correct. Never mutates state.
func Grade(kind string, given, answer json.RawMessage) (bool, error) {
	switch kind {
	case KindNumeric:
		var g, a NumericAnswer
		if err := json.Unmarshal(given, &g); err != nil {
			return false, fmt.Errorf("unmarshal given numeric: %w", err)
		}
		if err := json.Unmarshal(answer, &a); err != nil {
			return false, fmt.Errorf("unmarshal answer numeric: %w", err)
		}
		return g.Value == a.Value, nil

	case KindNumeric2:
		var g, a Numeric2Answer
		if err := json.Unmarshal(given, &g); err != nil {
			return false, fmt.Errorf("unmarshal given numeric2: %w", err)
		}
		if err := json.Unmarshal(answer, &a); err != nil {
			return false, fmt.Errorf("unmarshal answer numeric2: %w", err)
		}
		return g.Values == a.Values, nil

	case KindMC:
		var g, a MCAnswer
		if err := json.Unmarshal(given, &g); err != nil {
			return false, fmt.Errorf("unmarshal given mc: %w", err)
		}
		if err := json.Unmarshal(answer, &a); err != nil {
			return false, fmt.Errorf("unmarshal answer mc: %w", err)
		}
		return g.Index == a.Index, nil

	case KindFraction:
		var g, a FractionAnswer
		if err := json.Unmarshal(given, &g); err != nil {
			return false, fmt.Errorf("unmarshal given fraction: %w", err)
		}
		if err := json.Unmarshal(answer, &a); err != nil {
			return false, fmt.Errorf("unmarshal answer fraction: %w", err)
		}
		if g.Den == 0 || a.Den == 0 {
			return false, nil
		}
		// Integer cross-multiplication accepts equivalent forms (6/8 for 3/4).
		return g.Num*a.Den == a.Num*g.Den, nil

	case KindText:
		var g TextAnswer
		var a TextAnswer
		if err := json.Unmarshal(given, &g); err != nil {
			return false, fmt.Errorf("unmarshal given text: %w", err)
		}
		if err := json.Unmarshal(answer, &a); err != nil {
			return false, fmt.Errorf("unmarshal answer text: %w", err)
		}
		norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
		given := norm(g.Value)
		if given == norm(a.Value) {
			return true, nil
		}
		for _, alt := range a.Accept {
			if given == norm(alt) {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("unknown question kind: %q", kind)
	}
}
