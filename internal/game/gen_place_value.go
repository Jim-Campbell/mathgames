package game

import (
	"fmt"
	"math/rand"
	"strings"
)

var onesWords = []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
	"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
var tensWords = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}

// wordsForNumber renders n (0-9,999,999) in English words, for L8's
// read/write questions.
func wordsForNumber(n int) string {
	if n == 0 {
		return "zero"
	}
	var parts []string
	if n >= 1000000 {
		parts = append(parts, wordsBelowThousand(n/1000000)+" million")
		n %= 1000000
	}
	if n >= 1000 {
		parts = append(parts, wordsBelowThousand(n/1000)+" thousand")
		n %= 1000
	}
	if n > 0 {
		parts = append(parts, wordsBelowThousand(n))
	}
	return strings.Join(parts, " ")
}

func wordsBelowThousand(n int) string {
	var parts []string
	if n >= 100 {
		parts = append(parts, onesWords[n/100]+" hundred")
		n %= 100
	}
	if n >= 20 {
		w := tensWords[n/10]
		if n%10 != 0 {
			w += "-" + onesWords[n%10]
		}
		parts = append(parts, w)
	} else if n > 0 {
		parts = append(parts, onesWords[n])
	}
	return strings.Join(parts, " ")
}

func compareChoiceIdx(a, b int) int {
	switch {
	case a < b:
		return 0
	case a == b:
		return 1
	default:
		return 2
	}
}

func roundTo(n, place int) int {
	half := place / 2
	return ((n + half) / place) * place
}

// genPlaceValue implements the place_value level table (ARCHITECTURE.md
// "Skills and difficulty").
func genPlaceValue(level int, rng *rand.Rand) (payload Payload, answer any, explanation string) {
	compareChoices := []string{"<", "=", ">"}

	switch level {
	case 1: // value of a digit in a 3-digit number
		n := 100 + rng.Intn(900)
		place := []int{100, 10, 1}[rng.Intn(3)]
		digit := (n / place) % 10
		value := digit * place
		prompt := fmt.Sprintf("What is the value of the digit %d in %d?", digit, n)
		exp := fmt.Sprintf("the %d is in the %s place, so its value is %d", digit, placeName(place), value)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: value}, exp

	case 2: // 4-digit compare
		a := 1000 + rng.Intn(9000)
		b := 1000 + rng.Intn(9000)
		idx := compareChoiceIdx(a, b)
		prompt := fmt.Sprintf("%d ? %d", a, b)
		exp := fmt.Sprintf("%d %s %d", a, compareChoices[idx], b)
		return Payload{Kind: KindMC, Prompt: prompt, Choices: compareChoices}, MCAnswer{Index: idx}, exp

	case 3: // round to nearest 10/100
		place := []int{10, 100}[rng.Intn(2)]
		n := place + rng.Intn(9*place)
		result := roundTo(n, place)
		prompt := fmt.Sprintf("Round %d to the nearest %d", n, place)
		exp := fmt.Sprintf("%d rounds to %d (nearest %d)", n, result, place)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp

	case 4: // expanded form to 10,000 -> mc
		n := 1000 + rng.Intn(9000)
		correct := expandedForm(n)
		wrong1 := expandedForm(n + 100)
		wrong2 := expandedForm(scrambleDigits(n))
		choices := []string{correct, wrong1, wrong2}
		rng.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })
		idx := indexOf(choices, correct)
		prompt := fmt.Sprintf("Which is the expanded form of %d?", n)
		exp := fmt.Sprintf("%d = %s", n, correct)
		return Payload{Kind: KindMC, Prompt: prompt, Choices: choices}, MCAnswer{Index: idx}, exp

	case 5: // round to nearest 1,000
		n := 1000 + rng.Intn(9000)
		result := roundTo(n, 1000)
		prompt := fmt.Sprintf("Round %d to the nearest 1,000", n)
		exp := fmt.Sprintf("%d rounds to %d (nearest 1,000)", n, result)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp

	case 6: // 6-digit compare and order
		a := 100000 + rng.Intn(900000)
		b := 100000 + rng.Intn(900000)
		idx := compareChoiceIdx(a, b)
		prompt := fmt.Sprintf("%d ? %d", a, b)
		exp := fmt.Sprintf("%d %s %d", a, compareChoices[idx], b)
		return Payload{Kind: KindMC, Prompt: prompt, Choices: compareChoices}, MCAnswer{Index: idx}, exp

	case 7: // round to any place in 6-digit numbers
		n := 100000 + rng.Intn(900000)
		place := []int{10, 100, 1000, 10000, 100000}[rng.Intn(5)]
		result := roundTo(n, place)
		prompt := fmt.Sprintf("Round %d to the nearest %s", n, placeName(place))
		exp := fmt.Sprintf("%d rounds to %d (nearest %s)", n, result, placeName(place))
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp

	case 8: // 7-digit read/write (words -> numeral)
		n := 1000000 + rng.Intn(9000000)
		words := wordsForNumber(n)
		prompt := fmt.Sprintf("Write this number as digits: %s", words)
		exp := fmt.Sprintf("%s = %d", words, n)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: n}, exp

	case 9: // 10,000 more than
		n := 100000 + rng.Intn(800000)
		result := n + 10000
		prompt := fmt.Sprintf("What number is 10,000 more than %d?", n)
		exp := fmt.Sprintf("%d + 10,000 = %d", n, result)
		return Payload{Kind: KindNumeric, Prompt: prompt}, NumericAnswer{Value: result}, exp

	case 10: // mixed multi-step (round, then compare)
		place := []int{100, 1000, 10000}[rng.Intn(3)]
		a := 100000 + rng.Intn(900000)
		b := 100000 + rng.Intn(900000)
		ra, rb := roundTo(a, place), roundTo(b, place)
		idx := compareChoiceIdx(ra, rb)
		prompt := fmt.Sprintf("Round %d and %d to the nearest %s, then compare: ? ? ?", a, b, placeName(place))
		exp := fmt.Sprintf("%d rounds to %d, %d rounds to %d — %d %s %d", a, ra, b, rb, ra, compareChoices[idx], rb)
		return Payload{Kind: KindMC, Prompt: prompt, Choices: compareChoices}, MCAnswer{Index: idx}, exp

	default:
		return genPlaceValue(1, rng)
	}
}

func placeName(place int) string {
	switch place {
	case 1:
		return "ones"
	case 10:
		return "tens"
	case 100:
		return "hundreds"
	case 1000:
		return "thousands"
	case 10000:
		return "ten-thousands"
	case 100000:
		return "hundred-thousands"
	default:
		return fmt.Sprintf("%d's", place)
	}
}

func expandedForm(n int) string {
	var parts []string
	place := 1
	for place <= n {
		place *= 10
	}
	place /= 10
	for place >= 1 {
		digit := (n / place) % 10
		if digit != 0 {
			parts = append(parts, fmt.Sprintf("%d", digit*place))
		}
		place /= 10
	}
	if len(parts) == 0 {
		return "0"
	}
	return strings.Join(parts, " + ")
}

func scrambleDigits(n int) int {
	// A plausible-looking wrong answer: swap the top two nonzero digit
	// weights so the expanded form looks similar but wrong.
	s := fmt.Sprintf("%d", n)
	if len(s) < 2 {
		return n + 1
	}
	b := []byte(s)
	b[0], b[1] = b[1], b[0]
	var v int
	fmt.Sscanf(string(b), "%d", &v)
	if v == n {
		return n + 100
	}
	return v
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
