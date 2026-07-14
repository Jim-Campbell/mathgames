package game

import (
	"fmt"
	"math/rand"
)

// eventChance is the 1-in-N chance a correct answer triggers an event.
const eventChance = 25

// eventCooldown is the minimum number of attempts (correct or not) since the
// last attempt whose event fired, before another can fire.
const eventCooldown = 10

// Event is a random-event registry entry.
type Event struct {
	Slug    string // "kaioken"
	Name    string // "Kaio-ken ×2!"
	Message string // "WOW — you just DOUBLED your points!"
	Weight  int    // relative pick weight among eligible events (>0)
	XPNum   int    // multiplier numerator   (kaioken: 2)
	XPDen   int    // multiplier denominator (kaioken: 1)
	XPFlat  int    // flat XP bonus added after the multiplier (capsule: 100)

	// Eligible reports whether this event may fire for this attempt.
	// nil means always eligible.
	Eligible func(elapsedMS, difficulty int) bool
}

// Apply multiplies xp by the event's XPNum/XPDen, integer math, multiply
// before divide, then adds the flat bonus.
func (e *Event) Apply(xp int) int { return xp*e.XPNum/e.XPDen + e.XPFlat }

// MultiplierString is the display string for AttemptResult ("×2" or, for
// flat-only events like capsule, "+100 ⚡").
func (e *Event) MultiplierString() string {
	if e.XPNum == e.XPDen {
		return fmt.Sprintf("+%d ⚡", e.XPFlat)
	}
	if e.XPDen == 1 {
		return fmt.Sprintf("×%d", e.XPNum)
	}
	return fmt.Sprintf("×%d/%d", e.XPNum, e.XPDen)
}

// events is the code-defined registry of random events. kaioken stays first
// (events[0]) since existing tests reference it by index.
var events = []Event{
	{
		Slug:    "kaioken",
		Name:    "Kaio-ken ×2!",
		Message: "WOW — you just DOUBLED your points!",
		Weight:  4,
		XPNum:   2,
		XPDen:   1,
	},
	{
		Slug:    "capsule",
		Name:    "Bulma's Capsule!",
		Message: "A capsule pops open — +100 bonus points inside!",
		Weight:  3,
		XPNum:   1,
		XPDen:   1,
		XPFlat:  100,
	},
	{
		Slug:    "elder_kai",
		Name:    "Elder Kai's Ritual!",
		Message: "That took forever… but the power-up is REAL. Points DOUBLED!",
		Weight:  2,
		XPNum:   2,
		XPDen:   1,
		Eligible: func(elapsedMS, difficulty int) bool {
			return elapsedMS > okMS(difficulty)
		},
	},
	{
		Slug:    "ultra_instinct",
		Name:    "ULTRA INSTINCT!",
		Message: "Your body moved on its own — TRIPLE points!",
		Weight:  2,
		XPNum:   3,
		XPDen:   1,
		Eligible: func(elapsedMS, difficulty int) bool {
			return elapsedMS <= fastMS(difficulty)
		},
	},
}

// RollEvent decides whether a correct answer triggers an event. Pure given
// its inputs: rng injected, cooldown passed in. Fires with probability 1 in
// eventChance; never fires when attemptsSinceLast < eventCooldown. Among
// registered events, only those whose Eligible predicate (if any) allows
// elapsedMS/difficulty contribute weight to the pick; if none are eligible,
// nothing fires.
func RollEvent(rng *rand.Rand, attemptsSinceLast, elapsedMS, difficulty int) *Event {
	if attemptsSinceLast < eventCooldown {
		return nil
	}
	if rng.Intn(eventChance) != 0 {
		return nil
	}

	total := 0
	for _, e := range events {
		if e.Eligible == nil || e.Eligible(elapsedMS, difficulty) {
			total += e.Weight
		}
	}
	if total == 0 {
		return nil
	}
	r := rng.Intn(total)
	for i := range events {
		if events[i].Eligible != nil && !events[i].Eligible(elapsedMS, difficulty) {
			continue
		}
		if r < events[i].Weight {
			return &events[i]
		}
		r -= events[i].Weight
	}
	return nil
}
