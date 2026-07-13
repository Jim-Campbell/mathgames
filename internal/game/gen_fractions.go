package game

import (
	"fmt"
	"math/rand"
)

// fractionDenoms is the curated pool of "nice" denominators used across
// fraction levels.
var fractionDenoms = []int{2, 3, 4, 5, 6, 8, 10, 12}

func pickDenom(rng *rand.Rand) int {
	return fractionDenoms[rng.Intn(len(fractionDenoms))]
}

func fracStr(num, den int) string { return fmt.Sprintf("%d/%d", num, den) }

// genFractions implements the fractions level table (ARCHITECTURE.md
// "Skills and difficulty").
func genFractions(level int, rng *rand.Rand) (payload Payload, answer any, explanation string) {
	switch level {
	case 1: // which is bigger, same denominator -> mc
		d := pickDenom(rng)
		if d < 3 {
			d = 4
		}
		a := 1 + rng.Intn(d-1)
		b := 1 + rng.Intn(d-1)
		for b == a {
			b = 1 + rng.Intn(d-1)
		}
		idx := 0
		if b > a {
			idx = 1
		}
		prompt := "Which fraction is larger?"
		exp := fmt.Sprintf("%s vs %s: same denominator, so compare numerators — %d > %d",
			fracStr(a, d), fracStr(b, d), maxInt(a, b), minInt(a, b))
		return Payload{Kind: KindMC, Prompt: prompt, Choices: []string{fracStr(a, d), fracStr(b, d)}},
			MCAnswer{Index: idx}, exp

	case 2: // fraction of a shape -> display.fraction_bar
		parts := []int{4, 6, 8, 10, 12}[rng.Intn(5)]
		shaded := 1 + rng.Intn(parts-1)
		prompt := "What fraction of the shape is shaded?"
		disp := &Display{FractionBar: &FractionBar{Parts: parts, Shaded: shaded}}
		rn, rd := simplify(shaded, parts)
		exp := fmt.Sprintf("%d of %d parts shaded = %s", shaded, parts, fracStr(rn, rd))
		return Payload{Kind: KindFraction, Prompt: prompt, Display: disp},
			FractionAnswer{Num: shaded, Den: parts}, exp

	case 3: // equivalents (2/3 = ?/12) -> numeric (missing numerator)
		d := pickDenom(rng)
		if d < 2 {
			d = 2
		}
		n := 1 + rng.Intn(d-1)
		mult := 2 + rng.Intn(4) // x2..x5
		targetDen := d * mult
		targetNum := n * mult
		prompt := fmt.Sprintf("%s = ?/%d", fracStr(n, d), targetDen)
		exp := fmt.Sprintf("%s = %s (multiply top and bottom by %d)", fracStr(n, d), fracStr(targetNum, targetDen), mult)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: targetNum}, exp

	case 4: // compare unlike denominators -> mc
		d1, d2 := pickDenom(rng), pickDenom(rng)
		for d2 == d1 {
			d2 = pickDenom(rng)
		}
		n1 := 1 + rng.Intn(d1-1)
		n2 := 1 + rng.Intn(d2-1)
		left := n1 * d2
		right := n2 * d1
		idx := 0
		if right > left {
			idx = 1
		}
		prompt := "Which fraction is larger?"
		exp := fmt.Sprintf("%s vs %s: cross-multiply — %d×%d=%d vs %d×%d=%d",
			fracStr(n1, d1), fracStr(n2, d2), n1, d2, left, n2, d1, right)
		return Payload{Kind: KindMC, Prompt: prompt, Choices: []string{fracStr(n1, d1), fracStr(n2, d2)}},
			MCAnswer{Index: idx}, exp

	case 5: // add/sub like denominators -> fraction
		d := pickDenom(rng)
		if d < 3 {
			d = 4
		}
		a := 1 + rng.Intn(d-1)
		if rng.Intn(2) == 0 {
			maxB := d - a
			if maxB < 1 {
				maxB = 1
			}
			b := 1 + rng.Intn(maxB)
			prompt := fmt.Sprintf("%s + %s = ?", fracStr(a, d), fracStr(b, d))
			exp := fmt.Sprintf("%s+%s = %s", fracStr(a, d), fracStr(b, d), fracStr(a+b, d))
			return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: a + b, Den: d}, exp
		}
		b := 1 + rng.Intn(a)
		prompt := fmt.Sprintf("%s − %s = ?", fracStr(a, d), fracStr(b, d))
		exp := fmt.Sprintf("%s-%s = %s", fracStr(a, d), fracStr(b, d), fracStr(a-b, d))
		return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: a - b, Den: d}, exp

	case 6: // simplify to lowest terms -> fraction
		g := 2 + rng.Intn(5) // 2-6
		rn := 1 + rng.Intn(5)
		rd := rn + 1 + rng.Intn(6) // ensure rd > rn, i.e. proper fraction
		for gcd(rn, rd) != 1 {
			rn = 1 + rng.Intn(5)
			rd = rn + 1 + rng.Intn(6)
		}
		num, den := rn*g, rd*g
		prompt := fmt.Sprintf("Simplify %s to lowest terms", fracStr(num, den))
		exp := fmt.Sprintf("%s ÷ %d/%d = %s", fracStr(num, den), g, g, fracStr(rn, rd))
		return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: rn, Den: rd}, exp

	case 7: // mixed number -> improper fraction
		whole := 1 + rng.Intn(5)
		d := pickDenom(rng)
		if d < 2 {
			d = 2
		}
		n := 1 + rng.Intn(d-1)
		improperNum := whole*d + n
		prompt := fmt.Sprintf("Write %d %s as an improper fraction (num/den)", whole, fracStr(n, d))
		exp := fmt.Sprintf("%d %s = (%d×%d+%d)/%d = %s", whole, fracStr(n, d), whole, d, n, d, fracStr(improperNum, d))
		return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: improperNum, Den: d}, exp

	case 8: // add/sub unlike denominators, one denom a multiple of the other
		d1 := pickDenom(rng)
		k := 2 + rng.Intn(3) // x2..x4
		d2 := d1 * k
		if d2 > 24 {
			d2 = d1 * 2
			k = 2
		}
		n1 := 1 + rng.Intn(d1-1)
		n2 := 1 + rng.Intn(d2-1)
		commonNum1 := n1 * k
		if rng.Intn(2) == 0 {
			num, den := simplify(commonNum1+n2, d2)
			prompt := fmt.Sprintf("%s + %s = ?", fracStr(n1, d1), fracStr(n2, d2))
			exp := fmt.Sprintf("%s = %s; %s+%s = %s", fracStr(n1, d1), fracStr(commonNum1, d2), fracStr(commonNum1, d2), fracStr(n2, d2), fracStr(commonNum1+n2, d2))
			return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: num, Den: den}, exp
		}
		big, small := commonNum1, n2
		if small > big {
			big, small = small, big
		}
		num, den := simplify(big-small, d2)
		prompt := fmt.Sprintf("%s − %s = ?", fracStr(n1, d1), fracStr(n2, d2))
		exp := fmt.Sprintf("%s = %s; difference = %s", fracStr(n1, d1), fracStr(commonNum1, d2), fracStr(big-small, d2))
		return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: num, Den: den}, exp

	case 9: // fraction of a quantity (3/4 of 48) -> numeric
		d := pickDenom(rng)
		if d < 2 {
			d = 2
		}
		n := 1 + rng.Intn(d-1)
		mult := 1 + rng.Intn(12)
		quantity := d * mult
		result := n * mult
		prompt := fmt.Sprintf("%s of %d = ?", fracStr(n, d), quantity)
		exp := fmt.Sprintf("%s of %d = %d÷%d×%d = %d", fracStr(n, d), quantity, quantity, d, n, result)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp

	case 10: // add/sub any unlike denominators
		d1, d2 := pickDenom(rng), pickDenom(rng)
		for d2 == d1 {
			d2 = pickDenom(rng)
		}
		n1 := 1 + rng.Intn(d1-1)
		n2 := 1 + rng.Intn(d2-1)
		l := lcm(d1, d2)
		c1 := n1 * (l / d1)
		c2 := n2 * (l / d2)
		if rng.Intn(2) == 0 {
			num, den := simplify(c1+c2, l)
			prompt := fmt.Sprintf("%s + %s = ?", fracStr(n1, d1), fracStr(n2, d2))
			exp := fmt.Sprintf("common denominator %d: %s+%s = %s", l, fracStr(c1, l), fracStr(c2, l), fracStr(c1+c2, l))
			return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: num, Den: den}, exp
		}
		big, small := c1, c2
		if small > big {
			big, small = small, big
		}
		num, den := simplify(big-small, l)
		prompt := fmt.Sprintf("%s − %s = ?", fracStr(n1, d1), fracStr(n2, d2))
		exp := fmt.Sprintf("common denominator %d: difference = %s", l, fracStr(big-small, l))
		return Payload{Kind: KindFraction, Prompt: prompt}, FractionAnswer{Num: num, Den: den}, exp

	default:
		return genFractions(1, rng)
	}
}

func lcm(a, b int) int { return a / gcd(a, b) * b }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
