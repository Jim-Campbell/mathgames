package db

import (
	"context"
	"fmt"

	"github.com/jimgcampbell/mathgames/internal/game"
)

func (d *DB) InsertAIBatch(ctx context.Context, b *game.AIBatch) error {
	err := d.pool.QueryRow(ctx, `
		INSERT INTO ai_batches (kind, skill, difficulty, model, prompt, raw, accepted, rejected)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at`,
		b.Kind, b.Skill, b.Difficulty, b.Model, b.Prompt, nullableRaw(b.Raw), b.Accepted, b.Rejected).
		Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert ai batch: %w", err)
	}
	return nil
}

func (d *DB) UpdateAIBatchCounts(ctx context.Context, id int64, accepted, rejected int) error {
	_, err := d.pool.Exec(ctx, `UPDATE ai_batches SET accepted = $2, rejected = $3 WHERE id = $1`, id, accepted, rejected)
	if err != nil {
		return fmt.Errorf("update ai batch counts: %w", err)
	}
	return nil
}

func (d *DB) UpdateQuestChapterStory(ctx context.Context, id int64, title, story string, aiBatchID int64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE quest_chapters SET title = $2, story = $3, ai_batch_id = $4 WHERE id = $1`,
		id, title, story, aiBatchID)
	if err != nil {
		return fmt.Errorf("update quest chapter story: %w", err)
	}
	return nil
}
