package game

import (
	"fmt"
	"math/rand"
)

// sequenceDisplay builds a display.sequence with a nil entry at blankIdx.
func sequenceDisplay(terms []int, blankIdx int) *Display {
	seq := make([]*int, len(terms))
	for i, t := range terms {
		if i == blankIdx {
			continue
		}
		v := t
		seq[i] = &v
	}
	return &Display{Sequence: seq}
}

func sequencePayload(terms []int, blankIdx int, ruleDesc string) (Payload, any, string) {
	answer := terms[blankIdx]
	prompt := "What number is missing?"
	exp := fmt.Sprintf("rule: %s — the missing term is %d", ruleDesc, answer)
	return Payload{Kind: KindNumeric, Prompt: prompt, Display: sequenceDisplay(terms, blankIdx)},
		NumericAnswer{Value: answer}, exp
}

// genPatterns implements the patterns level table (ARCHITECTURE.md "Skills
// and difficulty").
func genPatterns(level int, rng *rand.Rand) (payload Payload, answer any, explanation string) {
	switch level {
	case 1: // +k sequences (skip counting)
		k := 2 + rng.Intn(9) // 2-10
		start := 1 + rng.Intn(20)
		terms := arithmeticSeq(start, k, 6)
		return sequencePayload(terms, 5, fmt.Sprintf("add %d each time", k))

	case 2: // -k sequences
		k := 2 + rng.Intn(9)
		start := 100 + rng.Intn(100)
		terms := arithmeticSeq(start, -k, 6)
		return sequencePayload(terms, 5, fmt.Sprintf("subtract %d each time", k))

	case 3: // x2 / x3 sequences
		ratio := []int{2, 3}[rng.Intn(2)]
		start := 1 + rng.Intn(5)
		terms := geometricSeq(start, ratio, 5)
		return sequencePayload(terms, 4, fmt.Sprintf("multiply by %d each time", ratio))

	case 4: // two-step rules (x2 then +1)
		start := 1 + rng.Intn(5)
		terms := make([]int, 5)
		terms[0] = start
		for i := 1; i < 5; i++ {
			terms[i] = terms[i-1]*2 + 1
		}
		return sequencePayload(terms, 4, "multiply by 2, then add 1")

	case 5: // alternating rules (+3, +5, +3, +5, ...)
		d1, d2 := 2+rng.Intn(6), 2+rng.Intn(6)
		for d2 == d1 {
			d2 = 2 + rng.Intn(6)
		}
		start := 1 + rng.Intn(10)
		terms := []int{start}
		diffs := []int{d1, d2}
		for i := 1; i < 6; i++ {
			terms = append(terms, terms[i-1]+diffs[(i-1)%2])
		}
		return sequencePayload(terms, 5, fmt.Sprintf("alternate +%d, +%d", d1, d2))

	case 6: // square or triangle numbers
		var terms []int
		var ruleDesc string
		if rng.Intn(2) == 0 {
			startN := 1 + rng.Intn(3)
			for i := 0; i < 6; i++ {
				n := startN + i
				terms = append(terms, n*n)
			}
			ruleDesc = "square numbers (n×n)"
		} else {
			startN := 1 + rng.Intn(3)
			for i := 0; i < 6; i++ {
				n := startN + i
				terms = append(terms, n*(n+1)/2)
			}
			ruleDesc = "triangle numbers (1+2+...+n)"
		}
		return sequencePayload(terms, 5, ruleDesc)

	case 7: // missing middle term
		k := 2 + rng.Intn(9)
		start := 1 + rng.Intn(20)
		terms := arithmeticSeq(start, k, 6)
		return sequencePayload(terms, 2+rng.Intn(2), fmt.Sprintf("add %d each time", k)) // blank at index 2 or 3

	case 8: // Fibonacci-style
		a, b := 1+rng.Intn(5), 1+rng.Intn(5)
		terms := []int{a, b}
		for i := 2; i < 7; i++ {
			terms = append(terms, terms[i-1]+terms[i-2])
		}
		return sequencePayload(terms, 6, "each term is the sum of the previous two")

	case 9: // mixed rule identification -> mc
		type rule struct {
			desc string
			fn   func(int) int
		}
		k := 2 + rng.Intn(9)
		rules := []rule{
			{fmt.Sprintf("Add %d", k), func(x int) int { return x + k }},
			{fmt.Sprintf("Subtract %d", k), func(x int) int { return x - k }},
			{fmt.Sprintf("Multiply by %d", k), func(x int) int { return x * k }},
		}
		chosen := rng.Intn(len(rules))
		start := 20 + rng.Intn(30)
		terms := []int{start}
		for i := 1; i < 4; i++ {
			terms = append(terms, rules[chosen].fn(terms[i-1]))
		}
		choices := make([]string, len(rules))
		for i, r := range rules {
			choices[i] = r.desc
		}
		prompt := fmt.Sprintf("What is the rule for this pattern? %v", terms)
		exp := fmt.Sprintf("each term follows: %s", rules[chosen].desc)
		return Payload{Kind: KindMC, Prompt: prompt, Choices: choices}, MCAnswer{Index: chosen}, exp

	case 10: // two interleaved sequences
		k1, k2 := 2+rng.Intn(6), 2+rng.Intn(6)
		s1, s2 := 1+rng.Intn(10), 1+rng.Intn(10)
		terms := make([]int, 7)
		for i := 0; i < 7; i++ {
			if i%2 == 0 {
				terms[i] = s1 + (i/2)*k1
			} else {
				terms[i] = s2 + (i/2)*k2
			}
		}
		ruleDesc := fmt.Sprintf("even positions add %d, odd positions add %d", k1, k2)
		return sequencePayload(terms, 6, ruleDesc)

	default:
		return genPatterns(1, rng)
	}
}

func arithmeticSeq(start, step, n int) []int {
	terms := make([]int, n)
	terms[0] = start
	for i := 1; i < n; i++ {
		terms[i] = terms[i-1] + step
	}
	return terms
}

func geometricSeq(start, ratio, n int) []int {
	terms := make([]int, n)
	terms[0] = start
	for i := 1; i < n; i++ {
		terms[i] = terms[i-1] * ratio
	}
	return terms
}
