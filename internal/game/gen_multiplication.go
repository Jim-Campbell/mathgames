package game

import (
	"fmt"
	"math/rand"
)

// multiplicationBaits are small (a,b) pairs whose product is a round number,
// used at L9 as "associativity bait" (e.g. 4×7×25 rewards spotting 4×25=100
// before multiplying by 7).
var multiplicationBaits = [][2]int{
	{4, 25}, {2, 50}, {5, 20}, {8, 125}, {4, 250}, {2, 500}, {5, 200},
}

// mulExplanation splits b into tens+ones and shows the partial-product
// working, matching the ARCHITECTURE.md worked example style
// ("27×34 = 27×30 + 27×4 = 810 + 108").
func mulExplanation(a, b int) string {
	b10 := (b / 10) * 10
	b1 := b % 10
	if b10 == 0 || b1 == 0 {
		return fmt.Sprintf("%d×%d = %d", a, b, a*b)
	}
	return fmt.Sprintf("%d×%d = %d×%d + %d×%d = %d + %d", a, b, a, b10, a, b1, a*b10, a*b1)
}

// Generate implements the multiplication level table (ARCHITECTURE.md
// "Skills and difficulty").
func genMultiplication(level int, rng *rand.Rand) (payload Payload, answer any, explanation string) {
	var a, b int

	switch level {
	case 1: // facts <= 5x5
		a, b = 1+rng.Intn(5), 1+rng.Intn(5)
	case 2: // facts <= 9x9
		a, b = 1+rng.Intn(9), 1+rng.Intn(9)
	case 3: // facts <= 12x12
		a, b = 1+rng.Intn(12), 1+rng.Intn(12)
	case 4: // 2-digit x 1-digit
		a, b = 10+rng.Intn(90), 1+rng.Intn(9)
	case 5: // multiples of 10 (30x70)
		a, b = (1+rng.Intn(9))*10, (1+rng.Intn(9))*10
	case 6: // 2-digit x 2-digit
		a, b = 10+rng.Intn(90), 10+rng.Intn(90)
	case 7: // 3-digit x 1-digit
		a, b = 100+rng.Intn(900), 1+rng.Intn(9)
	case 8: // 3-digit x 2-digit
		a, b = 100+rng.Intn(900), 10+rng.Intn(90)
	case 9: // 3 factors, associativity bait
		bait := multiplicationBaits[rng.Intn(len(multiplicationBaits))]
		c := 2 + rng.Intn(8)
		prompt := fmt.Sprintf("%d × %d × %d = ?", bait[0], c, bait[1])
		result := bait[0] * c * bait[1]
		exp := fmt.Sprintf("%d×%d×%d = %d×%d×%d = %d×%d = %d",
			bait[0], c, bait[1], bait[0], bait[1], c, bait[0]*bait[1], c, result)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp
	case 10: // 4-digit x 2-digit
		a, b = 1000+rng.Intn(9000), 10+rng.Intn(90)
	default:
		a, b = 1+rng.Intn(5), 1+rng.Intn(5)
	}

	result := a * b
	prompt := fmt.Sprintf("%d × %d = ?", a, b)
	return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, mulExplanation(a, b)
}
