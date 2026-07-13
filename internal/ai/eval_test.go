package ai

import "testing"

func TestEvalExpr(t *testing.T) {
	cases := []struct {
		expr    string
		want    int
		wantErr bool
	}{
		{"34*12+50", 458, false},
		{"2 + 3 * 4", 14, false},
		{"(2 + 3) * 4", 20, false},
		{"10 / 2", 5, false},
		{"7 / 2", 0, true}, // inexact
		{"5 / 0", 0, true}, // division by zero
		{"-5 + 10", 5, false},
		{"10 - 3 - 2", 5, false},
		{"3 * (4 - 1)", 9, false},
		{"3 *", 0, true}, // malformed
	}
	for _, c := range cases {
		got, err := EvalExpr(c.expr)
		if c.wantErr {
			if err == nil {
				t.Errorf("EvalExpr(%q) = %d, want error", c.expr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("EvalExpr(%q) unexpected error: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("EvalExpr(%q) = %d, want %d", c.expr, got, c.want)
		}
	}
}
