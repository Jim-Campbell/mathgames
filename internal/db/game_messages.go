package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jimgcampbell/mathgames/internal/game"
)

func (d *DB) InsertMessage(ctx context.Context, m *game.Message) error {
	var contextJSON any
	if m.Context != nil {
		b, err := json.Marshal(m.Context)
		if err != nil {
			return fmt.Errorf("marshal message context: %w", err)
		}
		contextJSON = b
	}
	err := d.pool.QueryRow(ctx, `
		INSERT INTO messages (kind, body, context)
		VALUES ($1,$2,$3)
		RETURNING id, created_at`,
		m.Kind, m.Body, contextJSON).
		Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

func (d *DB) ListMessages(ctx context.Context) ([]game.Message, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, kind, body, context, emailed, email_error, read_at, created_at
		FROM messages ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var out []game.Message
	for rows.Next() {
		var m game.Message
		var contextJSON []byte
		var emailError *string
		if err := rows.Scan(&m.ID, &m.Kind, &m.Body, &contextJSON, &m.Emailed, &emailError, &m.ReadAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if len(contextJSON) > 0 {
			if err := json.Unmarshal(contextJSON, &m.Context); err != nil {
				return nil, fmt.Errorf("unmarshal message context: %w", err)
			}
		}
		if emailError != nil {
			m.EmailError = *emailError
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) UpdateMessageEmailStatus(ctx context.Context, id int64, emailed bool, emailError string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE messages SET emailed = $2, email_error = $3 WHERE id = $1`,
		id, emailed, nullableString(emailError))
	if err != nil {
		return fmt.Errorf("update message email status: %w", err)
	}
	return nil
}

// MarkMessageRead sets read_at only if not already set, so re-marking keeps
// the original read time.
func (d *DB) MarkMessageRead(ctx context.Context, id int64) error {
	ct, err := d.pool.Exec(ctx, `
		UPDATE messages SET read_at = COALESCE(read_at, NOW()) WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark message read: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found: message")
	}
	return nil
}

func (d *DB) CountUnreadMessages(ctx context.Context) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE read_at IS NULL`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unread messages: %w", err)
	}
	return count, nil
}

func (d *DB) CountMessagesSince(ctx context.Context, t time.Time) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE created_at >= $1`, t).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count messages since: %w", err)
	}
	return count, nil
}
