package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxPromptLen = 500

// payloadShape is the subset of game.Payload needed for validation --
// duplicated here (rather than importing internal/game) to keep the AI
// package's validation independent of the domain package's exact struct
// tags, matching the "loose, AI-authored" note on Display.Grid.
type payloadShape struct {
	Kind    string   `json:"kind"`
	Prompt  string   `json:"prompt"`
	Choices []string `json:"choices"`
}

// ValidateItem checks one RawItem's shape and (for numeric answers) its
// check expression, per ARCHITECTURE.md "AI content generation". A nil
// error means the item is safe to insert as a game.Question.
func ValidateItem(item RawItem) error {
	if len(item.Payload) == 0 {
		return fmt.Errorf("missing payload")
	}
	if len(item.Answer) == 0 {
		return fmt.Errorf("missing answer")
	}
	if strings.TrimSpace(item.Explanation) == "" {
		return fmt.Errorf("missing explanation")
	}

	var payload payloadShape
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload json: %w", err)
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		return fmt.Errorf("empty prompt")
	}
	if len(prompt) > maxPromptLen {
		return fmt.Errorf("prompt exceeds %d chars", maxPromptLen)
	}

	switch payload.Kind {
	case "numeric":
		var ans struct {
			Value int `json:"value"`
		}
		if err := json.Unmarshal(item.Answer, &ans); err != nil {
			return fmt.Errorf("invalid numeric answer: %w", err)
		}
		if strings.TrimSpace(item.Check) == "" {
			return fmt.Errorf("numeric answer missing check expression")
		}
		got, err := EvalExpr(item.Check)
		if err != nil {
			return fmt.Errorf("check expression %q: %w", item.Check, err)
		}
		if got != ans.Value {
			return fmt.Errorf("check expression %q = %d, want %d", item.Check, got, ans.Value)
		}

	case "numeric2":
		var ans struct {
			Values [2]int `json:"values"`
		}
		if err := json.Unmarshal(item.Answer, &ans); err != nil {
			return fmt.Errorf("invalid numeric2 answer: %w", err)
		}

	case "mc":
		var ans struct {
			Index int `json:"index"`
		}
		if err := json.Unmarshal(item.Answer, &ans); err != nil {
			return fmt.Errorf("invalid mc answer: %w", err)
		}
		if len(payload.Choices) < 2 || len(payload.Choices) > 5 {
			return fmt.Errorf("mc choices count %d out of range [2,5]", len(payload.Choices))
		}
		if ans.Index < 0 || ans.Index >= len(payload.Choices) {
			return fmt.Errorf("mc index %d out of range for %d choices", ans.Index, len(payload.Choices))
		}

	case "fraction":
		var ans struct {
			Num int `json:"num"`
			Den int `json:"den"`
		}
		if err := json.Unmarshal(item.Answer, &ans); err != nil {
			return fmt.Errorf("invalid fraction answer: %w", err)
		}
		if ans.Den <= 0 {
			return fmt.Errorf("fraction denominator must be > 0, got %d", ans.Den)
		}

	case "text":
		var ans struct {
			Value  string   `json:"value"`
			Accept []string `json:"accept"`
		}
		if err := json.Unmarshal(item.Answer, &ans); err != nil {
			return fmt.Errorf("invalid text answer: %w", err)
		}
		if strings.TrimSpace(ans.Value) == "" {
			return fmt.Errorf("empty text answer value")
		}

	default:
		return fmt.Errorf("unknown kind %q", payload.Kind)
	}

	return nil
}
