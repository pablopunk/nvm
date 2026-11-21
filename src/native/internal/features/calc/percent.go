package calc

import (
	"math"
	"regexp"
	"strings"
)

// unitPercentPattern matches "19m + 47%": a quantity with a unit, a plus or minus, and a percentage.
var unitPercentPattern = regexp.MustCompile(`^([+-]?(?:\d+(?:,\d{3})*|\d*)(?:\.\d+)?)\s*([a-zA-Z]+)\s*([+-])\s*([+-]?(?:\d+(?:,\d{3})*|\d*)(?:\.\d+)?)\s*%$`)

// calculateUnitPercent applies a percentage change to a quantity that carries a unit and keeps the unit in the
// answer.
func calculateUnitPercent(expression, query string) (Result, bool) {
	trimmed := strings.TrimSpace(expression)
	groups := unitPercentPattern.FindStringSubmatch(trimmed)
	if groups == nil {
		return Result{}, false
	}
	left, ok := parseNumberLiteral(groups[1])
	if !ok {
		return Result{}, false
	}
	percent, ok := parseNumberLiteral(groups[4])
	if !ok {
		return Result{}, false
	}
	unitName := groups[2]
	multiplier := 1 - percent/100
	if groups[3] == "+" {
		multiplier = 1 + percent/100
	}
	value := left * multiplier
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Result{}, false
	}
	raw := formatRawNumber(value, normalizedPrecision) + " " + unitName
	display := formatDisplayNumber(value, normalizedPrecision) + " " + unitName
	r := newResult(KindPercent, strings.TrimSpace(query), trimmed, value, raw, display, "Copy result to clipboard")
	r.Unit = unitName
	return describe(r), true
}
