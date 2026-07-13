package api

import (
	"encoding/json"

	"github.com/jimgcampbell/mathgames/internal/game"
)

// questionOut is what GET /api/next and GET /api/daily serve to the client:
// everything needed to render a question, nothing needed to grade it.
// Answer and Explanation never appear here (ARCHITECTURE.md "Answers never
// leave the server before an attempt").
type questionOut struct {
	ID         int64           `json:"id"`
	Skill      string          `json:"skill"`
	Difficulty int             `json:"difficulty"`
	Source     string          `json:"source"`
	Payload    json.RawMessage `json:"payload"`
}

func toQuestionOut(q game.Question) questionOut {
	return questionOut{ID: q.ID, Skill: q.Skill, Difficulty: q.Difficulty, Source: q.Source, Payload: q.Payload}
}

func toQuestionsOut(qs []game.Question) []questionOut {
	out := make([]questionOut, len(qs))
	for i, q := range qs {
		out[i] = toQuestionOut(q)
	}
	return out
}
