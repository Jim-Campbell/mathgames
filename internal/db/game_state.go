package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jimgcampbell/mathgames/internal/game"
)

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// ---- attempts ----

// nullableString turns an empty string into SQL NULL, for optional TEXT
// columns like attempts.event.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (d *DB) InsertAttempt(ctx context.Context, a *game.Attempt) error {
	err := d.pool.QueryRow(ctx, `
		INSERT INTO attempts (session_id, question_id, skill, difficulty, given, correct, elapsed_ms, xp_earned, streak_after, level_after, event)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at`,
		a.SessionID, a.QuestionID, a.Skill, a.Difficulty, nullableRaw(a.Given), a.Correct, a.ElapsedMS, a.XPEarned, a.StreakAfter, a.LevelAfter, nullableString(a.Event)).
		Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert attempt: %w", err)
	}
	return nil
}

// AttemptsSinceLastEvent counts attempts newer than the most recent attempt
// whose event fired (or every attempt, if none has), for the random-event
// cooldown.
func (d *DB) AttemptsSinceLastEvent(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM attempts
		WHERE id > COALESCE((SELECT MAX(id) FROM attempts WHERE event IS NOT NULL), 0)`).
		Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("attempts since last event: %w", err)
	}
	return count, nil
}

func (d *DB) HasAttempt(ctx context.Context, sessionID, questionID int64) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM attempts WHERE session_id = $1 AND question_id = $2)`,
		sessionID, questionID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has attempt: %w", err)
	}
	return exists, nil
}

func (d *DB) ListAttempts(ctx context.Context, since time.Time) ([]game.Attempt, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, session_id, question_id, skill, difficulty, given, correct, elapsed_ms, xp_earned, streak_after, level_after, created_at
		FROM attempts WHERE created_at >= $1 ORDER BY created_at`, since)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}
	defer rows.Close()

	var out []game.Attempt
	for rows.Next() {
		var a game.Attempt
		var given []byte
		if err := rows.Scan(&a.ID, &a.SessionID, &a.QuestionID, &a.Skill, &a.Difficulty, &given,
			&a.Correct, &a.ElapsedMS, &a.XPEarned, &a.StreakAfter, &a.LevelAfter, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attempt: %w", err)
		}
		a.Given = json.RawMessage(given)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (d *DB) RecentAttemptCounts(ctx context.Context, since time.Time) (map[string]int, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT skill, COUNT(*) FROM attempts WHERE created_at >= $1 GROUP BY skill`, since)
	if err != nil {
		return nil, fmt.Errorf("recent attempt counts: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var skill string
		var count int
		if err := rows.Scan(&skill, &count); err != nil {
			return nil, fmt.Errorf("scan attempt count: %w", err)
		}
		out[skill] = count
	}
	return out, rows.Err()
}

// ---- skill state ----

func scanSkillState(row rowScanner) (*game.SkillState, error) {
	var s game.SkillState
	if err := row.Scan(&s.Skill, &s.Level, &s.XP, &s.Streak, &s.WrongRun, &s.WindowTotal, &s.WindowCorrect, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

const skillStateCols = `skill, level, xp, streak, wrong_run, window_total, window_correct, updated_at`

func (d *DB) GetSkillState(ctx context.Context, skill string) (*game.SkillState, error) {
	row := d.pool.QueryRow(ctx, `SELECT `+skillStateCols+` FROM skill_state WHERE skill = $1`, skill)
	s, err := scanSkillState(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get skill state: %w", err)
	}
	return s, nil
}

func (d *DB) ListSkillStates(ctx context.Context) ([]game.SkillState, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+skillStateCols+` FROM skill_state ORDER BY skill`)
	if err != nil {
		return nil, fmt.Errorf("list skill states: %w", err)
	}
	defer rows.Close()

	var out []game.SkillState
	for rows.Next() {
		s, err := scanSkillState(rows)
		if err != nil {
			return nil, fmt.Errorf("scan skill state: %w", err)
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (d *DB) UpdateSkillState(ctx context.Context, s *game.SkillState) error {
	err := d.pool.QueryRow(ctx, `
		INSERT INTO skill_state (skill, level, xp, streak, wrong_run, window_total, window_correct, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (skill) DO UPDATE SET
			level = EXCLUDED.level, xp = EXCLUDED.xp, streak = EXCLUDED.streak,
			wrong_run = EXCLUDED.wrong_run, window_total = EXCLUDED.window_total,
			window_correct = EXCLUDED.window_correct, updated_at = NOW()
		RETURNING updated_at`,
		s.Skill, s.Level, s.XP, s.Streak, s.WrongRun, s.WindowTotal, s.WindowCorrect).
		Scan(&s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update skill state: %w", err)
	}
	return nil
}

// ---- sessions ----

func (d *DB) CreateSession(ctx context.Context, s *game.Session) error {
	err := d.pool.QueryRow(ctx, `
		INSERT INTO sessions (mode) VALUES ($1) RETURNING id, started_at`, s.Mode).
		Scan(&s.ID, &s.StartedAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (d *DB) EndSession(ctx context.Context, id int64) error {
	ct, err := d.pool.Exec(ctx, `UPDATE sessions SET ended_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found: session")
	}
	return nil
}

func (d *DB) GetSession(ctx context.Context, id int64) (*game.Session, error) {
	var s game.Session
	err := d.pool.QueryRow(ctx, `SELECT id, mode, started_at, ended_at FROM sessions WHERE id = $1`, id).
		Scan(&s.ID, &s.Mode, &s.StartedAt, &s.EndedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &s, nil
}

// ---- unlocks ----

func (d *DB) ListUnlocks(ctx context.Context) ([]game.Unlock, error) {
	rows, err := d.pool.Query(ctx, `SELECT id, kind, ref, source, created_at FROM unlocks ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list unlocks: %w", err)
	}
	defer rows.Close()

	var out []game.Unlock
	for rows.Next() {
		var u game.Unlock
		if err := rows.Scan(&u.ID, &u.Kind, &u.Ref, &u.Source, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan unlock: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// InsertUnlock is idempotent (UNIQUE(kind,ref)); inserted is false when the
// row already existed.
func (d *DB) InsertUnlock(ctx context.Context, u *game.Unlock) (bool, error) {
	err := d.pool.QueryRow(ctx, `
		INSERT INTO unlocks (kind, ref, source) VALUES ($1,$2,$3)
		ON CONFLICT (kind, ref) DO NOTHING
		RETURNING id, created_at`, u.Kind, u.Ref, u.Source).
		Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, fmt.Errorf("insert unlock: %w", err)
	}
	return true, nil
}

func (d *DB) DeleteUnlocks(ctx context.Context, kind string, refs []string) error {
	if len(refs) == 0 {
		return nil
	}
	_, err := d.pool.Exec(ctx, `DELETE FROM unlocks WHERE kind = $1 AND ref = ANY($2)`, kind, refs)
	if err != nil {
		return fmt.Errorf("delete unlocks: %w", err)
	}
	return nil
}

// Wish runs the whole grant in one transaction so a dragon-ball count check
// can never race against a concurrent wish leaving balls consumed without a
// fighter granted (or vice versa) -- mirrors ~/projects/food's internal/db
// pattern of wrapping a single conceptual operation in pool.Begin.
func (d *DB) Wish(ctx context.Context, fighterRef, bonusSkill string, bonusXP int64) (ballCount int, alreadyUnlocked bool, err error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("begin wish tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM unlocks WHERE kind = 'dragon_ball'`).Scan(&ballCount); err != nil {
		return 0, false, fmt.Errorf("count dragon balls: %w", err)
	}
	if ballCount != 7 {
		return ballCount, false, nil
	}

	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM unlocks WHERE kind = 'fighter' AND ref = $1)`, fighterRef).
		Scan(&alreadyUnlocked); err != nil {
		return ballCount, false, fmt.Errorf("check fighter unlock: %w", err)
	}
	if alreadyUnlocked {
		return ballCount, true, nil
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO unlocks (kind, ref, source) VALUES ('fighter', $1, 'wish')`, fighterRef); err != nil {
		return ballCount, false, fmt.Errorf("insert wish fighter: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM unlocks WHERE kind = 'dragon_ball'`); err != nil {
		return ballCount, false, fmt.Errorf("delete dragon balls: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_state (skill, xp) VALUES ($1, $2)
		ON CONFLICT (skill) DO UPDATE SET xp = skill_state.xp + EXCLUDED.xp, updated_at = NOW()`,
		bonusSkill, bonusXP); err != nil {
		return ballCount, false, fmt.Errorf("credit wish xp: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ballCount, false, fmt.Errorf("commit wish: %w", err)
	}
	return ballCount, false, nil
}

// ---- quests ----

func scanQuestChapter(row rowScanner) (*game.QuestChapter, error) {
	var ch game.QuestChapter
	var reqJSON, rewardJSON []byte
	if err := row.Scan(&ch.ID, &ch.Saga, &ch.Chapter, &ch.Title, &ch.Story, &reqJSON, &rewardJSON,
		&ch.Progress, &ch.CompletedAt, &ch.AIBatchID); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(reqJSON, &ch.Requirement); err != nil {
		return nil, fmt.Errorf("unmarshal requirement: %w", err)
	}
	if err := json.Unmarshal(rewardJSON, &ch.Reward); err != nil {
		return nil, fmt.Errorf("unmarshal reward: %w", err)
	}
	return &ch, nil
}

const questChapterCols = `id, saga, chapter, title, story, requirement, reward, progress, completed_at, ai_batch_id`

func (d *DB) ListQuestChapters(ctx context.Context) ([]game.QuestChapter, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+questChapterCols+` FROM quest_chapters ORDER BY saga, chapter`)
	if err != nil {
		return nil, fmt.Errorf("list quest chapters: %w", err)
	}
	defer rows.Close()

	var out []game.QuestChapter
	for rows.Next() {
		ch, err := scanQuestChapter(rows)
		if err != nil {
			return nil, fmt.Errorf("scan quest chapter: %w", err)
		}
		out = append(out, *ch)
	}
	return out, rows.Err()
}

func (d *DB) GetQuestChapter(ctx context.Context, saga string, chapter int) (*game.QuestChapter, error) {
	row := d.pool.QueryRow(ctx, `SELECT `+questChapterCols+` FROM quest_chapters WHERE saga = $1 AND chapter = $2`, saga, chapter)
	ch, err := scanQuestChapter(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get quest chapter: %w", err)
	}
	return ch, nil
}

func (d *DB) UpdateQuestProgress(ctx context.Context, id int64, progress int, completedAt *time.Time) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE quest_chapters SET progress = $2, completed_at = $3 WHERE id = $1`,
		id, progress, completedAt)
	if err != nil {
		return fmt.Errorf("update quest progress: %w", err)
	}
	return nil
}

// ---- daily ----

func scanDailyResult(row rowScanner) (*game.DailyResult, error) {
	var d game.DailyResult
	var day time.Time
	if err := row.Scan(&day, &d.QuestionIDs, &d.Answered, &d.Correct, &d.ElapsedMS, &d.XPEarned, &d.CompletedAt); err != nil {
		return nil, err
	}
	d.Day = day.Format("2006-01-02")
	return &d, nil
}

const dailyResultCols = `day, question_ids, answered, correct, elapsed_ms, xp_earned, completed_at`

func (d *DB) GetDailyResult(ctx context.Context, day string) (*game.DailyResult, error) {
	row := d.pool.QueryRow(ctx, `SELECT `+dailyResultCols+` FROM daily_results WHERE day = $1::date`, day)
	res, err := scanDailyResult(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get daily result: %w", err)
	}
	return res, nil
}

func (d *DB) CreateDailyResult(ctx context.Context, res *game.DailyResult) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO daily_results (day, question_ids, answered, correct, elapsed_ms, xp_earned, completed_at)
		VALUES ($1::date,$2,$3,$4,$5,$6,$7)`,
		res.Day, res.QuestionIDs, res.Answered, res.Correct, res.ElapsedMS, res.XPEarned, res.CompletedAt)
	if err != nil {
		return fmt.Errorf("create daily result: %w", err)
	}
	return nil
}

func (d *DB) UpdateDailyResult(ctx context.Context, res *game.DailyResult) error {
	ct, err := d.pool.Exec(ctx, `
		UPDATE daily_results SET answered = $2, correct = $3, elapsed_ms = $4, xp_earned = $5, completed_at = $6
		WHERE day = $1::date`,
		res.Day, res.Answered, res.Correct, res.ElapsedMS, res.XPEarned, res.CompletedAt)
	if err != nil {
		return fmt.Errorf("update daily result: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found: daily result")
	}
	return nil
}

func (d *DB) ListDailyResults(ctx context.Context, sinceDay string) ([]game.DailyResult, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+dailyResultCols+` FROM daily_results WHERE day >= $1::date ORDER BY day`, sinceDay)
	if err != nil {
		return nil, fmt.Errorf("list daily results: %w", err)
	}
	defer rows.Close()

	var out []game.DailyResult
	for rows.Next() {
		res, err := scanDailyResult(rows)
		if err != nil {
			return nil, fmt.Errorf("scan daily result: %w", err)
		}
		out = append(out, *res)
	}
	return out, rows.Err()
}

// ---- settings ----

func (d *DB) GetSettings(ctx context.Context) (*game.Settings, error) {
	var s game.Settings
	var overrideJSON []byte
	err := d.pool.QueryRow(ctx, `SELECT id, daily_count, level_override, minutes_per_correct, clip_chance, clip_session_cap, updated_at FROM settings WHERE id = 1`).
		Scan(&s.ID, &s.DailyCount, &overrideJSON, &s.MinutesPerCorrect, &s.ClipChance, &s.ClipSessionCap, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	if err := json.Unmarshal(overrideJSON, &s.LevelOverride); err != nil {
		return nil, fmt.Errorf("unmarshal level_override: %w", err)
	}
	return &s, nil
}

func (d *DB) UpdateSettings(ctx context.Context, s *game.Settings) error {
	override := s.LevelOverride
	if override == nil {
		override = map[string]int{}
	}
	overrideJSON, err := json.Marshal(override)
	if err != nil {
		return fmt.Errorf("marshal level_override: %w", err)
	}
	_, err = d.pool.Exec(ctx, `
		UPDATE settings SET daily_count = $1, level_override = $2, minutes_per_correct = $3,
			clip_chance = $4, clip_session_cap = $5, updated_at = NOW() WHERE id = 1`,
		s.DailyCount, overrideJSON, s.MinutesPerCorrect, s.ClipChance, s.ClipSessionCap)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	return nil
}

// ---- screen time ----

func (d *DB) CountCorrectsSince(ctx context.Context, since *time.Time) (int, error) {
	var count int
	var err error
	if since == nil {
		err = d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM attempts WHERE correct`).Scan(&count)
	} else {
		err = d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM attempts WHERE correct AND created_at >= $1`, *since).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("count corrects since: %w", err)
	}
	return count, nil
}

func (d *DB) InsertScreenTimeReset(ctx context.Context, r *game.ScreenTimeReset) error {
	day, err := parseOptionalDay(r.Day)
	if err != nil {
		return err
	}
	err = d.pool.QueryRow(ctx, `
		INSERT INTO screen_time_resets (minutes_redeemed, corrects_counted, reason, day)
		VALUES ($1,$2,$3,$4)
		RETURNING id, reset_at`,
		r.MinutesRedeemed, r.CorrectsCounted, r.Reason, day).
		Scan(&r.ID, &r.ResetAt)
	if err != nil {
		return fmt.Errorf("insert screen time reset: %w", err)
	}
	return nil
}

// InsertDailyResetIfNew inserts a reason='daily' row for localDay unless one
// already exists, relying on the screen_time_daily_uniq partial unique
// index + ON CONFLICT DO NOTHING to stay race-safe under concurrent calls.
func (d *DB) InsertDailyResetIfNew(ctx context.Context, localDay string, resetAt time.Time, minutesRedeemed, correctsCounted int) (bool, error) {
	ct, err := d.pool.Exec(ctx, `
		INSERT INTO screen_time_resets (reset_at, minutes_redeemed, corrects_counted, reason, day)
		VALUES ($1,$2,$3,'daily',$4::date)
		ON CONFLICT (day) WHERE reason = 'daily' DO NOTHING`,
		resetAt, minutesRedeemed, correctsCounted, localDay)
	if err != nil {
		return false, fmt.Errorf("insert daily reset: %w", err)
	}
	return ct.RowsAffected() > 0, nil
}

func (d *DB) LastScreenTimeReset(ctx context.Context) (*game.ScreenTimeReset, error) {
	r, err := scanScreenTimeReset(d.pool.QueryRow(ctx, `
		SELECT id, reset_at, minutes_redeemed, corrects_counted, reason, day
		FROM screen_time_resets ORDER BY reset_at DESC LIMIT 1`))
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("last screen time reset: %w", err)
	}
	return r, nil
}

func (d *DB) ListScreenTimeResets(ctx context.Context) ([]game.ScreenTimeReset, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, reset_at, minutes_redeemed, corrects_counted, reason, day
		FROM screen_time_resets ORDER BY reset_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list screen time resets: %w", err)
	}
	defer rows.Close()

	var out []game.ScreenTimeReset
	for rows.Next() {
		r, err := scanScreenTimeReset(rows)
		if err != nil {
			return nil, fmt.Errorf("scan screen time reset: %w", err)
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func scanScreenTimeReset(row rowScanner) (*game.ScreenTimeReset, error) {
	var r game.ScreenTimeReset
	var day *time.Time
	if err := row.Scan(&r.ID, &r.ResetAt, &r.MinutesRedeemed, &r.CorrectsCounted, &r.Reason, &day); err != nil {
		return nil, err
	}
	if day != nil {
		s := day.Format("2006-01-02")
		r.Day = &s
	}
	return &r, nil
}

// parseOptionalDay converts a "YYYY-MM-DD" pointer (nil means no day) into a
// time.Time pointer suitable for a DATE column parameter.
func parseOptionalDay(day *string) (*time.Time, error) {
	if day == nil {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *day)
	if err != nil {
		return nil, fmt.Errorf("parse day: %w", err)
	}
	return &t, nil
}

// ---- admin ----

// ResetProgress wipes all player progress in one transaction. attempts must
// be deleted before sessions (attempts.session_id references sessions(id)).
// The question bank, quest_chapters story text, and ai_batches are left
// alone — only progress against that content resets.
func (d *DB) ResetProgress(ctx context.Context) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reset tx: %w", err)
	}
	defer tx.Rollback(ctx)

	stmts := []string{
		`DELETE FROM attempts`,
		`DELETE FROM sessions`,
		`DELETE FROM unlocks`,
		`DELETE FROM daily_results`,
		`UPDATE skill_state SET level = 1, xp = 0, streak = 0, wrong_run = 0, window_total = 0, window_correct = 0, updated_at = NOW()`,
		`UPDATE quest_chapters SET progress = 0, completed_at = NULL`,
		`UPDATE questions SET times_served = 0`,
		`UPDATE settings SET level_override = '{}'::jsonb, updated_at = NOW() WHERE id = 1`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("reset progress: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reset: %w", err)
	}
	return nil
}
