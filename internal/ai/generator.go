package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jimgcampbell/mathgames/internal/game"
)

// recentPromptLimit matches ARCHITECTURE.md "AI content generation": the
// last 50 prompts at that skill/level are sent for repeat avoidance.
const recentPromptLimit = 50

// BatchResult is what POST /api/generate returns.
type BatchResult struct {
	BatchID  int64 `json:"batch_id"`
	Accepted int   `json:"accepted"`
	Rejected int   `json:"rejected"`
}

// Generator ties the Anthropic client, prompt building, validation, and
// persistence together. It never grades or writes game state beyond
// inserting questions/quest_chapters/ai_batches rows -- see
// ARCHITECTURE.md "AI content generation".
type Generator struct {
	store  game.Store
	client Messenger
	model  string
	log    *slog.Logger
}

func NewGenerator(store game.Store, client Messenger, model string, log *slog.Logger) *Generator {
	return &Generator{store: store, client: client, model: model, log: log}
}

// GenerateBatch generates and validates one batch of word_problems or logic
// questions at (skill, difficulty), inserting accepted items as questions
// rows and recording the full call as an ai_batches row.
func (g *Generator) GenerateBatch(ctx context.Context, skill string, difficulty, count int) (*BatchResult, error) {
	if skill != "word_problems" && skill != "logic" {
		return nil, fmt.Errorf("invalid: unsupported AI skill %q", skill)
	}
	if difficulty < 1 || difficulty > 10 {
		return nil, fmt.Errorf("invalid: difficulty must be between 1 and 10")
	}
	if count <= 0 {
		count = 10
	}

	recent, err := g.recentPrompts(ctx, skill, difficulty)
	if err != nil {
		return nil, fmt.Errorf("fetch recent prompts: %w", err)
	}

	systemPrompt, rawText, items, err := GenerateQuestions(ctx, g.client, g.model, skill, difficulty, count, recent)
	if err != nil {
		return nil, fmt.Errorf("generate questions: %w", err)
	}

	batch := &game.AIBatch{
		Kind:       skill,
		Skill:      &skill,
		Difficulty: &difficulty,
		Model:      g.model,
		Prompt:     systemPrompt,
		Raw:        rawJSON(rawText),
	}
	if err := g.store.InsertAIBatch(ctx, batch); err != nil {
		return nil, fmt.Errorf("insert ai batch: %w", err)
	}

	var accepted, rejected int
	for _, item := range items {
		if err := ValidateItem(item); err != nil {
			rejected++
			g.log.Warn("rejected AI question", "skill", skill, "difficulty", difficulty, "error", err)
			continue
		}
		q := &game.Question{
			Skill:       skill,
			Difficulty:  difficulty,
			Source:      string(game.SourceAI),
			Payload:     item.Payload,
			Answer:      item.Answer,
			Explanation: item.Explanation,
			AIModel:     &g.model,
			AIBatchID:   &batch.ID,
		}
		if err := g.store.InsertQuestion(ctx, q); err != nil {
			return nil, fmt.Errorf("insert generated question: %w", err)
		}
		accepted++
	}

	if err := g.store.UpdateAIBatchCounts(ctx, batch.ID, accepted, rejected); err != nil {
		return nil, fmt.Errorf("update ai batch counts: %w", err)
	}

	return &BatchResult{BatchID: batch.ID, Accepted: accepted, Rejected: rejected}, nil
}

// recentPrompts returns up to recentPromptLimit prompts already banked at
// (skill, difficulty), most recent first (ListQuestions already orders by
// created_at desc), for repeat avoidance.
func (g *Generator) recentPrompts(ctx context.Context, skill string, difficulty int) ([]string, error) {
	qs, err := g.store.ListQuestions(ctx, skill, string(game.SourceAI), nil)
	if err != nil {
		return nil, err
	}
	var prompts []string
	for _, q := range qs {
		if q.Difficulty != difficulty {
			continue
		}
		var p struct {
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(q.Payload, &p); err == nil && p.Prompt != "" {
			prompts = append(prompts, p.Prompt)
		}
		if len(prompts) >= recentPromptLimit {
			break
		}
	}
	return prompts, nil
}

// GenerateStorySaga rewrites every chapter's title/story for one saga.
func (g *Generator) GenerateStorySaga(ctx context.Context, saga string) (*BatchResult, error) {
	chapters, err := g.store.ListQuestChapters(ctx)
	if err != nil {
		return nil, fmt.Errorf("list quest chapters: %w", err)
	}
	var inputs []ChapterInput
	var sagaChapters []game.QuestChapter
	for _, ch := range chapters {
		if ch.Saga != saga {
			continue
		}
		sagaChapters = append(sagaChapters, ch)
		inputs = append(inputs, ChapterInput{
			Chapter:            ch.Chapter,
			CurrentTitle:       ch.Title,
			RequirementSkills:  ch.Requirement.Skills,
			RequirementCorrect: ch.Requirement.Correct,
			RequirementMinDiff: ch.Requirement.MinDifficulty,
		})
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("invalid: unknown saga %q", saga)
	}

	systemPrompt, rawText, items, err := GenerateStory(ctx, g.client, g.model, saga, inputs)
	if err != nil {
		return nil, fmt.Errorf("generate story: %w", err)
	}

	batch := &game.AIBatch{
		Kind:   "story",
		Skill:  &saga,
		Model:  g.model,
		Prompt: systemPrompt,
		Raw:    rawJSON(rawText),
	}
	if err := g.store.InsertAIBatch(ctx, batch); err != nil {
		return nil, fmt.Errorf("insert ai batch: %w", err)
	}

	byChapter := make(map[int]int64, len(sagaChapters))
	for _, ch := range sagaChapters {
		byChapter[ch.Chapter] = ch.ID
	}

	var accepted, rejected int
	for _, item := range items {
		id, ok := byChapter[item.Chapter]
		if !ok || item.Title == "" || item.Story == "" {
			rejected++
			g.log.Warn("rejected AI story chapter", "saga", saga, "chapter", item.Chapter)
			continue
		}
		if err := g.store.UpdateQuestChapterStory(ctx, id, item.Title, item.Story, batch.ID); err != nil {
			return nil, fmt.Errorf("update quest chapter story: %w", err)
		}
		accepted++
	}

	return &BatchResult{BatchID: batch.ID, Accepted: accepted, Rejected: rejected}, nil
}

// rawJSON wraps arbitrary (possibly non-JSON) model output in a small JSON
// object so it can always be stored in the ai_batches.raw JSONB column,
// regardless of whether the model's response parsed cleanly.
func rawJSON(text string) json.RawMessage {
	b, err := json.Marshal(map[string]string{"response_text": text})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
