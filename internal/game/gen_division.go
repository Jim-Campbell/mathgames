package game

import (
	"fmt"
	"math/rand"
)

func exactDivPayload(divisor, quotient, dividend int) (Payload, any, string) {
	prompt := fmt.Sprintf("%d ÷ %d = ?", dividend, divisor)
	exp := fmt.Sprintf("%d÷%d = %d (%d×%d = %d)", dividend, divisor, quotient, quotient, divisor, dividend)
	return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: quotient}, exp
}

func remainderDivPayload(divisor, quotient, remainder, dividend int) (Payload, any, string) {
	prompt := fmt.Sprintf("%d ÷ %d = ? remainder ?", dividend, divisor)
	exp := fmt.Sprintf("%d÷%d = %d remainder %d (%d×%d = %d, %d-%d = %d)",
		dividend, divisor, quotient, remainder, quotient, divisor, quotient*divisor, dividend, quotient*divisor, remainder)
	return Payload{Kind: KindNumeric2, Prompt: prompt, Labels: []string{"quotient", "remainder"}},
		Numeric2Answer{Values: [2]int{quotient, remainder}}, exp
}

// exactDiv picks a divisor in [divLo,divHi] and a quotient such that
// dividend = divisor*quotient lands in [dividendLo,dividendHi].
func exactDiv(rng *rand.Rand, divLo, divHi, dividendLo, dividendHi int) (divisor, quotient, dividend int) {
	divisor = divLo + rng.Intn(divHi-divLo+1)
	minQ := ceilDiv(dividendLo, divisor)
	maxQ := dividendHi / divisor
	if minQ < 1 {
		minQ = 1
	}
	if maxQ < minQ {
		maxQ = minQ
	}
	quotient = minQ + rng.Intn(maxQ-minQ+1)
	dividend = divisor * quotient
	return
}

// remainderDiv picks a divisor, a nonzero remainder < divisor, and a
// quotient such that dividend = divisor*quotient+remainder lands in
// [dividendLo,dividendHi].
func remainderDiv(rng *rand.Rand, divLo, divHi, dividendLo, dividendHi int) (divisor, quotient, remainder, dividend int) {
	divisor = divLo + rng.Intn(divHi-divLo+1)
	if divisor < 2 {
		divisor = 2
	}
	remainder = 1 + rng.Intn(divisor-1)
	minQ := ceilDiv(dividendLo-remainder, divisor)
	maxQ := (dividendHi - remainder) / divisor
	if minQ < 1 {
		minQ = 1
	}
	if maxQ < minQ {
		maxQ = minQ
	}
	quotient = minQ + rng.Intn(maxQ-minQ+1)
	dividend = divisor*quotient + remainder
	return
}

func ceilDiv(a, b int) int {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// genDivision implements the division level table (ARCHITECTURE.md "Skills
// and difficulty").
func genDivision(level int, rng *rand.Rand) (payload Payload, answer any, explanation string) {
	switch level {
	case 1: // inverses of <=9x9 facts, exact
		divisor, quotient, dividend := exactDiv(rng, 1, 9, 1, 81)
		return exactDivPayload(divisor, quotient, dividend)
	case 2: // <=12x12 facts, exact
		divisor, quotient, dividend := exactDiv(rng, 1, 12, 1, 144)
		return exactDivPayload(divisor, quotient, dividend)
	case 3: // 2-digit / 1-digit exact
		divisor, quotient, dividend := exactDiv(rng, 2, 9, 10, 99)
		return exactDivPayload(divisor, quotient, dividend)
	case 4: // 2-digit / 1-digit with remainder
		divisor, quotient, remainder, dividend := remainderDiv(rng, 2, 9, 10, 99)
		return remainderDivPayload(divisor, quotient, remainder, dividend)
	case 5: // 3-digit / 1-digit exact
		divisor, quotient, dividend := exactDiv(rng, 2, 9, 100, 999)
		return exactDivPayload(divisor, quotient, dividend)
	case 6: // 3-digit / 1-digit with remainder
		divisor, quotient, remainder, dividend := remainderDiv(rng, 2, 9, 100, 999)
		return remainderDivPayload(divisor, quotient, remainder, dividend)
	case 7: // dividing multiples of 10 (4200 / 70)
		divisor := (1 + rng.Intn(9)) * 10 // 10-90
		quotient := (1 + rng.Intn(9)) * 10
		dividend := divisor * quotient
		return exactDivPayload(divisor, quotient, dividend)
	case 8: // 3-digit / 2-digit exact
		divisor, quotient, dividend := exactDiv(rng, 10, 99, 100, 999)
		return exactDivPayload(divisor, quotient, dividend)
	case 9: // 3-digit / 2-digit with remainder
		divisor, quotient, remainder, dividend := remainderDiv(rng, 10, 99, 100, 999)
		return remainderDivPayload(divisor, quotient, remainder, dividend)
	case 10: // 4-digit / 2-digit with remainder
		divisor, quotient, remainder, dividend := remainderDiv(rng, 10, 99, 1000, 9999)
		return remainderDivPayload(divisor, quotient, remainder, dividend)
	default:
		divisor, quotient, dividend := exactDiv(rng, 1, 9, 1, 81)
		return exactDivPayload(divisor, quotient, dividend)
	}
}
