package game

import (
	"context"
	"time"
)

// Store is the persistence interface the service depends on. internal/db
// implements it with pgx. Kept as one interface so tests can use a fake
// (see fake_store_test.go).
type Store interface {
	// ---- questions ----

	InsertQuestion(ctx context.Context, q *Question) error
	GetQuestion(ctx context.Context, id int64) (*Question, error)

	// PickAIQuestions picks least-served non-retired rows
	// (ORDER BY times_served, random()) at (skill, level), falling back to
	// nearest level (±1, ±2, ...) if fewer than n rows exist there. bankLow
	// is true whenever a fallback was needed or the bank still came up
	// short of n.
	PickAIQuestions(ctx context.Context, skill string, level, n int) (questions []Question, bankLow bool, err error)
	BumpTimesServed(ctx context.Context, ids []int64) error

	// ListQuestions supports the parent review view. source and retired are
	// optional filters; empty/nil means "any".
	ListQuestions(ctx context.Context, skill, source string, retired *bool) ([]Question, error)
	SetQuestionRetired(ctx context.Context, id int64, retired bool) error

	// ---- attempts ----

	InsertAttempt(ctx context.Context, a *Attempt) error

	// HasAttempt reports whether questionID already has a recorded attempt
	// within sessionID (used to reject re-answering a daily question).
	HasAttempt(ctx context.Context, sessionID, questionID int64) (bool, error)

	// ListAttempts returns every attempt since the given time, for the
	// parents summary (GET /api/parents/summary).
	ListAttempts(ctx context.Context, since time.Time) ([]Attempt, error)

	// AttemptsSinceLastEvent counts attempts newer than the most recent
	// attempt whose event is non-null (or every attempt, if none has ever
	// fired) -- the random-event cooldown counter.
	AttemptsSinceLastEvent(ctx context.Context) (int, error)

	// ---- skill state ----

	GetSkillState(ctx context.Context, skill string) (*SkillState, error)
	ListSkillStates(ctx context.Context) ([]SkillState, error)
	UpdateSkillState(ctx context.Context, s *SkillState) error

	// ---- sessions ----

	CreateSession(ctx context.Context, s *Session) error
	EndSession(ctx context.Context, id int64) error
	GetSession(ctx context.Context, id int64) (*Session, error)

	// RecentAttemptCounts returns attempts-per-skill since the given time,
	// for mixed-mode weighting.
	RecentAttemptCounts(ctx context.Context, since time.Time) (map[string]int, error)

	// ---- unlocks ----

	ListUnlocks(ctx context.Context) ([]Unlock, error)
	// InsertUnlock is idempotent (UNIQUE(kind,ref) in the DB); inserted is
	// false when the row already existed, so the caller doesn't report a
	// duplicate as "new".
	InsertUnlock(ctx context.Context, u *Unlock) (inserted bool, err error)
	// DeleteUnlocks consumes unlocks (e.g. the 7 dragon balls on a wish).
	DeleteUnlocks(ctx context.Context, kind string, refs []string) error

	// Wish atomically grants fighterRef in exchange for the 7 dragon-ball
	// unlocks and credits bonusXP to skill_state row bonusSkill, all in one
	// transaction (count check, insert, delete, credit). ballCount is the
	// number of dragon_ball unlocks found; when it isn't 7, nothing else
	// happens. alreadyUnlocked is true when fighterRef was already unlocked
	// (also a no-op beyond the count check).
	Wish(ctx context.Context, fighterRef, bonusSkill string, bonusXP int64) (ballCount int, alreadyUnlocked bool, err error)

	// ---- quests ----

	ListQuestChapters(ctx context.Context) ([]QuestChapter, error)
	GetQuestChapter(ctx context.Context, saga string, chapter int) (*QuestChapter, error)
	UpdateQuestProgress(ctx context.Context, id int64, progress int, completedAt *time.Time) error

	// ---- daily ----

	GetDailyResult(ctx context.Context, day string) (*DailyResult, error)
	CreateDailyResult(ctx context.Context, d *DailyResult) error
	UpdateDailyResult(ctx context.Context, d *DailyResult) error
	// ListDailyResults returns days >= sinceDay, ascending, for the calendar
	// and streak computation.
	ListDailyResults(ctx context.Context, sinceDay string) ([]DailyResult, error)

	// ---- settings ----

	GetSettings(ctx context.Context) (*Settings, error)
	UpdateSettings(ctx context.Context, s *Settings) error

	// ---- screen time ----

	// CountCorrectsSince counts correct attempts created at or after since;
	// nil means "all time" (used when no reset has ever happened).
	CountCorrectsSince(ctx context.Context, since *time.Time) (int, error)
	InsertScreenTimeReset(ctx context.Context, r *ScreenTimeReset) error
	// LastScreenTimeReset returns the most recent reset, or nil if none has
	// happened yet.
	LastScreenTimeReset(ctx context.Context) (*ScreenTimeReset, error)
	// ListScreenTimeResets returns every reset, newest first.
	ListScreenTimeResets(ctx context.Context) ([]ScreenTimeReset, error)

	// ---- export ----

	// ExportAll dumps every table, keyed by table name, for GET /api/export.
	ExportAll(ctx context.Context) (map[string]any, error)

	// ---- AI content generation ----

	// InsertAIBatch records one generation call (full raw response) and sets
	// b.ID/b.CreatedAt; b.Accepted/b.Rejected are filled in afterward via
	// UpdateAIBatchCounts once the caller has validated every item (the
	// batch row's id is needed first, to tag accepted questions with it).
	InsertAIBatch(ctx context.Context, b *AIBatch) error

	// UpdateAIBatchCounts records how many items a batch's items validated
	// into (accepted) vs failed validation (rejected).
	UpdateAIBatchCounts(ctx context.Context, id int64, accepted, rejected int) error

	// UpdateQuestChapterStory rewrites one chapter's title/story after a
	// story batch, tagging it with the batch that produced it.
	UpdateQuestChapterStory(ctx context.Context, id int64, title, story string, aiBatchID int64) error

	// ---- admin ----

	// ResetProgress wipes every player-progress row (attempts, sessions,
	// unlocks, daily_results), resets skill_state to fresh (level 1, 0 XP,
	// streak/wrong_run/window all 0), zeros quest_chapters progress and
	// times_served, and clears settings.level_override — a factory reset
	// for handing the app to a kid the first time, or starting over. The
	// question bank, saga story text, and ai_batches history are untouched
	// (expensive to regenerate, and not "progress").
	ResetProgress(ctx context.Context) error
}
