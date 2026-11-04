package calc

import (
	"math"
	"strconv"
	"strings"
)

// suffixMultipliers are the magnitude suffixes that scale a number: 50k, 1.2m, 3b.
var suffixMultipliers = map[string]float64{"k": 1e3, "m": 1e6, "b": 1e9}

// suffixMultiplier returns the multiplier for a single lower-case suffix letter.
func suffixMultiplier(suffix string) (float64, bool) {
	multiplier, ok := suffixMultipliers[suffix]
	return multiplier, ok
}

// isPlainNumber reports whether text is an optional sign, digits with thousands commas, and at most one
// decimal point, with at least one digit. Exponents, hex and words are rejected.
func isPlainNumber(text string) bool {
	digits, dots := 0, 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.':
			dots++
		case c == ',':
		case (c == '+' || c == '-') && i == 0:
		default:
			return false
		}
	}
	return digits > 0 && dots <= 1
}

// parseNumberLiteral reads a plain decimal, ignoring thousands commas, the way the TypeScript Number call
// does for the strings the patterns admit. Results that overflow to infinity are rejected.
func parseNumberLiteral(text string) (float64, bool) {
	if !isPlainNumber(text) {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(text, ",", ""), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, false
	}
	return value, true
}

// parseAmountWithSuffix reads "1.2m" as 1200000 and "50" as 50.
func parseAmountWithSuffix(text string) (float64, bool) {
	trimmed := strings.TrimSpace(text)
	multiplier := 1.0
	number := trimmed
	if n := len(trimmed); n > 0 {
		if m, ok := suffixMultiplier(strings.ToLower(trimmed[n-1:])); ok {
			multiplier = m
			number = strings.TrimSpace(trimmed[:n-1])
		}
	}
	value, ok := parseNumberLiteral(number)
	if !ok {
		return 0, false
	}
	return value * multiplier, true
}
