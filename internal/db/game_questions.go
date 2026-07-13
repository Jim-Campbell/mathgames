package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jimgcampbell/mathgames/internal/game"
)

// nullableRaw passes JSONB through pgx as-is; an empty RawMessage becomes
// SQL NULL rather than an empty jsonb value. Mirrors ~/projects/food.
func nullableRaw(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

func scanQuestion(row rowScanner) (*game.Question, error) {
	var q game.Question
	var payload, answer []byte
	var aiModel *string
	var aiBatchID *int64
	if err := row.Scan(&q.ID, &q.Skill, &q.Difficulty, &q.Source, &payload, &answer,
		&q.Explanation, &aiModel, &aiBatchID, &q.TimesServed, &q.Retired, &q.CreatedAt); err != nil {
		return nil, err
	}
	q.Payload = json.RawMessage(payload)
	q.Answer = json.RawMessage(answer)
	q.AIModel = aiModel
	q.AIBatchID = aiBatchID
	return &q, nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

const questionCols = `id, skill, difficulty, source, payload, answer, explanation, ai_model, ai_batch_id, times_served, retired, created_at`

func (d *DB) InsertQuestion(ctx context.Context, q *game.Question) error {
	err := d.pool.QueryRow(ctx, `
		INSERT INTO questions (skill, difficulty, source, payload, answer, explanation, ai_model, ai_batch_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, times_served, retired, created_at`,
		q.Skill, q.Difficulty, q.Source, nullableRaw(q.Payload), nullableRaw(q.Answer), q.Explanation, q.AIModel, q.AIBatchID).
		Scan(&q.ID, &q.TimesServed, &q.Retired, &q.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert question: %w", err)
	}
	return nil
}

func (d *DB) GetQuestion(ctx context.Context, id int64) (*game.Question, error) {
	row := d.pool.QueryRow(ctx, `SELECT `+questionCols+` FROM questions WHERE id = $1`, id)
	q, err := scanQuestion(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	return q, nil
}

// PickAIQuestions picks the least-served non-retired rows at (skill,
// level), falling back to nearest level (±1, ±2, ...) if fewer than n exist
// there. bankLow is true whenever a fallback was needed or the bank still
// came up short of n.
func (d *DB) PickAIQuestions(ctx context.Context, skill string, level, n int) ([]game.Question, bool, error) {
	var out []game.Question
	seen := map[int64]bool{}
	bankLow := false

	for delta := 0; delta <= 9 && len(out) < n; delta++ {
		levels := []int{level - delta, level + delta}
		if delta == 0 {
			levels = []int{level}
		}
		for _, lvl := range levels {
			if lvl < 1 || lvl > 10 || len(out) >= n {
				continue
			}
			if delta > 0 {
				bankLow = true
			}
			need := n - len(out)
			rows, err := d.pool.Query(ctx, `
				SELECT `+questionCols+` FROM questions
				WHERE skill = $1 AND difficulty = $2 AND source = 'ai' AND NOT retired
				ORDER BY times_served, random()
				LIMIT $3`, skill, lvl, need)
			if err != nil {
				return nil, false, fmt.Errorf("pick ai questions: %w", err)
			}
			for rows.Next() {
				q, err := scanQuestion(rows)
				if err != nil {
					rows.Close()
					return nil, false, fmt.Errorf("scan ai question: %w", err)
				}
				if seen[q.ID] {
					continue
				}
				seen[q.ID] = true
				out = append(out, *q)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, false, fmt.Errorf("ai question rows: %w", err)
			}
			rows.Close()
		}
	}
	if len(out) < n {
		bankLow = true
	}
	return out, bankLow, nil
}

func (d *DB) BumpTimesServed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := d.pool.Exec(ctx, `UPDATE questions SET times_served = times_served + 1 WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("bump times served: %w", err)
	}
	return nil
}

func (d *DB) ListQuestions(ctx context.Context, skill, source string, retired *bool) ([]game.Question, error) {
	query := `SELECT ` + questionCols + ` FROM questions WHERE TRUE`
	var args []any
	if skill != "" {
		args = append(args, skill)
		query += fmt.Sprintf(" AND skill = $%d", len(args))
	}
	if source != "" {
		args = append(args, source)
		query += fmt.Sprintf(" AND source = $%d", len(args))
	}
	if retired != nil {
		args = append(args, *retired)
		query += fmt.Sprintf(" AND retired = $%d", len(args))
	}
	query += " ORDER BY created_at DESC"

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	defer rows.Close()

	var out []game.Question
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan question: %w", err)
		}
		out = append(out, *q)
	}
	return out, rows.Err()
}

func (d *DB) SetQuestionRetired(ctx context.Context, id int64, retired bool) error {
	ct, err := d.pool.Exec(ctx, `UPDATE questions SET retired = $2 WHERE id = $1`, id, retired)
	if err != nil {
		return fmt.Errorf("set question retired: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("not found: question")
	}
	return nil
}
