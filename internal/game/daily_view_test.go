package game

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestService_Daily_SameDayIsDeterministic(t *testing.T) {
	svc, _ := testService()
	ctx := context.Background()

	v1, err := svc.Daily(ctx, "2026-01-15")
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	v2, err := svc.Daily(ctx, "2026-01-15")
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	if len(v1.Questions) == 0 {
		t.Fatal("expected daily questions to be seeded")
	}
	ids1 := questionIDs(v1.Questions)
	ids2 := questionIDs(v2.Questions)
	if len(ids1) != len(ids2) {
		t.Fatalf("question count changed between fetches: %v vs %v", ids1, ids2)
	}
	for i := range ids1 {
		if ids1[i] != ids2[i] {
			t.Fatalf("question_ids changed between fetches: %v vs %v", ids1, ids2)
		}
	}
}

func TestService_Attempt_DailyRejectsReAnswer(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	today := time.Now().UTC().Format("2006-01-02")
	view, err := svc.Daily(ctx, today)
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	if len(view.Questions) == 0 {
		t.Fatal("expected at least one daily question")
	}
	q := view.Questions[0]

	sess := &Session{Mode: ModeDaily}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	var payload Payload
	if err := json.Unmarshal(q.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	given := wrongGivenFor(t, payload.Kind) // wrong is fine, just needs to be a well-formed attempt

	if _, err := svc.Attempt(ctx, sess.ID, q.ID, given, 1000); err != nil {
		t.Fatalf("first attempt: %v", err)
	}

	_, err = svc.Attempt(ctx, sess.ID, q.ID, given, 1000)
	if err == nil {
		t.Fatal("expected an error re-answering the same daily question")
	}
	if !strings.HasPrefix(err.Error(), "conflict:") {
		t.Fatalf("expected conflict: prefix, got %q", err.Error())
	}
}

// wrongGivenFor builds a well-formed but (almost certainly) incorrect
// answer payload for the given question kind, so tests can submit an
// attempt without needing to know the actual generated answer.
func wrongGivenFor(t *testing.T, kind string) json.RawMessage {
	t.Helper()
	var v any
	switch kind {
	case KindNumeric:
		v = NumericAnswer{Value: -999999}
	case KindNumeric2:
		v = Numeric2Answer{Values: [2]int{-1, -1}}
	case KindMC:
		v = MCAnswer{Index: 999}
	case KindFraction:
		v = FractionAnswer{Num: -1, Den: 1}
	case KindText:
		v = TextAnswer{Value: "__not_a_match__"}
	default:
		t.Fatalf("unknown kind %q", kind)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func questionIDs(qs []Question) []int64 {
	out := make([]int64, len(qs))
	for i, q := range qs {
		out[i] = q.ID
	}
	return out
}
