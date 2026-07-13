package game

import (
	"fmt"
	"math/rand"
)

// noRegroupPair picks a,b in [lo,hi] (same digit width) such that a+b needs
// no carrying in any place (each digit pair sums <= 9). Falls back to
// swapping/retrying a bounded number of times; if it can't find one (should
// be rare given the ranges used), it returns whatever it last tried, which
// is still in-range even if regrouping happens to occur.
func noRegroupAddPair(rng *rand.Rand, lo, hi int) (a, b int) {
	for i := 0; i < 50; i++ {
		a = lo + rng.Intn(hi-lo+1)
		b = lo + rng.Intn(hi-lo+1)
		if digitsAddNoCarry(a, b) {
			return a, b
		}
	}
	return a, b
}

func digitsAddNoCarry(a, b int) bool {
	for a > 0 || b > 0 {
		if a%10+b%10 > 9 {
			return false
		}
		a /= 10
		b /= 10
	}
	return true
}

// regroupAddPair forces at least one carry (ones digits sum >= 10).
func regroupAddPair(rng *rand.Rand, lo, hi int) (a, b int) {
	for i := 0; i < 50; i++ {
		a = lo + rng.Intn(hi-lo+1)
		b = lo + rng.Intn(hi-lo+1)
		if a%10+b%10 >= 10 {
			return a, b
		}
	}
	return a, b
}

// noRegroupSubPair picks a>=b in [lo,hi] such that every digit of a is >=
// the corresponding digit of b (no borrowing).
func noRegroupSubPair(rng *rand.Rand, lo, hi int) (a, b int) {
	for i := 0; i < 50; i++ {
		x := lo + rng.Intn(hi-lo+1)
		y := lo + rng.Intn(hi-lo+1)
		if x < y {
			x, y = y, x
		}
		if digitsSubNoBorrow(x, y) {
			return x, y
		}
	}
	if a < b {
		a, b = b, a
	}
	return a, b
}

func digitsSubNoBorrow(a, b int) bool {
	for a > 0 || b > 0 {
		if a%10 < b%10 {
			return false
		}
		a /= 10
		b /= 10
	}
	return true
}

// regroupSubPair forces a>=b with at least one borrow (some digit of a is
// smaller than the corresponding digit of b).
func regroupSubPair(rng *rand.Rand, lo, hi int) (a, b int) {
	for i := 0; i < 50; i++ {
		x := lo + rng.Intn(hi-lo+1)
		y := lo + rng.Intn(hi-lo+1)
		if x < y {
			x, y = y, x
		}
		if x == y {
			continue
		}
		if !digitsSubNoBorrow(x, y) {
			return x, y
		}
	}
	x := lo + rng.Intn(hi-lo+1)
	y := lo + rng.Intn(hi-lo+1)
	if x < y {
		x, y = y, x
	}
	return x, y
}

func addSubPayload(a, b int, add bool) (Payload, any, string) {
	if add {
		result := a + b
		return Payload{Kind: KindNumeric, Prompt: fmt.Sprintf("%d + %d = ?", a, b)},
			NumericAnswer{Value: result}, fmt.Sprintf("%d+%d = %d", a, b, result)
	}
	result := a - b
	return Payload{Kind: KindNumeric, Prompt: fmt.Sprintf("%d − %d = ?", a, b)},
		NumericAnswer{Value: result}, fmt.Sprintf("%d-%d = %d", a, b, result)
}

// genAddSub implements the addsub level table (ARCHITECTURE.md "Skills and
// difficulty").
func genAddSub(level int, rng *rand.Rand) (payload Payload, answer any, explanation string) {
	switch level {
	case 1: // 2-digit +- 2-digit, no regrouping
		if rng.Intn(2) == 0 {
			a, b := noRegroupAddPair(rng, 10, 89)
			return addSubPayload(a, b, true)
		}
		a, b := noRegroupSubPair(rng, 10, 99)
		return addSubPayload(a, b, false)
	case 2: // 2-digit with regrouping
		if rng.Intn(2) == 0 {
			a, b := regroupAddPair(rng, 10, 89)
			return addSubPayload(a, b, true)
		}
		a, b := regroupSubPair(rng, 10, 99)
		return addSubPayload(a, b, false)
	case 3: // 3-digit with regrouping
		if rng.Intn(2) == 0 {
			a, b := regroupAddPair(rng, 100, 899)
			return addSubPayload(a, b, true)
		}
		a, b := regroupSubPair(rng, 100, 999)
		return addSubPayload(a, b, false)
	case 4: // 3 addends of 2-3 digits
		a, b, c := 10+rng.Intn(990), 10+rng.Intn(990), 10+rng.Intn(990)
		result := a + b + c
		prompt := fmt.Sprintf("%d + %d + %d = ?", a, b, c)
		exp := fmt.Sprintf("%d+%d+%d = %d+%d = %d", a, b, c, a+b, c, result)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp
	case 5: // 4-digit +- 4-digit
		if rng.Intn(2) == 0 {
			a, b := regroupAddPair(rng, 1000, 8999)
			return addSubPayload(a, b, true)
		}
		a, b := regroupSubPair(rng, 1000, 9999)
		return addSubPayload(a, b, false)
	case 6: // subtraction across zeros (5003-1876)
		a := (1+rng.Intn(9))*1000 + rng.Intn(10) // e.g. 5003, 8007
		b := 1000 + rng.Intn(a-999)
		if b >= a {
			b = a - 1
		}
		return addSubPayload(a, b, false)
	case 7: // 5-digit
		if rng.Intn(2) == 0 {
			a, b := regroupAddPair(rng, 10000, 89999)
			return addSubPayload(a, b, true)
		}
		a, b := regroupSubPair(rng, 10000, 99999)
		return addSubPayload(a, b, false)
	case 8: // mixed chains a+b-c
		a, b := 10+rng.Intn(890), 10+rng.Intn(890)
		c := rng.Intn(a + b - 9)
		if c < 10 {
			c = 10
		}
		result := a + b - c
		prompt := fmt.Sprintf("%d + %d − %d = ?", a, b, c)
		exp := fmt.Sprintf("%d+%d-%d = %d-%d = %d", a, b, c, a+b, c, result)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp
	case 9: // missing-operand (___ - 2748 = 1265)
		b := 1000 + rng.Intn(8000)
		r := 100 + rng.Intn(8000)
		missing := b + r
		prompt := fmt.Sprintf("? − %d = %d", b, r)
		exp := fmt.Sprintf("? = %d + %d = %d", b, r, missing)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: missing}, exp
	case 10: // 6-digit chains
		a, b := 100000+rng.Intn(800000), 10000+rng.Intn(80000)
		c := 10000 + rng.Intn(80000)
		result := a + b - c
		prompt := fmt.Sprintf("%d + %d − %d = ?", a, b, c)
		exp := fmt.Sprintf("%d+%d-%d = %d-%d = %d", a, b, c, a+b, c, result)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp
	default:
		a, b := noRegroupAddPair(rng, 10, 89)
		return addSubPayload(a, b, true)
	}
}
