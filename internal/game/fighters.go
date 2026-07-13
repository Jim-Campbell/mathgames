package game

// Fighters is the code-defined catalog (~20 entries) of power-level,
// saga-reward, and wish-only fighters. Slugs are stable identifiers used in
// unlocks.ref and quest_chapters.reward.fighter; never renumber/reslug an
// existing entry once shipped.
//
// The power-level thresholds below are a judgment call filling in a
// monotonic table between the anchors named in ARCHITECTURE.md (Krillin
// 500, Yamcha 1,000, Tien 2,000, Piccolo 4,000, Goku 9,001, Gohan 15,000,
// Vegeta 25,000, ..., Beerus 250,000) with a handful of additional
// real-DBZ-fighter waypoints so the collection doesn't have long dead
// stretches. Worth a sanity-check against how Jim wants pacing to feel.
var Fighters = []Fighter{
	{Slug: "krillin", Name: "Krillin", Rarity: RarityCommon, Condition: UnlockCondition{Type: "power_level", PowerLevel: 500}},
	{Slug: "yamcha", Name: "Yamcha", Rarity: RarityCommon, Condition: UnlockCondition{Type: "power_level", PowerLevel: 1000}},
	{Slug: "tien", Name: "Tien", Rarity: RarityCommon, Condition: UnlockCondition{Type: "power_level", PowerLevel: 2000}},
	{Slug: "chiaotzu", Name: "Chiaotzu", Rarity: RarityCommon, Condition: UnlockCondition{Type: "power_level", PowerLevel: 3000}},
	{Slug: "piccolo", Name: "Piccolo", Rarity: RarityRare, Condition: UnlockCondition{Type: "power_level", PowerLevel: 4000}},
	{Slug: "raditz", Name: "Raditz", Rarity: RarityRare, Condition: UnlockCondition{Type: "power_level", PowerLevel: 5500}},
	{Slug: "nappa", Name: "Nappa", Rarity: RarityRare, Condition: UnlockCondition{Type: "power_level", PowerLevel: 7000}},
	{Slug: "goku", Name: "Goku", Rarity: RarityEpic, Condition: UnlockCondition{Type: "power_level", PowerLevel: 9001}}, // IT'S OVER 9000!
	{Slug: "krillin-2", Name: "Krillin (Trained)", Rarity: RarityRare, Condition: UnlockCondition{Type: "power_level", PowerLevel: 12000}},
	{Slug: "gohan", Name: "Gohan", Rarity: RarityEpic, Condition: UnlockCondition{Type: "power_level", PowerLevel: 15000}},
	{Slug: "vegeta", Name: "Vegeta", Rarity: RarityEpic, Condition: UnlockCondition{Type: "power_level", PowerLevel: 25000}},
	{Slug: "android-17", Name: "Android 17", Rarity: RarityEpic, Condition: UnlockCondition{Type: "power_level", PowerLevel: 40000}},
	{Slug: "android-18", Name: "Android 18", Rarity: RarityEpic, Condition: UnlockCondition{Type: "power_level", PowerLevel: 55000}},
	{Slug: "future-trunks", Name: "Future Trunks", Rarity: RarityEpic, Condition: UnlockCondition{Type: "power_level", PowerLevel: 75000}},
	{Slug: "goku-ssj", Name: "Super Saiyan Goku", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "power_level", PowerLevel: 100000}},
	{Slug: "vegeta-ssj", Name: "Super Saiyan Vegeta", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "power_level", PowerLevel: 140000}},
	{Slug: "gohan-ssj2", Name: "Super Saiyan 2 Gohan", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "power_level", PowerLevel: 180000}},
	{Slug: "goku-ssj3", Name: "Super Saiyan 3 Goku", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "power_level", PowerLevel: 220000}},
	{Slug: "beerus", Name: "Beerus", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "power_level", PowerLevel: 250000}},

	// Saga-reward fighters: villains join when their saga is beaten.
	{Slug: "frieza", Name: "Frieza", Rarity: RarityEpic, Condition: UnlockCondition{Type: "saga", Saga: "namek", Chapter: 4}},
	{Slug: "cell", Name: "Cell", Rarity: RarityEpic, Condition: UnlockCondition{Type: "saga", Saga: "cell", Chapter: 4}},
	{Slug: "majin-buu", Name: "Majin Buu", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "saga", Saga: "buu", Chapter: 4}},

	// Wish-only: never earned by threshold/saga, only via POST /api/wish.
	{Slug: "shenron", Name: "Shenron", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "wish_only"}},
}

// StreakBadges are daily-challenge calendar-streak unlocks. They aren't
// fighters (kind="badge" per the unlocks schema), so they live in a
// separate small table rather than the Fighters catalog.
var StreakBadges = []struct {
	Days int
	Ref  string
	Name string
}{
	{Days: 3, Ref: "streak-3", Name: "3-Day Streak"},
	{Days: 7, Ref: "streak-7", Name: "7-Day Streak"},
	{Days: 14, Ref: "streak-14", Name: "14-Day Streak"},
	{Days: 30, Ref: "streak-30", Name: "30-Day Streak"},
}

// FighterBySlug looks up a catalog entry by slug.
func FighterBySlug(slug string) (Fighter, bool) {
	for _, f := range Fighters {
		if f.Slug == slug {
			return f, true
		}
	}
	return Fighter{}, false
}

// DetectUnlocks returns the fighters/badges newly earned by a power-level
// change from oldPower to newPower (power level only ever goes up, but the
// function doesn't assume that), plus any saga completions newly true and
// any streak-day badges newly reached. already is keyed by "kind:ref" (e.g.
// "fighter:goku", "badge:streak-7") for what's already been unlocked in the
// DB, so a threshold already crossed on a prior attempt is never re-reported
// — including when a single jump crosses more than one threshold at once
// (e.g. power 400 -> 3000 crosses both Krillin@500 and Yamcha@1000; both are
// returned).
func DetectUnlocks(oldPower, newPower int64, dailyStreak int, sagaCompletions map[string]bool, already map[string]bool) []Fighter {
	var out []Fighter

	for _, f := range Fighters {
		key := UnlockFighter + ":" + f.Slug
		if already[key] {
			continue
		}
		switch f.Condition.Type {
		case "power_level":
			if newPower >= f.Condition.PowerLevel && oldPower < f.Condition.PowerLevel {
				out = append(out, f)
			}
		case "saga":
			if sagaCompletions[f.Condition.Saga] {
				out = append(out, f)
			}
		case "wish_only":
			// Never auto-unlocked.
		}
	}

	for _, b := range StreakBadges {
		key := UnlockBadge + ":" + b.Ref
		if already[key] {
			continue
		}
		if dailyStreak >= b.Days {
			out = append(out, Fighter{Slug: b.Ref, Name: b.Name, Rarity: RarityCommon,
				Condition: UnlockCondition{Type: "streak", StreakDays: b.Days}})
		}
	}

	return out
}
