package db

import (
	"context"
	"fmt"

	"github.com/jimgcampbell/mathgames/internal/game"
)

// SeedSkillState ensures every registered skill has a skill_state row,
// leaving existing rows (and their progress) untouched. The registry is
// authoritative, so adding a skill in code needs no migration.
//
// This lives in internal/db (not internal/game) because internal/db also
// implements game.Store — a one-directional db -> game import is fine, but
// game must never import db, so the seeding function that needs *DB moved
// here.
func SeedSkillState(ctx context.Context, database *DB, skills []game.Skill) error {
	for _, s := range skills {
		if _, err := database.Pool().Exec(ctx,
			`INSERT INTO skill_state (skill) VALUES ($1) ON CONFLICT DO NOTHING`,
			s.Slug); err != nil {
			return fmt.Errorf("seed skill_state for %s: %w", s.Slug, err)
		}
	}
	return nil
}
