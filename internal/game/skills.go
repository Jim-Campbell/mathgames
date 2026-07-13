package game

import (
	"context"
	"fmt"

	"github.com/jimgcampbell/mathgames/internal/db"
)

// Source identifies whether a skill's questions come from a deterministic
// template generator or an AI batch-generation pipeline.
type Source string

const (
	SourceTemplate Source = "template"
	SourceAI       Source = "ai"
)

// Skill is a code-defined entry in the skill registry. The DB only stores
// per-skill state (skill_state); the registry itself is authoritative.
type Skill struct {
	Slug   string
	Name   string
	Source Source
}

// Skills is the rev-1 registry, in display order.
var Skills = []Skill{
	{Slug: "multiplication", Name: "Multiplication", Source: SourceTemplate},
	{Slug: "division", Name: "Division", Source: SourceTemplate},
	{Slug: "addsub", Name: "Addition & Subtraction", Source: SourceTemplate},
	{Slug: "fractions", Name: "Fractions", Source: SourceTemplate},
	{Slug: "place_value", Name: "Place Value", Source: SourceTemplate},
	{Slug: "patterns", Name: "Patterns", Source: SourceTemplate},
	{Slug: "word_problems", Name: "Word Problems", Source: SourceAI},
	{Slug: "logic", Name: "Logic Puzzles", Source: SourceAI},
}

// SeedSkillState ensures every registered skill has a skill_state row,
// leaving existing rows (and their progress) untouched. The registry is
// authoritative, so adding a skill in code needs no migration.
func SeedSkillState(ctx context.Context, database *db.DB) error {
	for _, s := range Skills {
		if _, err := database.Pool().Exec(ctx,
			`INSERT INTO skill_state (skill) VALUES ($1) ON CONFLICT DO NOTHING`,
			s.Slug); err != nil {
			return fmt.Errorf("seed skill_state for %s: %w", s.Slug, err)
		}
	}
	return nil
}
