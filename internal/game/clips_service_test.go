package game

import (
	"context"
	"encoding/json"
	mrand "math/rand"
	"testing"
)

func TestService_Attempt_ForcedClipAttachesAndRecordsPlay(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	svc.clipRoll = func(rng *mrand.Rand, correct bool, eligible []Clip, lastPlayedID int64, playsThisSession, sessionCap, chance int) *Clip {
		return &Clip{ID: 1, Title: "Uncle Jim says hi", URL: "https://cdn.example.com/clips/a.mp4", ContentType: "video/mp4"}
	}

	if err := store.InsertClip(ctx, &Clip{Title: "Uncle Jim says hi", Enabled: true, OnCorrect: true, Weight: 1}); err != nil {
		t.Fatal(err)
	}

	sess := &Session{Mode: ModeTraining}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 42})

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 1000)
	if err != nil {
		t.Fatalf("Attempt: %v", err)
	}
	if res.Clip == nil {
		t.Fatal("expected a clip to be attached")
	}
	if res.Clip.Title != "Uncle Jim says hi" || res.Clip.URL != "https://cdn.example.com/clips/a.mp4" {
		t.Fatalf("unexpected clip payload: %+v", res.Clip)
	}

	plays, err := store.ListClipPlays(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plays) != 1 {
		t.Fatalf("expected 1 clip play recorded, got %d", len(plays))
	}
	if plays[0].Trigger != "correct" {
		t.Fatalf("trigger = %q, want correct", plays[0].Trigger)
	}
	if c, _ := store.GetClip(ctx, 1); c.PlayCount != 1 {
		t.Fatalf("play_count = %d, want 1", c.PlayCount)
	}
}

func TestService_Attempt_NoClipsMeansNoClipAttached(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	sess := &Session{Mode: ModeTraining}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 42})

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 1000)
	if err != nil {
		t.Fatalf("Attempt: %v", err)
	}
	if res.Clip != nil {
		t.Fatalf("expected no clip with an empty clip bank, got %+v", res.Clip)
	}
}

func TestService_Attempt_ClipCanFireOnWrongAnswer(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	svc.clipRoll = func(rng *mrand.Rand, correct bool, eligible []Clip, lastPlayedID int64, playsThisSession, sessionCap, chance int) *Clip {
		if correct {
			return nil
		}
		return &Clip{ID: 1, Title: "You've got this!", URL: "https://cdn.example.com/clips/b.mp4", ContentType: "video/mp4"}
	}
	if err := store.InsertClip(ctx, &Clip{Title: "You've got this!", Enabled: true, OnWrong: true, Weight: 1}); err != nil {
		t.Fatal(err)
	}

	sess := &Session{Mode: ModeTraining}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 999})

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 1000)
	if err != nil {
		t.Fatalf("Attempt: %v", err)
	}
	if res.Correct {
		t.Fatal("expected wrong answer")
	}
	if res.Clip == nil {
		t.Fatal("expected a clip to be attached on a wrong answer")
	}
}
