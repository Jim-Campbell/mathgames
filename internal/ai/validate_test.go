package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

// goodWordProblemsFixture is a recorded (hand-written, representative)
// word_problems batch response: a well-formed numeric item whose check
// expression matches its answer.
const goodWordProblemsFixture = `[
  {
    "payload": {"kind": "numeric", "prompt": "Ash trains 34 Pokémon for 12 days each, plus 50 bonus minutes. How many total minutes?"},
    "answer": {"value": 458},
    "explanation": "34 warriors x 12 days = 408, plus 50 bonus minutes = 458.",
    "check": "34*12+50"
  },
  {
    "payload": {"kind": "numeric", "prompt": "Brock has 120 Poké Puffs split evenly among 8 Pokémon. How many does each get?"},
    "answer": {"value": 15},
    "explanation": "120 / 8 = 15 Poké Puffs per Pokémon.",
    "check": "120/8"
  }
]`

// badWordProblemsFixture mixes one valid item with three invalid ones: a
// check expression that doesn't match the answer, a missing check field,
// and malformed answer JSON.
const badWordProblemsFixture = `[
  {
    "payload": {"kind": "numeric", "prompt": "Misty trains 20 minutes, three times a day, for 5 days. Total minutes?"},
    "answer": {"value": 300},
    "explanation": "20 x 3 x 5 = 300.",
    "check": "20*3*5"
  },
  {
    "payload": {"kind": "numeric", "prompt": "This check expression lies about its answer."},
    "answer": {"value": 999},
    "explanation": "Bogus.",
    "check": "1+1"
  },
  {
    "payload": {"kind": "numeric", "prompt": "This one has no check field at all."},
    "answer": {"value": 42},
    "explanation": "Missing check."
  },
  {
    "payload": {"kind": "numeric", "prompt": "This one has malformed answer JSON."},
    "answer": "not-an-object",
    "explanation": "Bad shape.",
    "check": "1+1"
  }
]`

const goodLogicFixture = `[
  {
    "payload": {"kind": "mc", "prompt": "Which does not belong: Pikachu, Charmander, Squirtle, banana?", "choices": ["Pikachu", "Charmander", "Squirtle", "banana"]},
    "answer": {"index": 3},
    "explanation": "A banana is not a Pokémon."
  },
  {
    "payload": {"kind": "text", "prompt": "In a grid puzzle, Misty is left of Brock and right of Ash. Who is in the middle?"},
    "answer": {"value": "misty", "accept": ["misty"]},
    "explanation": "Misty sits between Ash and Brock."
  }
]`

func TestValidateItem_Good(t *testing.T) {
	var items []RawItem
	if err := json.Unmarshal([]byte(goodWordProblemsFixture), &items); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	for i, item := range items {
		if err := ValidateItem(item); err != nil {
			t.Errorf("item %d: expected valid, got error: %v", i, err)
		}
	}

	var logicItems []RawItem
	if err := json.Unmarshal([]byte(goodLogicFixture), &logicItems); err != nil {
		t.Fatalf("unmarshal logic fixture: %v", err)
	}
	for i, item := range logicItems {
		if err := ValidateItem(item); err != nil {
			t.Errorf("logic item %d: expected valid, got error: %v", i, err)
		}
	}
}

func TestValidateItem_Bad(t *testing.T) {
	var items []RawItem
	if err := json.Unmarshal([]byte(badWordProblemsFixture), &items); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 fixture items, got %d", len(items))
	}

	if err := ValidateItem(items[0]); err != nil {
		t.Errorf("item 0 should be valid, got error: %v", err)
	}

	wantSubstrings := []string{"check expression", "missing check", "invalid numeric answer"}
	for i, want := range wantSubstrings {
		idx := i + 1
		err := ValidateItem(items[idx])
		if err == nil {
			t.Errorf("item %d: expected error, got nil", idx)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("item %d: error %q does not contain %q", idx, err.Error(), want)
		}
	}
}

func TestValidateItem_UnknownKind(t *testing.T) {
	item := RawItem{
		Payload:     json.RawMessage(`{"kind":"essay","prompt":"write a poem"}`),
		Answer:      json.RawMessage(`{"value":"anything"}`),
		Explanation: "n/a",
	}
	if err := ValidateItem(item); err == nil {
		t.Error("expected error for unknown kind")
	}
}

func TestValidateItem_PromptTooLong(t *testing.T) {
	long := strings.Repeat("x", maxPromptLen+1)
	item := RawItem{
		Payload:     json.RawMessage(`{"kind":"text","prompt":"` + long + `"}`),
		Answer:      json.RawMessage(`{"value":"x"}`),
		Explanation: "n/a",
	}
	if err := ValidateItem(item); err == nil {
		t.Error("expected error for over-length prompt")
	}
}
