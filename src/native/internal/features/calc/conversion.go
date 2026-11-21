package calc

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// conversionPattern matches "10 ft in m", "convert 5kg to lb" and "145 mins to timespan".
var conversionPattern = regexp.MustCompile(`(?i)^(?:convert\s+)?(` + numberPattern + `)\s*(` + unitPattern + `)\s+(?:in|to|as)\s+(` + unitPattern + `|timespan)$`)

// numberPattern is the plain decimal form a conversion amount may take.
const numberPattern = `[+-]?(?:(?:\d+(?:,\d{3})*)|(?:\d+)|(?:\d*\.\d+))(?:\.\d+)?`

// unitPattern is the word form of a unit, including the degree sign used by temperatures.
const unitPattern = `[a-zA-Z°]+`

// conversionTimespan is the pseudo-unit that turns a time quantity into a duration like "2h 25m".
const conversionTimespan = "timespan"

// conversionMatch is the text of a conversion expression split into its three parts.
type conversionMatch struct {
	amount     string
	sourceUnit string
	targetUnit string
}

// matchConversion splits an expression that reads as "amount unit to unit".
func matchConversion(expression string) (conversionMatch, bool) {
	groups := conversionPattern.FindStringSubmatch(strings.TrimSpace(expression))
	if groups == nil {
		return conversionMatch{}, false
	}
	return conversionMatch{amount: groups[1], sourceUnit: groups[2], targetUnit: groups[3]}, true
}

// calculateConversion converts between two units of the same dimension. Mismatched dimensions and unknown units
// are not conversions, so they fall through to arithmetic and fail there.
func calculateConversion(expression, query string) (Result, bool) {
	match, ok := matchConversion(expression)
	if !ok {
		return Result{}, false
	}
	amount, ok := parseNumberLiteral(match.amount)
	if !ok {
		return Result{}, false
	}
	source, ok := unitForAlias(match.sourceUnit)
	if !ok {
		return Result{}, false
	}
	isTimespan := normalizeUnitAlias(match.targetUnit) == conversionTimespan
	var target unit
	if isTimespan {
		if source.dimension != dimensionTime {
			return Result{}, false
		}
	} else {
		target, ok = unitForAlias(match.targetUnit)
		if !ok || source.dimension != target.dimension {
			return Result{}, false
		}
	}
	baseValue := source.toBase(amount)
	value := baseValue
	if !isTimespan {
		value = target.fromBase(baseValue)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Result{}, false
	}
	return conversionResult(query, strings.TrimSpace(expression), source, target, isTimespan, value), true
}

// conversionResult builds the answer for a converted value, with the reverse conversion as swap text.
func conversionResult(query, expression string, source, target unit, isTimespan bool, value float64) Result {
	raw, display, swap, targetSymbol := conversionTexts(source, target, isTimespan, value)
	r := newResult(KindUnit, strings.TrimSpace(query), expression, value, raw, display, "Copy converted result to clipboard")
	r.Unit = source.symbol
	r.TargetUnit = targetSymbol
	r.SwapQuery = swap
	return describe(r)
}

// conversionTexts renders the raw and display text of a converted value, the swap query, and the target label.
func conversionTexts(source, target unit, isTimespan bool, value float64) (raw, display, swap, targetSymbol string) {
	if isTimespan {
		text := formatTimespan(value)
		return text, text, "", conversionTimespan
	}
	raw = formatRawNumber(value, 4) + " " + target.symbol
	display = formatDisplayNumber(value, 4) + " " + target.symbol
	swap = formatRawNumber(value, 4) + " " + target.symbol + " to " + source.symbol
	return raw, display, swap, target.symbol
}

// formatTimespan renders seconds as "2d 3h 4m 5s", omitting zero parts and always showing at least seconds.
func formatTimespan(seconds float64) string {
	sign := ""
	if seconds < 0 {
		sign = "-"
	}
	remaining := jsRound(math.Abs(seconds))
	days := math.Floor(remaining / 86400)
	remaining -= days * 86400
	hours := math.Floor(remaining / 3600)
	remaining -= hours * 3600
	minutes := math.Floor(remaining / 60)
	remaining -= minutes * 60
	var parts []string
	if days != 0 {
		parts = append(parts, integerText(days)+"d")
	}
	if hours != 0 {
		parts = append(parts, integerText(hours)+"h")
	}
	if minutes != 0 {
		parts = append(parts, integerText(minutes)+"m")
	}
	if remaining != 0 || len(parts) == 0 {
		parts = append(parts, integerText(remaining)+"s")
	}
	return sign + strings.Join(parts, " ")
}

// parseSmallInt reads a short run of digits as an int; it is used for clock and week fields that the patterns
// already restrict to a few digits.
func parseSmallInt(digits string) int {
	value, _ := strconv.Atoi(digits)
	return value
}
