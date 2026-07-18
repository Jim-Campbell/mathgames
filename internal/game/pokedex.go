package game

// Pokedex is the code-defined catalog (~20 entries) of XP-threshold,
// arc-reward, and catch-only Pokémon. Slugs are stable identifiers used in
// unlocks.ref and quest_chapters.reward.pokemon; never renumber/reslug an
// existing entry once shipped.
//
// The XP thresholds below are unchanged from the original catalog (this is
// a pure reskin) — a monotonic table with a signature moment at 9001 (the
// old high-water-mark slot, now Charizard) and a top anchor at 250,000.
var Pokedex = []Pokemon{
	{Slug: "pidgey", Name: "Pidgey", Rarity: RarityCommon, Condition: UnlockCondition{Type: "xp", XP: 500}},
	{Slug: "rattata", Name: "Rattata", Rarity: RarityCommon, Condition: UnlockCondition{Type: "xp", XP: 1000}},
	{Slug: "caterpie", Name: "Caterpie", Rarity: RarityCommon, Condition: UnlockCondition{Type: "xp", XP: 2000}},
	{Slug: "eevee", Name: "Eevee", Rarity: RarityCommon, Condition: UnlockCondition{Type: "xp", XP: 3000}},
	{Slug: "growlithe", Name: "Growlithe", Rarity: RarityRare, Condition: UnlockCondition{Type: "xp", XP: 4000}},
	{Slug: "machop", Name: "Machop", Rarity: RarityRare, Condition: UnlockCondition{Type: "xp", XP: 5500}},
	{Slug: "geodude", Name: "Geodude", Rarity: RarityRare, Condition: UnlockCondition{Type: "xp", XP: 7000}},
	{Slug: "charizard", Name: "Charizard", Rarity: RarityEpic, Condition: UnlockCondition{Type: "xp", XP: 9001}}, // the signature moment
	{Slug: "psyduck", Name: "Psyduck", Rarity: RarityRare, Condition: UnlockCondition{Type: "xp", XP: 12000}},
	{Slug: "gyarados", Name: "Gyarados", Rarity: RarityEpic, Condition: UnlockCondition{Type: "xp", XP: 15000}},
	{Slug: "gengar", Name: "Gengar", Rarity: RarityEpic, Condition: UnlockCondition{Type: "xp", XP: 25000}},
	{Slug: "snorlax", Name: "Snorlax", Rarity: RarityEpic, Condition: UnlockCondition{Type: "xp", XP: 40000}},
	{Slug: "blastoise", Name: "Blastoise", Rarity: RarityEpic, Condition: UnlockCondition{Type: "xp", XP: 55000}},
	{Slug: "venusaur", Name: "Venusaur", Rarity: RarityEpic, Condition: UnlockCondition{Type: "xp", XP: 75000}},
	{Slug: "dragonite", Name: "Dragonite", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "xp", XP: 100000}},
	{Slug: "articuno", Name: "Articuno", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "xp", XP: 140000}},
	{Slug: "zapdos", Name: "Zapdos", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "xp", XP: 180000}},
	{Slug: "moltres", Name: "Moltres", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "xp", XP: 220000}},
	{Slug: "mewtwo", Name: "Mewtwo", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "xp", XP: 250000}},

	// Arc-reward Pokémon: strong Pokémon awarded for beating each gym/region arc.
	{Slug: "onix", Name: "Onix", Rarity: RarityEpic, Condition: UnlockCondition{Type: "saga", Saga: "cerulean", Chapter: 4}},
	{Slug: "alakazam", Name: "Alakazam", Rarity: RarityEpic, Condition: UnlockCondition{Type: "saga", Saga: "celadon", Chapter: 4}},
	{Slug: "lapras", Name: "Lapras", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "saga", Saga: "cinnabar", Chapter: 4}},

	// Catch-only: never earned by threshold/saga, only via POST /api/catch
	// with the Master Ball.
	{Slug: "mew", Name: "Mew", Rarity: RarityLegendary, Condition: UnlockCondition{Type: "catch_only"}},
}

// StreakRibbons are daily-challenge calendar-streak unlocks. They aren't
// Pokémon (kind="ribbon" per the unlocks schema), so they live in a
// separate small table rather than the Pokedex catalog.
var StreakRibbons = []struct {
	Days int
	Ref  string
	Name string
}{
	{Days: 3, Ref: "streak-3", Name: "3-Day Streak"},
	{Days: 7, Ref: "streak-7", Name: "7-Day Streak"},
	{Days: 14, Ref: "streak-14", Name: "14-Day Streak"},
	{Days: 30, Ref: "streak-30", Name: "30-Day Streak"},
}

// PokemonBySlug looks up a catalog entry by slug.
func PokemonBySlug(slug string) (Pokemon, bool) {
	for _, p := range Pokedex {
		if p.Slug == slug {
			return p, true
		}
	}
	return Pokemon{}, false
}

// DetectUnlocks returns the Pokémon/ribbons newly earned by an XP change
// from oldXP to newXP (XP only ever goes up, but the function doesn't
// assume that), plus any saga completions newly true and any streak-day
// ribbons newly reached. already is keyed by "kind:ref" (e.g.
// "pokemon:charizard", "ribbon:streak-7") for what's already been unlocked
// in the DB, so a threshold already crossed on a prior attempt is never
// re-reported — including when a single jump crosses more than one
// threshold at once (e.g. xp 400 -> 3000 crosses both Pidgey@500 and
// Rattata@1000; both are returned).
func DetectUnlocks(oldXP, newXP int64, dailyStreak int, sagaCompletions map[string]bool, already map[string]bool) []Pokemon {
	var out []Pokemon

	for _, p := range Pokedex {
		key := UnlockPokemon + ":" + p.Slug
		if already[key] {
			continue
		}
		switch p.Condition.Type {
		case "xp":
			if newXP >= p.Condition.XP && oldXP < p.Condition.XP {
				out = append(out, p)
			}
		case "saga":
			if sagaCompletions[p.Condition.Saga] {
				out = append(out, p)
			}
		case "catch_only":
			// Never auto-unlocked.
		}
	}

	for _, r := range StreakRibbons {
		key := UnlockRibbon + ":" + r.Ref
		if already[key] {
			continue
		}
		if dailyStreak >= r.Days {
			out = append(out, Pokemon{Slug: r.Ref, Name: r.Name, Rarity: RarityCommon,
				Condition: UnlockCondition{Type: "streak", StreakDays: r.Days}})
		}
	}

	return out
}
