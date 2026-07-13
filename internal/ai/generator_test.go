package ai

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jimgcampbell/mathgames/internal/game"
)

// stubStore is a minimal game.Store fake covering only what Generator
// touches (ListQuestions, InsertQuestion, InsertAIBatch,
// UpdateAIBatchCounts, ListQuestChapters, UpdateQuestChapterStory). The
// embedded nil game.Store satisfies the rest of the interface; calling any
// other method would panic, which is fine since Generator never does.
type stubStore struct {
	game.Store

	questions   []*game.Question
	nextQID     int64
	batches     []*game.AIBatch
	nextBatchID int64
	chapters    []*game.QuestChapter
}

func (s *stubStore) InsertQuestion(ctx context.Context, q *game.Question) error {
	s.nextQID++
	q.ID = s.nextQID
	q.CreatedAt = time.Now()
	cp := *q
	s.questions = append(s.questions, &cp)
	return nil
}

func (s *stubStore) ListQuestions(ctx context.Context, skill, source string, retired *bool) ([]game.Question, error) {
	var out []game.Question
	for _, q := range s.questions {
		if skill != "" && q.Skill != skill {
			continue
		}
		if source != "" && q.Source != source {
			continue
		}
		out = append(out, *q)
	}
	return out, nil
}

func (s *stubStore) InsertAIBatch(ctx context.Context, b *game.AIBatch) error {
	s.nextBatchID++
	b.ID = s.nextBatchID
	b.CreatedAt = time.Now()
	cp := *b
	s.batches = append(s.batches, &cp)
	return nil
}

func (s *stubStore) UpdateAIBatchCounts(ctx context.Context, id int64, accepted, rejected int) error {
	for _, b := range s.batches {
		if b.ID == id {
			b.Accepted = accepted
			b.Rejected = rejected
		}
	}
	return nil
}

func (s *stubStore) ListQuestChapters(ctx context.Context) ([]game.QuestChapter, error) {
	var out []game.QuestChapter
	for _, ch := range s.chapters {
		out = append(out, *ch)
	}
	return out, nil
}

func (s *stubStore) UpdateQuestChapterStory(ctx context.Context, id int64, title, story string, aiBatchID int64) error {
	for _, ch := range s.chapters {
		if ch.ID == id {
			ch.Title = title
			ch.Story = story
			ch.AIBatchID = &aiBatchID
		}
	}
	return nil
}

// fakeMessenger scripts a fixed response text for CreateMessage.
type fakeMessenger struct {
	text string
	err  error
}

func (m *fakeMessenger) CreateMessage(ctx context.Context, req Request) (*Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &Response{Content: []contentBlock{{Type: "text", Text: m.text}}}, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestGenerator_GenerateBatch_AcceptsAndRejects(t *testing.T) {
	// One valid numeric item, one with a check expression that lies.
	resp := `[
		{"payload": {"kind": "numeric", "prompt": "Goku trains 34 warriors for 12 days, plus 50 bonus minutes."}, "answer": {"value": 458}, "explanation": "34*12+50=458", "check": "34*12+50"},
		{"payload": {"kind": "numeric", "prompt": "This one lies about its check."}, "answer": {"value": 999}, "explanation": "bogus", "check": "1+1"}
	]`
	store := &stubStore{}
	gen := NewGenerator(store, &fakeMessenger{text: resp}, "claude-sonnet-5", discardLogger())

	result, err := gen.GenerateBatch(context.Background(), "word_problems", 3, 2)
	if err != nil {
		t.Fatalf("GenerateBatch: %v", err)
	}
	if result.Accepted != 1 || result.Rejected != 1 {
		t.Errorf("got accepted=%d rejected=%d, want 1/1", result.Accepted, result.Rejected)
	}
	if len(store.questions) != 1 {
		t.Fatalf("expected 1 inserted question, got %d", len(store.questions))
	}
	q := store.questions[0]
	if q.Skill != "word_problems" || q.Difficulty != 3 || q.Source != string(game.SourceAI) {
		t.Errorf("unexpected question fields: %+v", q)
	}
	if q.AIBatchID == nil || *q.AIBatchID != result.BatchID {
		t.Errorf("question ai_batch_id = %v, want %d", q.AIBatchID, result.BatchID)
	}
	if len(store.batches) != 1 || store.batches[0].Accepted != 1 || store.batches[0].Rejected != 1 {
		t.Fatalf("unexpected batch record: %+v", store.batches)
	}
}

func TestGenerator_GenerateBatch_InvalidSkill(t *testing.T) {
	store := &stubStore{}
	gen := NewGenerator(store, &fakeMessenger{text: "[]"}, "claude-sonnet-5", discardLogger())
	_, err := gen.GenerateBatch(context.Background(), "multiplication", 3, 5)
	if err == nil {
		t.Fatal("expected error for non-AI skill")
	}
}

func TestGenerator_GenerateStorySaga(t *testing.T) {
	store := &stubStore{
		chapters: []*game.QuestChapter{
			{ID: 1, Saga: "saiyan", Chapter: 1, Title: "old title", Requirement: game.QuestRequirement{Correct: 8, Skills: []string{"multiplication"}, MinDifficulty: 1}},
			{ID: 2, Saga: "saiyan", Chapter: 2, Title: "old title 2", Requirement: game.QuestRequirement{Correct: 10, Skills: []string{"division"}, MinDifficulty: 1}},
		},
	}
	resp := `[
		{"chapter": 1, "title": "Training Begins", "story": "Goku sends you to train. Land 8 multiplication hits to prove your strength!"},
		{"chapter": 2, "title": "Raditz Arrives", "story": "A new threat appears. Solve 10 division problems to power up!"}
	]`
	gen := NewGenerator(store, &fakeMessenger{text: resp}, "claude-sonnet-5", discardLogger())

	result, err := gen.GenerateStorySaga(context.Background(), "saiyan")
	if err != nil {
		t.Fatalf("GenerateStorySaga: %v", err)
	}
	if result.Accepted != 2 || result.Rejected != 0 {
		t.Errorf("got accepted=%d rejected=%d, want 2/0", result.Accepted, result.Rejected)
	}
	if store.chapters[0].Title != "Training Begins" || store.chapters[1].Title != "Raditz Arrives" {
		t.Errorf("titles not updated: %+v", store.chapters)
	}
	if store.chapters[0].AIBatchID == nil || *store.chapters[0].AIBatchID != result.BatchID {
		t.Errorf("chapter ai_batch_id not set to batch id")
	}
}

func TestGenerator_GenerateStorySaga_UnknownSaga(t *testing.T) {
	store := &stubStore{}
	gen := NewGenerator(store, &fakeMessenger{text: "[]"}, "claude-sonnet-5", discardLogger())
	_, err := gen.GenerateStorySaga(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown saga")
	}
}
