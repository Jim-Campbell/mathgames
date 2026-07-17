package game

import (
	"encoding/json"
	"time"
)

// Payload kinds. The kind determines which answer shape (below) applies and
// how the PWA renders the input control.
const (
	KindNumeric  = "numeric"
	KindNumeric2 = "numeric2"
	KindMC       = "mc"
	KindFraction = "fraction"
	KindText     = "text"
)

// Session modes.
const (
	ModeTraining = "training"
	ModeQuest    = "quest"
	ModeDaily    = "daily"
)

// Unlock kinds, matching the DB CHECK constraint on unlocks.kind.
const (
	UnlockFighter    = "fighter"
	UnlockDragonBall = "dragon_ball"
	UnlockBadge      = "badge"
)

// Rarity tiers for the fighter catalog.
type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

// FractionBar is a display hint rendering a shaded pie/bar for visual
// fraction questions.
type FractionBar struct {
	Parts  int `json:"parts"`
	Shaded int `json:"shaded"`
}

// Display carries optional render hints for the PWA; any subset may be set
// depending on the question's kind/skill.
type Display struct {
	FractionBar *FractionBar    `json:"fraction_bar,omitempty"`
	Sequence    []*int          `json:"sequence,omitempty"` // pattern with the blank slot as a nil entry
	Grid        json.RawMessage `json:"grid,omitempty"`     // logic grid puzzles; shape is loose, AI-authored
}

// Payload is the client-visible half of a question: everything needed to
// render it, and nothing needed to grade it.
type Payload struct {
	Kind    string   `json:"kind"`
	Prompt  string   `json:"prompt"`
	Labels  []string `json:"labels,omitempty"`
	Choices []string `json:"choices,omitempty"`
	Display *Display `json:"display,omitempty"`
}

// Answer shapes, one per Kind. Generators build one of these and
// json.Marshal it into Question.Answer.

// NumericAnswer is used by kind "numeric": a single integer input.
type NumericAnswer struct {
	Value int `json:"value"`
}

// Numeric2Answer is used by kind "numeric2": two integer inputs (e.g.
// quotient + remainder).
type Numeric2Answer struct {
	Values [2]int `json:"values"`
}

// MCAnswer is used by kind "mc": the index of the correct choice.
type MCAnswer struct {
	Index int `json:"index"`
}

// FractionAnswer is used by kind "fraction": graded by integer
// cross-multiplication so equivalent forms (6/8 for target 3/4) are
// accepted.
type FractionAnswer struct {
	Num int `json:"num"`
	Den int `json:"den"`
}

// TextAnswer is used by kind "text": graded case/space-insensitive against
// Value or any of Accept.
type TextAnswer struct {
	Value  string   `json:"value"`
	Accept []string `json:"accept,omitempty"`
}

// Question is a served/servable question row. Payload and Answer are raw
// JSONB pass-through; generators marshal a Payload/answer struct into them.
// Answer is never serialized to the client before an attempt is submitted.
type Question struct {
	ID          int64           `json:"id"`
	Skill       string          `json:"skill"`
	Difficulty  int             `json:"difficulty"`
	Source      string          `json:"source"`
	Payload     json.RawMessage `json:"payload"`
	Answer      json.RawMessage `json:"answer,omitempty"`
	Explanation string          `json:"explanation,omitempty"`
	AIModel     *string         `json:"ai_model,omitempty"`
	AIBatchID   *int64          `json:"ai_batch_id,omitempty"`
	TimesServed int             `json:"times_served"`
	Retired     bool            `json:"retired"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Attempt is the raw record of one answer submission; never aggregated away.
type Attempt struct {
	ID          int64           `json:"id"`
	SessionID   int64           `json:"session_id"`
	QuestionID  int64           `json:"question_id"`
	Skill       string          `json:"skill"`
	Difficulty  int             `json:"difficulty"`
	Given       json.RawMessage `json:"given"`
	Correct     bool            `json:"correct"`
	ElapsedMS   int             `json:"elapsed_ms"`
	XPEarned    int             `json:"xp_earned"`
	StreakAfter int             `json:"streak_after"`
	LevelAfter  int             `json:"level_after"`
	Event       string          `json:"event,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// AttemptResult is what POST /api/attempts returns: everything the PWA
// needs for the moment of feedback.
type AttemptResult struct {
	Correct           bool            `json:"correct"`
	Answer            json.RawMessage `json:"answer"`
	Explanation       string          `json:"explanation"`
	XPEarned          int             `json:"xp_earned"`
	Zenkai            bool            `json:"zenkai"`
	Streak            int             `json:"streak"`
	SkillLevel        int             `json:"skill_level"`
	LevelChanged      int             `json:"level_changed"` // -1/0/+1
	PowerLevel        int64           `json:"power_level"`
	PowerLevelBefore  int64           `json:"power_level_before"`
	Unlocks           []Unlock        `json:"unlocks"`
	Event             *EventResult    `json:"event,omitempty"`
	ScreenTimeMinutes int             `json:"screen_time_minutes"`
	Clip              *ClipPlay       `json:"clip,omitempty"`
}

// ClipPlay is the API-facing shape of a clip chosen by ClipRoll, carried on
// AttemptResult.
type ClipPlay struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
}

// Clip is a manage-page video clip row (migration 005). Video features are
// off until all five R2_* env vars are set (see internal/storage.R2Client).
type Clip struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	R2Key       string    `json:"r2_key"`
	URL         string    `json:"url"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	DurationMS  *int      `json:"duration_ms,omitempty"`
	Enabled     bool      `json:"enabled"`
	OnCorrect   bool      `json:"on_correct"`
	OnWrong     bool      `json:"on_wrong"`
	Weight      int       `json:"weight"`
	PlayCount   int       `json:"play_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// ClipPlayLog is one clip_plays row for the manage page's recent-plays list
// (GET /api/clips/plays), joined with the clip's title for display.
type ClipPlayLog struct {
	ID        int64     `json:"id"`
	ClipID    int64     `json:"clip_id"`
	ClipTitle string    `json:"clip_title"`
	AttemptID *int64    `json:"attempt_id,omitempty"`
	Trigger   string    `json:"trigger"`
	PlayedAt  time.Time `json:"played_at"`
}

// EventResult is the API-facing shape of a fired Event, carried on
// AttemptResult. XPBefore is the XP before the event multiplier was applied;
// XPEarned on the enclosing AttemptResult is the final post-event value.
type EventResult struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Message    string `json:"message"`
	Multiplier string `json:"multiplier"` // "×2" — display string
	XPBefore   int    `json:"xp_before"`
}

// SkillState is the per-skill adaptive/XP state; DB primary key is skill.
type SkillState struct {
	Skill         string    `json:"skill"`
	Level         int       `json:"level"`
	XP            int64     `json:"xp"`
	Streak        int       `json:"streak"`
	WrongRun      int       `json:"wrong_run"`
	WindowTotal   int       `json:"window_total"`
	WindowCorrect int       `json:"window_correct"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Session groups attempts under a mode (training/quest/daily).
type Session struct {
	ID        int64      `json:"id"`
	Mode      string     `json:"mode"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// UnlockCondition describes how a Fighter (or badge) is earned.
type UnlockCondition struct {
	Type       string `json:"type"` // "power_level"|"saga"|"streak"|"wish_only"
	PowerLevel int64  `json:"power_level,omitempty"`
	Saga       string `json:"saga,omitempty"`
	Chapter    int    `json:"chapter,omitempty"`
	StreakDays int    `json:"streak_days,omitempty"`
}

// Fighter is a code-defined catalog entry (internal/game/fighters.go); the
// DB stores only which ones have been unlocked.
type Fighter struct {
	Slug      string          `json:"slug"`
	Name      string          `json:"name"`
	Rarity    Rarity          `json:"rarity"`
	Condition UnlockCondition `json:"condition"`
}

// Unlock is a DB row recording an earned fighter/dragon-ball/badge. Name and
// Rarity are optional and filled in by the service from the Fighter catalog
// for API convenience (not stored — badges and dragon balls have no catalog
// rarity).
type Unlock struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Ref       string    `json:"ref"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name,omitempty"`
	Rarity    string    `json:"rarity,omitempty"`
}

// QuestRequirement gates chapter completion.
type QuestRequirement struct {
	Correct       int      `json:"correct"`
	Skills        []string `json:"skills"`
	MinDifficulty int      `json:"min_difficulty"`
}

// QuestReward is granted on chapter completion; any subset of fields may be
// set (all omitempty).
type QuestReward struct {
	XP         int    `json:"xp,omitempty"`
	Fighter    string `json:"fighter,omitempty"`
	DragonBall int    `json:"dragon_ball,omitempty"`
}

// QuestChapter is one saga chapter.
type QuestChapter struct {
	ID          int64            `json:"id"`
	Saga        string           `json:"saga"`
	Chapter     int              `json:"chapter"`
	Title       string           `json:"title"`
	Story       string           `json:"story"`
	Requirement QuestRequirement `json:"requirement"`
	Reward      QuestReward      `json:"reward"`
	Progress    int              `json:"progress"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	AIBatchID   *int64           `json:"ai_batch_id,omitempty"`
}

// DailyResult is one calendar day's daily-challenge state.
type DailyResult struct {
	Day         string     `json:"day"` // YYYY-MM-DD
	QuestionIDs []int64    `json:"question_ids"`
	Answered    int        `json:"answered"`
	Correct     int        `json:"correct"`
	ElapsedMS   int        `json:"elapsed_ms"`
	XPEarned    int        `json:"xp_earned"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// AIBatch is one batch content-generation call, recorded for audit/replay
// per ARCHITECTURE.md "AI content generation". Skill/Difficulty are unset
// for story batches.
type AIBatch struct {
	ID         int64           `json:"id"`
	Kind       string          `json:"kind"` // word_problems|logic|story
	Skill      *string         `json:"skill,omitempty"`
	Difficulty *int            `json:"difficulty,omitempty"`
	Model      string          `json:"model"`
	Prompt     string          `json:"prompt"`
	Raw        json.RawMessage `json:"raw"`
	Accepted   int             `json:"accepted"`
	Rejected   int             `json:"rejected"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Settings is the single-row (id=1) app configuration.
type Settings struct {
	ID                int            `json:"id"`
	DailyCount        int            `json:"daily_count"`
	LevelOverride     map[string]int `json:"level_override"`
	MinutesPerCorrect int            `json:"minutes_per_correct"`
	ClipChance        int            `json:"clip_chance"`      // 1-in-N chance per answer
	ClipSessionCap    int            `json:"clip_session_cap"` // max clips per session
	UpdatedAt         time.Time      `json:"updated_at"`
}

// ScreenTimeReset is one row of screen-time redemption history: a snapshot
// of the dial at the moment it was reset -- either by a parent (reason
// "manual") or automatically at the first use of a new local day (reason
// "daily"). Day is the device-local YYYY-MM-DD the reset applies to; nil for
// historical manual resets recorded before this column existed.
type ScreenTimeReset struct {
	ID              int64     `json:"id"`
	ResetAt         time.Time `json:"reset_at"`
	MinutesRedeemed int       `json:"minutes_redeemed"`
	CorrectsCounted int       `json:"corrects_counted"`
	Reason          string    `json:"reason"`
	Day             *string   `json:"day,omitempty"`
}
