package game

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
