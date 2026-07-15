package game

import "math/rand"

// ClipRoll decides whether an answer triggers a video clip and which one.
// Pure given its inputs. Returns nil when nothing should play. This is a
// separate roll from RollEvent (internal/game/events.go): it runs on every
// answer, correct or wrong, never fires more than sessionCap times per
// session, and never fires when eligible is empty.
func ClipRoll(rng *rand.Rand, correct bool, eligible []Clip, lastPlayedID int64, playsThisSession, sessionCap, chance int) *Clip {
	if playsThisSession >= sessionCap {
		return nil
	}

	var matching []Clip
	for _, c := range eligible {
		if !c.Enabled {
			continue
		}
		if correct && !c.OnCorrect {
			continue
		}
		if !correct && !c.OnWrong {
			continue
		}
		matching = append(matching, c)
	}
	if len(matching) == 0 {
		return nil
	}

	if rng.Intn(chance) != 0 {
		return nil
	}

	// Avoid an immediate repeat, but only when doing so still leaves a
	// choice -- a single eligible clip is allowed to repeat.
	pickFrom := matching
	if len(matching) > 1 {
		var filtered []Clip
		for _, c := range matching {
			if c.ID != lastPlayedID {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) > 0 {
			pickFrom = filtered
		}
	}

	total := 0
	for _, c := range pickFrom {
		total += c.Weight
	}
	r := rng.Intn(total)
	for i := range pickFrom {
		if r < pickFrom[i].Weight {
			return &pickFrom[i]
		}
		r -= pickFrom[i].Weight
	}
	return &pickFrom[len(pickFrom)-1]
}
