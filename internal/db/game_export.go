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

	batches, err := d.listAllAIBatches(ctx)
	if err != nil {
		return nil, err
	}
	out["ai_batches"] = batches

	resets, err := d.ListScreenTimeResets(ctx)
	if err != nil {
		return nil, err
	}
	out["screen_time_resets"] = resets

	clips, err := d.ListClips(ctx)
	if err != nil {
		return nil, err
	}
	out["clips"] = clips

	clipPlays, err := d.ListClipPlays(ctx, 0)
	if err != nil {
		return nil, err
	}
	out["clip_plays"] = clipPlays

	messages, err := d.ListMessages(ctx)
	if err != nil {
		return nil, err
	}
	out["messages"] = messages

	return out, nil
}

func (d *DB) listAllAIBatches(ctx context.Context) ([]map[string]any, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, kind, skill, difficulty, model, prompt, accepted, rejected, created_at
		FROM ai_batches ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list ai batches: %w", err)
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id int64
		var kind, model, prompt string
		var skill *string
		var difficulty *int
		var accepted, rejected int
		var createdAt time.Time
		if err := rows.Scan(&id, &kind, &skill, &difficulty, &model, &prompt, &accepted, &rejected, &createdAt); err != nil {
			return nil, fmt.Errorf("scan ai batch: %w", err)
		}
		out = append(out, map[string]any{
			"id": id, "kind": kind, "skill": skill, "difficulty": difficulty, "model": model,
			"prompt": prompt, "accepted": accepted, "rejected": rejected, "created_at": createdAt,
		})
	}
	return out, rows.Err()
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
