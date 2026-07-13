package ai

import "encoding/json"

// RawItem is one element of a word_problems/logic batch response, exactly
// as the model produces it and before validation. Payload/Answer are
// passed through to game.Question once validated.
type RawItem struct {
	Payload     json.RawMessage `json:"payload"`
	Answer      json.RawMessage `json:"answer"`
	Explanation string          `json:"explanation"`
	Check       string          `json:"check,omitempty"`
}

// StoryItem is one chapter rewrite from a story batch response.
type StoryItem struct {
	Chapter int    `json:"chapter"`
	Title   string `json:"title"`
	Story   string `json:"story"`
}
