package ai

import "testing"

func TestParseItems(t *testing.T) {
	const two = `[
		{"payload": {"kind": "mc", "prompt": "a?", "choices": ["x","y"]}, "answer": {"index": 0}, "explanation": "e1"},
		{"payload": {"kind": "text", "prompt": "b?"}, "answer": {"value": "red", "accept": ["red"]}, "explanation": "e2"}
	]`

	cases := []struct {
		name  string
		text  string
		want  int
		isErr bool
	}{
		{"clean array", two, 2, false},
		{"code fence", "```json\n" + two + "\n```", 2, false},
		{"prose preamble", "Sure! Here are the questions:\n" + two, 2, false},
		{"trailing prose", two + "\nHope these help!", 2, false},
		// Truncated mid-second-object: the first complete item is salvaged.
		{"truncated tail", `[
			{"payload": {"kind": "mc", "prompt": "a?", "choices": ["x","y"]}, "answer": {"index": 0}, "explanation": "e1"},
			{"payload": {"kind": "text", "prompt": "b?"}, "answer": {"value": "re`, 1, false},
		{"no array", "I cannot help with that.", 0, true},
		{"empty array", "[]", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := parseItems(tc.text)
			if tc.isErr {
				if err == nil {
					t.Fatalf("expected error, got %d items", len(items))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(items) != tc.want {
				t.Fatalf("got %d items, want %d", len(items), tc.want)
			}
		})
	}
}
