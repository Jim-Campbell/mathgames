package game

import (
	"encoding/json"
	"testing"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %+v: %v", v, err)
	}
	return b
}

func TestGrade_Numeric(t *testing.T) {
	answer := mustJSON(t, NumericAnswer{Value: 918})
	ok, err := Grade(KindNumeric, mustJSON(t, NumericAnswer{Value: 918}), answer)
	if err != nil || !ok {
		t.Fatalf("exact match: ok=%v err=%v", ok, err)
	}
	ok, err = Grade(KindNumeric, mustJSON(t, NumericAnswer{Value: 917}), answer)
	if err != nil || ok {
		t.Fatalf("wrong value: ok=%v err=%v", ok, err)
	}
}

func TestGrade_Numeric2(t *testing.T) {
	answer := mustJSON(t, Numeric2Answer{Values: [2]int{21, 3}})
	ok, _ := Grade(KindNumeric2, mustJSON(t, Numeric2Answer{Values: [2]int{21, 3}}), answer)
	if !ok {
		t.Fatal("both values correct should grade true")
	}
	ok, _ = Grade(KindNumeric2, mustJSON(t, Numeric2Answer{Values: [2]int{21, 4}}), answer)
	if ok {
		t.Fatal("wrong remainder should grade false")
	}
	ok, _ = Grade(KindNumeric2, mustJSON(t, Numeric2Answer{Values: [2]int{20, 3}}), answer)
	if ok {
		t.Fatal("wrong quotient should grade false")
	}
}

func TestGrade_MC(t *testing.T) {
	answer := mustJSON(t, MCAnswer{Index: 1})
	ok, _ := Grade(KindMC, mustJSON(t, MCAnswer{Index: 1}), answer)
	if !ok {
		t.Fatal("matching index should grade true")
	}
	ok, _ = Grade(KindMC, mustJSON(t, MCAnswer{Index: 0}), answer)
	if ok {
		t.Fatal("wrong index should grade false")
	}
}

func TestGrade_Fraction(t *testing.T) {
	target := mustJSON(t, FractionAnswer{Num: 3, Den: 4})

	// 6/8 is equivalent to 3/4 and must be accepted.
	ok, err := Grade(KindFraction, mustJSON(t, FractionAnswer{Num: 6, Den: 8}), target)
	if err != nil || !ok {
		t.Fatalf("6/8 should be accepted for target 3/4: ok=%v err=%v", ok, err)
	}

	// 2/5 is not equivalent to 3/4 and must be rejected.
	ok, err = Grade(KindFraction, mustJSON(t, FractionAnswer{Num: 2, Den: 5}), target)
	if err != nil || ok {
		t.Fatalf("2/5 should be rejected for target 3/4: ok=%v err=%v", ok, err)
	}

	// A non-reduced target (4/8) should still accept its lowest-terms form (1/2).
	target2 := mustJSON(t, FractionAnswer{Num: 4, Den: 8})
	ok, err = Grade(KindFraction, mustJSON(t, FractionAnswer{Num: 1, Den: 2}), target2)
	if err != nil || !ok {
		t.Fatalf("1/2 should be accepted for non-reduced target 4/8: ok=%v err=%v", ok, err)
	}
}

func TestGrade_Text(t *testing.T) {
	target := mustJSON(t, TextAnswer{Value: "red", Accept: []string{"the red one"}})

	ok, _ := Grade(KindText, mustJSON(t, TextAnswer{Value: "RED"}), target)
	if !ok {
		t.Fatal("case-insensitive match should grade true")
	}
	ok, _ = Grade(KindText, mustJSON(t, TextAnswer{Value: "  red  "}), target)
	if !ok {
		t.Fatal("whitespace-insensitive match should grade true")
	}
	ok, _ = Grade(KindText, mustJSON(t, TextAnswer{Value: "The Red One"}), target)
	if !ok {
		t.Fatal("accept-list match should grade true")
	}
	ok, _ = Grade(KindText, mustJSON(t, TextAnswer{Value: "blue"}), target)
	if ok {
		t.Fatal("non-matching value should grade false")
	}
}

func TestGrade_UnknownKind(t *testing.T) {
	if _, err := Grade("bogus", mustJSON(t, NumericAnswer{}), mustJSON(t, NumericAnswer{})); err == nil {
		t.Fatal("unknown kind should error")
	}
}
