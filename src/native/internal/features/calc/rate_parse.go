package calc

import (
	"regexp"
	"strings"
)

// RateExpression is a currency question such as "10 usd in gbp" or "8 dollars/hour in gbp", parsed but not yet
// priced.
type RateExpression struct {
	Amount         float64
	SourceCurrency string
	TargetCurrency string
	Expression     string
	RateUnit       string
}

// The pieces of a currency question. An amount may carry a k, m or b suffix; a currency is a symbol or a word of
// three or more letters; a rate unit follows a slash.
const (
	amountPattern    = `([+-]?(?:(?:\d+(?:,\d{3})*)|(?:\d+)|(?:\d*\.\d+))(?:\.\d+)?\s*[kmb]?)`
	currencyPattern  = `([$€£¥₹]|[a-zA-Z]{3,})`
	rateUnitPattern  = `(?:\s*/\s*([a-zA-Z]+))?`
	connectorPattern = `\s+(?:in|to|as)\s+`
)

// Compiled currency question shapes. The groups are: suffix form amount, currency, rate unit, target; prefix form
// currency, amount, rate unit, target; default suffix and default prefix form amount, symbol.
var (
	suffixRatePattern        = regexp.MustCompile(`(?i)^` + amountPattern + `\s*` + currencyPattern + rateUnitPattern + connectorPattern + currencyPattern + `$`)
	prefixRatePattern        = regexp.MustCompile(`(?i)^` + currencyPattern + `\s*` + amountPattern + rateUnitPattern + connectorPattern + currencyPattern + `$`)
	defaultSuffixRatePattern = regexp.MustCompile(`(?i)^` + amountPattern + `\s*([$€])$`)
	defaultPrefixRatePattern = regexp.MustCompile(`(?i)^([$€])\s*` + amountPattern + `$`)
)

// rateShape is the regex groups of one currency question, with empty strings for groups the shape lacks.
type rateShape struct {
	amount   string
	source   string
	rateUnit string
	target   string
}

// matchRateShape tries each question shape in the order the original does and returns the first that matches.
func matchRateShape(expression string) (rateShape, bool) {
	if g := suffixRatePattern.FindStringSubmatch(expression); g != nil {
		return rateShape{amount: g[1], source: g[2], rateUnit: g[3], target: g[4]}, true
	}
	if g := prefixRatePattern.FindStringSubmatch(expression); g != nil {
		return rateShape{amount: g[2], source: g[1], rateUnit: g[3], target: g[4]}, true
	}
	if g := defaultSuffixRatePattern.FindStringSubmatch(expression); g != nil {
		return rateShape{amount: g[1], source: g[2]}, true
	}
	if g := defaultPrefixRatePattern.FindStringSubmatch(expression); g != nil {
		return rateShape{amount: g[2], source: g[1]}, true
	}
	return rateShape{}, false
}

// parseRateExpression reads a currency question. It fails for anything that is not one, for an unknown source or
// target, and for a conversion from a currency to itself.
func parseRateExpression(query string) (RateExpression, bool) {
	normalized, ok := normalizeCalculationQuery(query)
	if !ok {
		return RateExpression{}, false
	}
	expression := normalized.expression
	shape, ok := matchRateShape(expression)
	if !ok || shape.amount == "" || shape.source == "" {
		return RateExpression{}, false
	}
	source, ok := currencyForAlias(shape.source)
	if !ok {
		return RateExpression{}, false
	}
	target, ok := currencyForAlias(shape.target)
	if shape.target == "" {
		target, ok = defaultTargetCurrency(source)
	}
	if !ok || source == target {
		return RateExpression{}, false
	}
	amount, ok := parseAmountWithSuffix(shape.amount)
	if !ok {
		return RateExpression{}, false
	}
	parsed := RateExpression{Amount: amount, SourceCurrency: source, TargetCurrency: target, Expression: strings.TrimSpace(expression)}
	if shape.rateUnit != "" {
		parsed.RateUnit = normalizeRateUnit(shape.rateUnit)
	}
	return parsed, true
}

// looksLikeRateExpression reports whether an expression parses as a currency question.
func looksLikeRateExpression(expression string) bool {
	_, ok := parseRateExpression(expression)
	return ok
}
