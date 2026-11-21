package calc

import (
	"regexp"
	"strings"
)

// maxExpressionLength caps a query in UTF-16 units, as the original does, so hostile input is cheap to reject.
const maxExpressionLength = 500

// calculationQuery is a query with its leading "=" or "calc" removed. Explicit means the user asked for math, which
// lets any expression through.
type calculationQuery struct {
	expression string
	explicit   bool
}

var (
	calcPrefixPattern       = regexp.MustCompile(`(?i)^calc(?:ulate)?\s+`)
	leadingVerbPattern      = regexp.MustCompile(`(?i)^(?:what(?:'s| is)|calculate|calc)\s+`)
	squareRootPattern       = regexp.MustCompile(`(?i)^square\s+root\s+of\s+(.+)$`)
	squaredPattern          = regexp.MustCompile(`(?i)^(.+)\s+squared$`)
	cubedPattern            = regexp.MustCompile(`(?i)^(.+)\s+cubed$`)
	powerPattern            = regexp.MustCompile(`(?i)\bpower\b`)
	ofPattern               = regexp.MustCompile(`(?i)\bof\b`)
	mathKeywordPattern      = regexp.MustCompile(`(?i)\b(sqrt|square root|power|squared|cubed|sin|cos|tan|log|ln|abs|round|floor|ceil|min|max|percent)\b`)
	unitOperatorPattern     = regexp.MustCompile(`(?i)\d(?:\.\d+)?\s*[kmb]\b.*[+\-*/^]`)
	dateLikeSubtraction     = regexp.MustCompile(`^\d{4}-\d{1,2}(?:-\d{1,2})?$`)
	numericOperandPattern   = `[+-]?(?:(?:\d+(?:,\d{3})*(?:\.\d+)?)|(?:\.\d+))(?:\s*[kmb])?`
	numericSubtraction      = regexp.MustCompile(`(?i)(?:^|[^\w.])` + numericOperandPattern + `\s*-\s*` + numericOperandPattern + `(?:$|[^\w.])`)
	dateTimeQuestionPattern = regexp.MustCompile(`(?i)^(?:(?:days?\s+until\s+)?(?:\d{1,2}\s+[a-z]+|[a-z]+\s+\d{1,2})|\d+\s+days?\s+ago|(?:next\s+)?(?:sun|mon|tue|wed|thu|fri|sat|sunday|monday|tuesday|wednesday|thursday|friday|saturday)(?:\s+in\s+\d+\s+weeks?)?|tomorrow|yesterday|time\s+in\s+.+|\d{1,2}(?::\d{2})?\s*(?:am|pm)?\s+.+\s+in\s+.+)$`)
)

// wordOperators rewrites spoken operators into symbols, in the order the original applies them.
var wordOperators = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\bplus\b`), "+"},
	{regexp.MustCompile(`(?i)\bminus\b`), "-"},
	{regexp.MustCompile(`(?i)\btimes\b`), "*"},
	{regexp.MustCompile(`(?i)\bmultiplied\s+by\b`), "*"},
	{regexp.MustCompile(`(?i)\bdivided\s+by\b`), "/"},
	{regexp.MustCompile(`(?i)\bover\b`), "/"},
	{regexp.MustCompile(`(?i)\bpercent\b`), "%"},
}

// symbolOperators rewrites typographic operators into ASCII.
var symbolOperators = strings.NewReplacer("×", "*", "÷", "/", "−", "-")

// normalizeCalculationQuery trims a query, drops a leading "=" or "calc", and reports whether that made it explicit.
// It fails for empty or over-long queries.
func normalizeCalculationQuery(query string) (calculationQuery, bool) {
	trimmed := strings.Trim(canonicalSpace(query), " ")
	if trimmed == "" || jsLength(trimmed) > maxExpressionLength {
		return calculationQuery{}, false
	}
	withoutEquals := trimmed
	if strings.HasPrefix(trimmed, "=") {
		withoutEquals = strings.TrimLeft(trimmed[1:], " ")
	}
	withoutCalc := withoutEquals
	if loc := calcPrefixPattern.FindStringIndex(withoutEquals); loc != nil {
		withoutCalc = withoutEquals[loc[1]:]
	}
	return calculationQuery{
		expression: strings.TrimSpace(withoutCalc),
		explicit:   withoutEquals != trimmed || withoutCalc != withoutEquals,
	}, true
}

// isLikelyCalculation decides whether an unexplicit expression is worth evaluating. Plain words and bare numbers are
// not calculations, so they never reach the evaluator and never shadow other results.
func isLikelyCalculation(expression string, explicit bool) bool {
	if explicit {
		return expression != ""
	}
	natural := normalizeNaturalMath(expression)
	switch {
	case strings.ContainsAny(natural, "+*/%^()"):
		return true
	case looksLikeNumericSubtraction(natural):
		return true
	case mathKeywordPattern.MatchString(expression):
		return true
	case looksLikeDateTimeQuestion(expression):
		return true
	case looksLikeConversion(expression):
		return true
	}
	return unitOperatorPattern.MatchString(expression)
}

// looksLikeNumericSubtraction recognizes "33-18" and "10--5" while leaving ISO dates such as "2026-08-31" alone.
func looksLikeNumericSubtraction(expression string) bool {
	trimmed := strings.TrimSpace(expression)
	return !dateLikeSubtraction.MatchString(trimmed) && numericSubtraction.MatchString(trimmed)
}

// looksLikeDateTimeQuestion reports whether an expression has the shape of a calendar or clock question.
func looksLikeDateTimeQuestion(expression string) bool {
	return dateTimeQuestionPattern.MatchString(strings.TrimSpace(expression))
}

// looksLikeConversion reports whether an expression is a unit conversion or a currency question.
func looksLikeConversion(expression string) bool {
	_, isUnit := matchConversion(expression)
	return isUnit || looksLikeRateExpression(expression)
}

// normalizeNaturalMath rewrites spoken and typographic arithmetic into the symbols the evaluator reads: "plus",
// "divided by", "power", "squared", "square root of", and "of" after a percentage.
func normalizeNaturalMath(expression string) string {
	next := symbolOperators.Replace(strings.TrimSpace(expression))
	if !hasASCIILetter(next) {
		return withPercentOf(next)
	}
	next = leadingVerbPattern.ReplaceAllString(next, "")
	for _, op := range wordOperators {
		next = op.pattern.ReplaceAllString(next, op.replacement)
	}
	next = squareRootPattern.ReplaceAllString(next, "sqrt(${1})")
	next = powerPattern.ReplaceAllString(next, "^")
	next = squaredPattern.ReplaceAllString(next, "(${1})^2")
	next = cubedPattern.ReplaceAllString(next, "(${1})^3")
	return withPercentOf(next)
}

// withPercentOf turns "of" into multiplication when the expression has a percent sign, so "32% of 5" works.
func withPercentOf(next string) string {
	if strings.Contains(next, "%") {
		return ofPattern.ReplaceAllString(next, "*")
	}
	return next
}
