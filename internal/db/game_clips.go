package db

import (
	"context"
	"fmt"

	"github.com/jimgcampbell/mathgames/internal/game"
)

const clipCols = `id, title, r2_key, url, content_type, size_bytes, duration_ms, enabled, on_correct, on_wrong, weight, play_count, created_at`

func scanClip(row rowScanner) (*game.Clip, error) {
	var c game.Clip
	if err := row.Scan(&c.ID, &c.Title, &c.R2Key, &c.URL, &c.ContentType, &c.SizeBytes, &c.DurationMS,
		&c.Enabled, &c.OnCorrect, &c.OnWrong, &c.Weight, &c.PlayCount, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) ListClips(ctx context.Context) ([]game.Clip, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+clipCols+` FROM clips ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list clips: %w", err)
	}
	defer rows.Close()

	var out []game.Clip
	for rows.Next() {
		c, err := scanClip(rows)
		if err != nil {
			return nil, fmt.Errorf("scan clip: %w", err)
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (d *DB) GetClip(ctx context.Context, id int64) (*game.Clip, error) {
	row := d.pool.QueryRow(ctx, `SELECT `+clipCols+` FROM clips WHERE id = $1`, id)
	c, err := scanClip(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get clip: %w", err)
	}
	return c, nil
}

func (d *DB) InsertClip(ctx context.Context, c *game.Clip) error {
	err := d.pool.QueryRow(ctx, `
		INSERT INTO clips (title, r2_key, url, content_type, size_bytes, duration_ms, enabled, on_correct, on_wrong, weight)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, play_count, created_at`,
		c.Title, c.R2Key, c.URL, c.ContentType, c.SizeBytes, c.DurationMS, c.Enabled, c.OnCorrect, c.OnWrong, c.Weight).
		Scan(&c.ID, &c.PlayCount, &c.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert clip: %w", err)
	}
	return nil
}

func (d *DB) UpdateClipConditions(ctx context.Context, id int64, title string, enabled, onCorrect, onWrong bool, weight int) error {
	ct, err := d.pool.Exec(ctx, `
		UPDATE clips SET title = $2, enabled = $3, on_correct = $4, on_wrong = $5, weight = $6 WHERE id = $1`,
		id, title, enabled, onCorrect, onWrong, weight)
	if err != nil {
		return fmt.Errorf("update clip conditions: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found: clip")
	}
	return nil
}

func (d *DB) DeleteClip(ctx context.Context, id int64) error {
	ct, err := d.pool.Exec(ctx, `DELETE FROM clips WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete clip: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found: clip")
	}
	return nil
}

func (d *DB) CountClipPlaysInSession(ctx context.Context, sessionID int64) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM clip_plays cp
		JOIN attempts a ON a.id = cp.attempt_id
		WHERE a.session_id = $1`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count clip plays in session: %w", err)
	}
	return count, nil
}

func (d *DB) LastPlayedClipID(ctx context.Context) (int64, error) {
	var clipID int64
	err := d.pool.QueryRow(ctx, `SELECT clip_id FROM clip_plays ORDER BY id DESC LIMIT 1`).Scan(&clipID)
	if err != nil {
		if isNoRows(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("last played clip id: %w", err)
	}
	return clipID, nil
}

func (d *DB) InsertClipPlay(ctx context.Context, clipID, attemptID int64, trigger string) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin clip play tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO clip_plays (clip_id, attempt_id, trigger) VALUES ($1,$2,$3)`,
		clipID, attemptID, trigger); err != nil {
		return fmt.Errorf("insert clip play: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE clips SET play_count = play_count + 1 WHERE id = $1`, clipID); err != nil {
		return fmt.Errorf("bump clip play count: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit clip play: %w", err)
	}
	return nil
}

// ListClipPlays returns the most recent plays, newest first. limit <= 0
// means no limit (Postgres treats LIMIT NULL as unbounded), for the export.
func (d *DB) ListClipPlays(ctx context.Context, limit int) ([]game.ClipPlayLog, error) {
	var limitArg any
	if limit > 0 {
		limitArg = limit
	}
	rows, err := d.pool.Query(ctx, `
		SELECT cp.id, cp.clip_id, c.title, cp.attempt_id, cp.trigger, cp.played_at
		FROM clip_plays cp
		JOIN clips c ON c.id = cp.clip_id
		ORDER BY cp.played_at DESC
		LIMIT $1`, limitArg)
	if err != nil {
		return nil, fmt.Errorf("list clip plays: %w", err)
	}
	defer rows.Close()

	var out []game.ClipPlayLog
	for rows.Next() {
		var l game.ClipPlayLog
		if err := rows.Scan(&l.ID, &l.ClipID, &l.ClipTitle, &l.AttemptID, &l.Trigger, &l.PlayedAt); err != nil {
			return nil, fmt.Errorf("scan clip play: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
