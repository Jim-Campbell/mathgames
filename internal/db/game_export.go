package db

import (
	"context"
	"fmt"
	"time"
)

// ExportAll dumps every game table, keyed by table name, for GET
// /api/export (the backup story — Render disks are ephemeral). Kept simple:
// phase 4+ can refine shape/streaming if the DB grows large.
func (d *DB) ExportAll(ctx context.Context) (map[string]any, error) {
	out := map[string]any{"exported_at": time.Now().UTC()}

	skillStates, err := d.ListSkillStates(ctx)
	if err != nil {
		return nil, err
	}
	out["skill_state"] = skillStates

	questions, err := d.ListQuestions(ctx, "", "", nil)
	if err != nil {
		return nil, err
	}
	out["questions"] = questions

	unlocks, err := d.ListUnlocks(ctx)
	if err != nil {
		return nil, err
	}
	out["unlocks"] = unlocks

	chapters, err := d.ListQuestChapters(ctx)
	if err != nil {
		return nil, err
	}
	out["quest_chapters"] = chapters

	dailyResults, err := d.ListDailyResults(ctx, "2000-01-01")
	if err != nil {
		return nil, err
	}
	out["daily_results"] = dailyResults

	settings, err := d.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	out["settings"] = settings

	sessions, err := d.listAllSessions(ctx)
	if err != nil {
		return nil, err
	}
	out["sessions"] = sessions

	attempts, err := d.listAllAttempts(ctx)
	if err != nil {
		return nil, err
	}
	out["attempts"] = attempts

	return out, nil
}

func (d *DB) listAllSessions(ctx context.Context) ([]map[string]any, error) {
	rows, err := d.pool.Query(ctx, `SELECT id, mode, started_at, ended_at FROM sessions ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id int64
		var mode string
		var startedAt time.Time
		var endedAt *time.Time
		if err := rows.Scan(&id, &mode, &startedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		out = append(out, map[string]any{"id": id, "mode": mode, "started_at": startedAt, "ended_at": endedAt})
	}
	return out, rows.Err()
}

func (d *DB) listAllAttempts(ctx context.Context) ([]map[string]any, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, session_id, question_id, skill, difficulty, given, correct, elapsed_ms, xp_earned, streak_after, level_after, created_at
		FROM attempts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, sessionID, questionID int64
		var skill string
		var difficulty, elapsedMS, xpEarned, streakAfter, levelAfter int
		var given []byte
		var correct bool
		var createdAt time.Time
		if err := rows.Scan(&id, &sessionID, &questionID, &skill, &difficulty, &given, &correct,
			&elapsedMS, &xpEarned, &streakAfter, &levelAfter, &createdAt); err != nil {
			return nil, fmt.Errorf("scan attempt: %w", err)
		}
		out = append(out, map[string]any{
			"id": id, "session_id": sessionID, "question_id": questionID, "skill": skill,
			"difficulty": difficulty, "given": string(given), "correct": correct,
			"elapsed_ms": elapsedMS, "xp_earned": xpEarned, "streak_after": streakAfter,
			"level_after": levelAfter, "created_at": createdAt,
		})
	}
	return out, rows.Err()
}
