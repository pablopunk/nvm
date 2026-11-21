package calc

import (
	"math"
	"strconv"
	"strings"
)

// normalizedPrecision is the number of significant digits every computed value is rounded to.
const normalizedPrecision = 12

// exactDigits is how many significant digits are read from a float before rounding. 31 digits of the exact
// binary value are enough to decide every tie at 13 digits correctly.
const exactDigits = 31

// normalizeFloatingPoint rounds value to 12 significant digits, as Number(value.toPrecision(12)) does. Ties
// round away from zero (toPrecision picks the larger magnitude), which strconv's half-to-even would not.
func normalizeFloatingPoint(value float64) float64 {
	if value == 0 {
		return 0
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	digits, exponent := significantDigits(math.Abs(value), normalizedPrecision)
	parsed, _ := strconv.ParseFloat(digits[:1]+"."+digits[1:]+"e"+strconv.Itoa(exponent), 64)
	if value < 0 {
		return -parsed
	}
	return parsed
}

// significantDigits returns the first n significant digits of the positive finite v, rounded half up from the
// exact binary value, and the decimal exponent of the first digit.
func significantDigits(v float64, n int) (string, int) {
	text := strconv.FormatFloat(v, 'e', exactDigits-1, 64)
	marker := strings.IndexByte(text, 'e')
	exponent, _ := strconv.Atoi(text[marker+1:])
	digits := []byte(text[:1] + text[2:marker])
	kept := digits[:n]
	if digits[n] >= '5' && incrementDigits(kept) {
		kept[0] = '1'
		exponent++
	}
	return string(kept), exponent
}

// incrementDigits adds one to a decimal digit string in place. It reports true when every digit was 9, in
// which case the digits are now all zeros and the caller must carry into a new leading digit.
func incrementDigits(digits []byte) bool {
	for i := len(digits) - 1; i >= 0; i-- {
		if digits[i] < '9' {
			digits[i]++
			return false
		}
		digits[i] = '0'
	}
	return true
}

// formatRawNumber renders a value for copying and chaining: no grouping, at most maxFractionDigits decimals,
// and integers without a decimal point.
func formatRawNumber(value float64, maxFractionDigits int) string {
	normalized := normalizeFloatingPoint(value)
	if normalized == math.Trunc(normalized) {
		return integerText(normalized)
	}
	return formatDecimal(normalized, 0, maxFractionDigits, false)
}

// formatDisplayNumber renders a value for people: grouped thousands and at most maxFractionDigits decimals.
func formatDisplayNumber(value float64, maxFractionDigits int) string {
	return formatDecimal(normalizeFloatingPoint(value), 0, maxFractionDigits, true)
}

// integerText renders a whole number the way JavaScript's String does: plain digits below 1e21 and an
// exponent form above.
func integerText(value float64) string {
	if math.Abs(value) < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return strconv.FormatFloat(value, 'e', -1, 64)
}

// formatDecimal rounds v to at most maxFraction decimals and pads to minFraction, the way Intl.NumberFormat
// does with half-expand rounding on the shortest decimal form of the number. Negative values keep their sign
// even when they round to zero, as Intl does.
func formatDecimal(v float64, minFraction, maxFraction int, grouped bool) string {
	digits, point := "0", 1
	if v != 0 {
		digits, point = shortestDecimal(math.Abs(v))
		digits, point = roundDecimal(digits, point, maxFraction)
	}
	if digits == "" {
		digits, point = "0", 1
	}
	whole, fraction := splitDecimal(digits, point)
	fraction = strings.TrimRight(fraction, "0")
	if len(fraction) < minFraction {
		fraction += strings.Repeat("0", minFraction-len(fraction))
	}
	if grouped {
		whole = groupThousands(whole)
	}
	text := whole
	if fraction != "" {
		text += "." + fraction
	}
	if v < 0 {
		return "-" + text
	}
	return text
}

// shortestDecimal returns the shortest round-tripping digits of the positive finite v and the position of the
// decimal point relative to the first digit, so that v = 0.digits × 10^point.
func shortestDecimal(v float64) (string, int) {
	text := strconv.FormatFloat(v, 'e', -1, 64)
	marker := strings.IndexByte(text, 'e')
	exponent, _ := strconv.Atoi(text[marker+1:])
	return strings.Replace(text[:marker], ".", "", 1), exponent + 1
}

// roundDecimal keeps the digits that sit within maxFraction places after the decimal point, rounding half up.
// A result that rounds to nothing is the empty string.
func roundDecimal(digits string, point, maxFraction int) (string, int) {
	keep := point + maxFraction
	if keep >= len(digits) {
		return digits, point
	}
	if keep < 0 {
		return "", point
	}
	roundUp := digits[keep] >= '5'
	digits = digits[:keep]
	if !roundUp {
		return digits, point
	}
	if digits == "" {
		return "1", point + 1
	}
	kept := []byte(digits)
	if incrementDigits(kept) {
		return "1" + string(kept), point + 1
	}
	return string(kept), point
}

// splitDecimal splits digits with the decimal point at point into whole and fraction parts.
func splitDecimal(digits string, point int) (string, string) {
	if point <= 0 {
		return "0", strings.Repeat("0", -point) + digits
	}
	if len(digits) <= point {
		return digits + strings.Repeat("0", point-len(digits)), ""
	}
	return digits[:point], digits[point:]
}

// groupThousands inserts commas every three digits of a whole-number string.
func groupThousands(whole string) string {
	if len(whole) <= 3 {
		return whole
	}
	var b strings.Builder
	first := len(whole) % 3
	if first > 0 {
		b.WriteString(whole[:first])
	}
	for i := first; i < len(whole); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(whole[i : i+3])
	}
	return b.String()
}
