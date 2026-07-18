package game

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	mrand "math/rand"
	"testing"
	"time"
)

func testService() (*Service, *fakeStore) {
	store := newFakeStore()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(store, nil, log), store
}

// insertNumericQuestion is a test helper: inserts a "skill × difficulty ->
// value" numeric question directly into the fake store and returns its ID.
func insertNumericQuestion(t *testing.T, store *fakeStore, skill string, difficulty, value int) int64 {
	t.Helper()
	payload, _ := json.Marshal(Payload{Kind: KindNumeric, Prompt: "2+2=?"})
	answer, _ := json.Marshal(NumericAnswer{Value: value})
	q := &Question{
		Skill: skill, Difficulty: difficulty, Source: string(SourceTemplate),
		Payload: payload, Answer: answer, Explanation: "because math",
	}
	if err := store.InsertQuestion(context.Background(), q); err != nil {
		t.Fatalf("insert question: %v", err)
	}
	return q.ID
}

func TestService_Attempt_CorrectIncreasesXPStreakPower(t *testing.T) {
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
	if !res.Correct {
		t.Fatal("expected correct")
	}
	if res.XPEarned <= 0 {
		t.Fatalf("expected positive XP, got %d", res.XPEarned)
	}
	if res.Streak != 1 {
		t.Fatalf("streak = %d, want 1", res.Streak)
	}
	if res.XP != res.XPBefore+int64(res.XPEarned) {
		t.Fatalf("power level mismatch: before=%d earned=%d after=%d",
			res.XPBefore, res.XPEarned, res.XP)
	}

	state, err := store.GetSkillState(ctx, "multiplication")
	if err != nil {
		t.Fatal(err)
	}
	if state.XP != int64(res.XPEarned) {
		t.Fatalf("persisted skill XP = %d, want %d", state.XP, res.XPEarned)
	}
	if state.Streak != 1 {
		t.Fatalf("persisted streak = %d, want 1", state.Streak)
	}
}

func TestService_Attempt_WrongResetsStreakAndIncrementsWrongRun(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "division", 2, 5)

	given, _ := json.Marshal(NumericAnswer{Value: 999}) // wrong
	res, err := svc.Attempt(ctx, sess.ID, qID, given, 500)
	if err != nil {
		t.Fatal(err)
	}
	if res.Correct {
		t.Fatal("expected incorrect")
	}
	if res.XPEarned != 1 {
		t.Fatalf("wrong answer XP = %d, want 1", res.XPEarned)
	}
	if res.Streak != 0 {
		t.Fatalf("streak after wrong = %d, want 0", res.Streak)
	}
	state, _ := store.GetSkillState(ctx, "division")
	if state.WrongRun != 1 {
		t.Fatalf("wrong_run = %d, want 1", state.WrongRun)
	}
}

func TestService_Attempt_TenCorrectPromotesLevel(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)

	for i := 0; i < 10; i++ {
		qID := insertNumericQuestion(t, store, "addsub", 1, 10)
		given, _ := json.Marshal(NumericAnswer{Value: 10})
		if _, err := svc.Attempt(ctx, sess.ID, qID, given, 100); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	state, err := store.GetSkillState(ctx, "addsub")
	if err != nil {
		t.Fatal(err)
	}
	if state.Level != 2 {
		t.Fatalf("after 10/10 correct: level = %d, want 2", state.Level)
	}
	if state.WindowTotal != 0 {
		t.Fatalf("window not reset: %d", state.WindowTotal)
	}
}

func TestService_Attempt_QuestModeAdvancesAndCompletesChapter(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	ch := store.addChapter(QuestChapter{
		Saga: "pewter", Chapter: 1, Title: "t", Story: "s",
		Requirement: QuestRequirement{Correct: 2, Skills: []string{"multiplication"}, MinDifficulty: 1},
		Reward:      QuestReward{XP: 500, Pokemon: "gengar"},
	})

	sess := &Session{Mode: ModeQuest}
	store.CreateSession(ctx, sess)

	for i := 0; i < 2; i++ {
		qID := insertNumericQuestion(t, store, "multiplication", 3, 9)
		given, _ := json.Marshal(NumericAnswer{Value: 9})
		res, err := svc.Attempt(ctx, sess.ID, qID, given, 100)
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if i == 1 {
			// Second correct attempt completes the chapter and grants the
			// reward, so its XP/unlock should show up.
			foundGengar := false
			for _, u := range res.Unlocks {
				if u.Ref == "gengar" {
					foundGengar = true
				}
			}
			if !foundGengar {
				t.Fatalf("expected gengar unlock on chapter completion, got %+v", res.Unlocks)
			}
		}
	}

	updated, err := store.GetQuestChapter(ctx, ch.Saga, ch.Chapter)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Progress != 2 {
		t.Fatalf("progress = %d, want 2", updated.Progress)
	}
	if updated.CompletedAt == nil {
		t.Fatal("expected chapter completed")
	}

	unlocks, _ := store.ListUnlocks(ctx)
	found := false
	for _, u := range unlocks {
		if u.Kind == UnlockPokemon && u.Ref == "gengar" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected gengar pokemon unlock persisted")
	}
}

func TestService_Attempt_DailyModeUpdatesResultsAndPerfectDayBonus(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	q1 := insertNumericQuestion(t, store, "multiplication", 2, 4)
	q2 := insertNumericQuestion(t, store, "division", 2, 3)

	day := time.Now().UTC().Format("2006-01-02")
	if err := store.CreateDailyResult(ctx, &DailyResult{Day: day, QuestionIDs: []int64{q1, q2}}); err != nil {
		t.Fatal(err)
	}

	sess := &Session{Mode: ModeDaily}
	store.CreateSession(ctx, sess)

	given1, _ := json.Marshal(NumericAnswer{Value: 4})
	if _, err := svc.Attempt(ctx, sess.ID, q1, given1, 100); err != nil {
		t.Fatal(err)
	}
	mid, err := store.GetDailyResult(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	if mid.Answered != 1 || mid.CompletedAt != nil {
		t.Fatalf("after 1/2 answered: %+v", mid)
	}

	given2, _ := json.Marshal(NumericAnswer{Value: 3})
	res2, err := svc.Attempt(ctx, sess.ID, q2, given2, 100)
	if err != nil {
		t.Fatal(err)
	}
	final, err := store.GetDailyResult(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	if final.Answered != 2 || final.CompletedAt == nil {
		t.Fatalf("after 2/2 answered: %+v", final)
	}
	if final.Correct != 2 {
		t.Fatalf("correct = %d, want 2 (perfect day)", final.Correct)
	}
	// Perfect day bonus (100) should be folded into the day's total XP, on
	// top of both per-answer XP amounts (this attempt's daily-doubled XP
	// plus the first attempt's).
	if final.XPEarned < 100 {
		t.Fatalf("expected perfect-day bonus folded into day XP, got %d (res2 earned %d)", final.XPEarned, res2.XPEarned)
	}
}

func TestService_Attempt_NewUnlockSurfacesExactlyOnce(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)

	// Seed skill_state XP just under the Pidgey threshold (500) so one
	// more correct attempt crosses it.
	store.UpdateSkillState(ctx, &SkillState{Skill: "multiplication", Level: 5, XP: 399})

	qID := insertNumericQuestion(t, store, "multiplication", 10, 100)
	given, _ := json.Marshal(NumericAnswer{Value: 100})

	res1, err := svc.Attempt(ctx, sess.ID, qID, given, 100)
	if err != nil {
		t.Fatal(err)
	}
	foundFirst := false
	for _, u := range res1.Unlocks {
		if u.Ref == "pidgey" {
			foundFirst = true
		}
	}
	if !foundFirst {
		t.Fatalf("expected pidgey unlock crossing xp 500, xpBefore=%d xpAfter=%d unlocks=%+v",
			res1.XPBefore, res1.XP, res1.Unlocks)
	}

	// A subsequent attempt (still above threshold) must not re-report it.
	qID2 := insertNumericQuestion(t, store, "multiplication", 10, 100)
	res2, err := svc.Attempt(ctx, sess.ID, qID2, given, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range res2.Unlocks {
		if u.Ref == "pidgey" {
			t.Fatalf("pidgey should not be re-reported: %+v", res2.Unlocks)
		}
	}
}

func TestService_NextQuestions_TemplateSkillInsertsAndServes(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	qs, bankLow, err := svc.NextQuestions(ctx, "multiplication", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if bankLow {
		t.Fatal("template skill should never report bank_low")
	}
	if len(qs) != 3 {
		t.Fatalf("got %d questions, want 3", len(qs))
	}
	for _, q := range qs {
		if q.ID == 0 {
			t.Fatal("expected inserted question to have an ID")
		}
		stored, err := store.GetQuestion(ctx, q.ID)
		if err != nil || stored == nil {
			t.Fatalf("question %d not persisted: err=%v", q.ID, err)
		}
	}
}

func TestService_NextQuestions_AISkillReportsBankLow(t *testing.T) {
	svc, _ := testService()
	ctx := context.Background()

	// No AI questions seeded at all -> bank_low should be true.
	qs, bankLow, err := svc.NextQuestions(ctx, "word_problems", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bankLow {
		t.Fatal("expected bank_low when the AI bank is empty")
	}
	if len(qs) != 0 {
		t.Fatalf("expected 0 questions from an empty bank, got %d", len(qs))
	}
}

func TestService_NextQuestions_MixedRespectsSessionAndLevelOverride(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	store.UpdateSettings(ctx, &Settings{ID: 1, DailyCount: 5, LevelOverride: map[string]int{"multiplication": 7}})

	qs, _, err := svc.NextQuestions(ctx, "multiplication", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 1 {
		t.Fatalf("got %d questions", len(qs))
	}
	if qs[0].Difficulty != 7 {
		t.Fatalf("difficulty = %d, want override level 7", qs[0].Difficulty)
	}
}

func TestService_ResetProgress_WipesProgressKeepsBank(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	// Seed a spread of progress: an answered attempt, XP/level/streak,
	// an unlock, a completed daily, a quest chapter with progress, a
	// served AI question, and a level override.
	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	store.questions[qID].TimesServed = 9
	given, _ := json.Marshal(NumericAnswer{Value: 42})
	if _, err := svc.Attempt(ctx, sess.ID, qID, given, 1000); err != nil {
		t.Fatal(err)
	}
	store.UpdateSkillState(ctx, &SkillState{Skill: "division", Level: 6, XP: 1234, Streak: 5, WrongRun: 2, WindowTotal: 7, WindowCorrect: 4})
	store.InsertUnlock(ctx, &Unlock{Kind: "pokemon", Ref: "pidgey", Source: "xp 500"})
	store.CreateDailyResult(ctx, &DailyResult{Day: time.Now().UTC().Format("2006-01-02"), QuestionIDs: []int64{qID}, Answered: 5, Correct: 5})
	ch := store.addChapter(QuestChapter{Saga: "pewter", Chapter: 1, Progress: 8})
	store.settings.LevelOverride = map[string]int{"multiplication": 7}

	if err := svc.ResetProgress(ctx); err != nil {
		t.Fatalf("ResetProgress: %v", err)
	}

	// Progress is gone.
	if len(store.attempts) != 0 {
		t.Fatalf("attempts not cleared: %d", len(store.attempts))
	}
	if len(store.sessions) != 0 {
		t.Fatalf("sessions not cleared: %d", len(store.sessions))
	}
	if len(store.unlocks) != 0 {
		t.Fatalf("unlocks not cleared: %d", len(store.unlocks))
	}
	if len(store.daily) != 0 {
		t.Fatalf("daily not cleared: %d", len(store.daily))
	}
	for skill, s := range store.skillStates {
		if s.Level != 1 || s.XP != 0 || s.Streak != 0 || s.WrongRun != 0 || s.WindowTotal != 0 || s.WindowCorrect != 0 {
			t.Fatalf("skill_state %s not reset: %+v", skill, s)
		}
	}
	if store.chapters[ch.ID].Progress != 0 || store.chapters[ch.ID].CompletedAt != nil {
		t.Fatalf("chapter progress not reset: %+v", store.chapters[ch.ID])
	}
	if len(store.settings.LevelOverride) != 0 {
		t.Fatalf("level_override not cleared: %+v", store.settings.LevelOverride)
	}
	// Power level is back to the floor of 100.
	if p, _ := svc.Profile(ctx); p.XP != 100 {
		t.Fatalf("total xp = %d after reset, want 100", p.XP)
	}

	// The question bank survives (times_served zeroed but the row stays).
	if _, ok := store.questions[qID]; !ok {
		t.Fatal("question bank row was deleted by reset")
	}
	if store.questions[qID].TimesServed != 0 {
		t.Fatalf("times_served = %d, want 0", store.questions[qID].TimesServed)
	}
}

// ---- random events ----

func TestService_Attempt_ForcedEventDoublesXPAndRecords(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	luckyEgg := events[0]
	svc.rollEvent = func(rng *mrand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event { return &luckyEgg }

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 42})

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 9200)
	if err != nil {
		t.Fatal(err)
	}
	if res.Event == nil {
		t.Fatal("expected event in result")
	}
	if res.Event.Slug != "lucky_egg" {
		t.Fatalf("event slug = %q, want lucky_egg", res.Event.Slug)
	}
	if res.Event.Name != luckyEgg.Name || res.Event.Message != luckyEgg.Message || res.Event.Multiplier != "×2" {
		t.Fatalf("event fields = %+v, want name/message/multiplier from registry", res.Event)
	}
	if res.Event.XPBefore <= 0 || res.XPEarned != res.Event.XPBefore*2 {
		t.Fatalf("xp_before=%d xp_earned=%d, want xp_earned = 2*xp_before", res.Event.XPBefore, res.XPEarned)
	}

	if len(store.attempts) != 1 || store.attempts[0].Event != "lucky_egg" {
		t.Fatalf("attempt row event = %q, want lucky_egg", store.attempts[0].Event)
	}
	state, _ := store.GetSkillState(ctx, "multiplication")
	if state.XP != int64(res.XPEarned) {
		t.Fatalf("persisted skill XP = %d, want %d (doubled xp)", state.XP, res.XPEarned)
	}
	if res.XP != res.XPBefore+int64(res.XPEarned) {
		t.Fatalf("power level should reflect doubled XP: before=%d earned=%d after=%d",
			res.XPBefore, res.XPEarned, res.XP)
	}
}

func TestService_Attempt_ForcedCapsuleAddsFlatXPAndRecords(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	rareCandy := findEvent(t, "rare_candy")
	svc.rollEvent = func(rng *mrand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event { return rareCandy }

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 42})

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 9200)
	if err != nil {
		t.Fatal(err)
	}
	if res.Event == nil || res.Event.Slug != "rare_candy" {
		t.Fatalf("expected rare_candy event, got %+v", res.Event)
	}
	if res.XPEarned != res.Event.XPBefore+100 {
		t.Fatalf("xp_before=%d xp_earned=%d, want xp_earned = xp_before + 100", res.Event.XPBefore, res.XPEarned)
	}
	if len(store.attempts) != 1 || store.attempts[0].Event != "rare_candy" {
		t.Fatalf("attempt row event = %q, want rare_candy", store.attempts[0].Event)
	}
}

func TestService_Attempt_ForcedUltraInstinctTriplesXPAndRecords(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	ch2 := findEvent(t, "critical_hit")
	svc.rollEvent = func(rng *mrand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event { return ch2 }

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 42})

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 9200)
	if err != nil {
		t.Fatal(err)
	}
	if res.Event == nil || res.Event.Slug != "critical_hit" {
		t.Fatalf("expected critical_hit event, got %+v", res.Event)
	}
	if res.Event.XPBefore <= 0 || res.XPEarned != res.Event.XPBefore*3 {
		t.Fatalf("xp_before=%d xp_earned=%d, want xp_earned = 3*xp_before", res.Event.XPBefore, res.XPEarned)
	}
	if len(store.attempts) != 1 || store.attempts[0].Event != "critical_hit" {
		t.Fatalf("attempt row event = %q, want critical_hit", store.attempts[0].Event)
	}
}

func TestService_Attempt_NoEventLeavesFieldsEmpty(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	svc.rollEvent = func(rng *mrand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event { return nil }

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 42})

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if res.Event != nil {
		t.Fatalf("expected no event, got %+v", res.Event)
	}
	if len(store.attempts) != 1 || store.attempts[0].Event != "" {
		t.Fatalf("attempt row event = %q, want empty", store.attempts[0].Event)
	}
}

func TestService_Attempt_WrongAnswerNeverFiresEvent(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	luckyEgg := events[0]
	// Even a roll stub that would always fire must never be consulted for a
	// wrong answer (RollEvent is only called when correct).
	svc.rollEvent = func(rng *mrand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event { return &luckyEgg }

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)
	qID := insertNumericQuestion(t, store, "division", 2, 5)
	given, _ := json.Marshal(NumericAnswer{Value: 999}) // wrong

	res, err := svc.Attempt(ctx, sess.ID, qID, given, 500)
	if err != nil {
		t.Fatal(err)
	}
	if res.Correct {
		t.Fatal("expected incorrect")
	}
	if res.Event != nil {
		t.Fatalf("wrong answer should never fire an event, got %+v", res.Event)
	}
	if res.XPEarned != 1 {
		t.Fatalf("wrong answer XP = %d, want 1 (unaffected by event stub)", res.XPEarned)
	}
}

func TestService_Attempt_CooldownHoldsWithinTenAttempts(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	luckyEgg := events[0]
	svc.rollEvent = func(rng *mrand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event { return &luckyEgg }

	sess := &Session{Mode: ModeTraining}
	store.CreateSession(ctx, sess)

	firstQ := insertNumericQuestion(t, store, "multiplication", 4, 42)
	given, _ := json.Marshal(NumericAnswer{Value: 42})
	first, err := svc.Attempt(ctx, sess.ID, firstQ, given, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if first.Event == nil {
		t.Fatal("expected first forced attempt to fire an event")
	}

	// Real cooldown enforcement lives in RollEvent/AttemptsSinceLastEvent, so
	// swap in the real roller for the remaining attempts within the
	// cooldown window and assert none of them fire, no matter the RNG.
	svc.rollEvent = RollEvent
	for i := 0; i < eventCooldown-1; i++ {
		qID := insertNumericQuestion(t, store, "multiplication", 4, 42)
		res, err := svc.Attempt(ctx, sess.ID, qID, given, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if res.Event != nil {
			t.Fatalf("attempt %d within cooldown window fired an event: %+v", i, res.Event)
		}
	}
}
