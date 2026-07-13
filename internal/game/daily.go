package game

import (
	"hash/fnv"
	"math/rand"
)

// templateSkillSlugs is the fixed rotation order of the 6 template skills
// (see skills.go); daily.go picks a language-agnostic subset of 4 by index
// rather than importing the registry directly, so it has no dependency on
// skill ordering elsewhere.
var templateSkillSlugs = []string{
	"multiplication", "division", "addsub", "fractions", "place_value", "patterns",
}

// dailyPick is one slot in a seeded daily set: a skill and the level to
// serve it at.
type dailyPick struct {
	Skill string
	Level int
}

// SeedDaily deterministically picks the day's question slots: 4 of the 6
// rotating template skills (at their current levels) plus 1 AI skill,
// seeded from FNV-1a(day) so re-opening the app the same day reproduces the
// same set, while different days differ (with very high probability).
//
// templateSkillLevels maps template skill slug -> level to serve.
// aiSkills is the pool of AI skill slugs to choose the 5th slot from.
func SeedDaily(day string, templateSkillLevels map[string]int, aiSkills []string, count int) []dailyPick {
	h := fnv.New64a()
	_, _ = h.Write([]byte(day))
	rng := rand.New(rand.NewSource(int64(h.Sum64())))

	// Rotate which 4 of the 6 template skills play today: shuffle the full
	// slug order and take the first 4 (a full Fisher-Yates shuffle, not just
	// a rotation, so the day seed picks from all 360 ordered 4-of-6 subsets
	// rather than only the 6 contiguous windows a rotation would allow).
	n := len(templateSkillSlugs)
	rotated := append([]string{}, templateSkillSlugs...)
	rng.Shuffle(n, func(i, j int) { rotated[i], rotated[j] = rotated[j], rotated[i] })
	if len(rotated) > 4 {
		rotated = rotated[:4]
	}

	picks := make([]dailyPick, 0, count)
	for _, slug := range rotated {
		if len(picks) >= count {
			break
		}
		picks = append(picks, dailyPick{Skill: slug, Level: templateSkillLevels[slug]})
	}

	if len(aiSkills) > 0 && len(picks) < count {
		ai := aiSkills[rng.Intn(len(aiSkills))]
		picks = append(picks, dailyPick{Skill: ai, Level: templateSkillLevels[ai]})
	}

	return picks
}
