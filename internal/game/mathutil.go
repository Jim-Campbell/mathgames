package game

// gcd returns the greatest common divisor of two positive integers.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// simplify reduces num/den to lowest terms. den must be nonzero.
func simplify(num, den int) (int, int) {
	if num == 0 {
		return 0, den
	}
	g := gcd(abs(num), abs(den))
	if g == 0 {
		return num, den
	}
	return num / g, den / g
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
