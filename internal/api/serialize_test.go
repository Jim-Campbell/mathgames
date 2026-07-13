package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jimgcampbell/mathgames/internal/game"
)

// TestQuestionOutStripsAnswer fails if answer/explanation ever leak into the
// client-facing question serialization (ARCHITECTURE.md "Answers never
// leave the server before an attempt").
func TestQuestionOutStripsAnswer(t *testing.T) {
	q := game.Question{
		ID:          1,
		Skill:       "multiplication",
		Difficulty:  4,
		Source:      "template",
		Payload:     json.RawMessage(`{"kind":"numeric","prompt":"27 x 34 = ?"}`),
		Answer:      json.RawMessage(`{"value":918}`),
		Explanation: "27x34 = 27x30 + 27x4 = 810 + 108",
	}

	b, err := json.Marshal(toQuestionOut(q))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)

	if strings.Contains(s, "\"answer\"") {
		t.Fatalf("questionOut JSON leaks answer: %s", s)
	}
	if strings.Contains(s, "\"explanation\"") {
		t.Fatalf("questionOut JSON leaks explanation: %s", s)
	}
	if strings.Contains(s, "918") {
		t.Fatalf("questionOut JSON leaks answer value: %s", s)
	}
	if !strings.Contains(s, "27 x 34") {
		t.Fatalf("questionOut JSON dropped the prompt: %s", s)
	}
}

func TestQuestionsOutStripsAnswer(t *testing.T) {
	qs := []game.Question{
		{ID: 1, Skill: "division", Difficulty: 2, Source: "template",
			Payload: json.RawMessage(`{"kind":"numeric","prompt":"8 / 2 = ?"}`),
			Answer:  json.RawMessage(`{"value":4}`), Explanation: "8/2=4"},
	}
	b, err := json.Marshal(toQuestionsOut(qs))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "\"answer\"") || strings.Contains(s, "\"explanation\"") {
		t.Fatalf("toQuestionsOut leaks answer/explanation: %s", s)
	}
}
