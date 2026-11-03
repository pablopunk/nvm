package calc

import (
	"fmt"
	"time"
)

// Kind classifies a calculator result. It decides the icon and the copy wording a view shows.
type Kind uint8

const (
	// KindMath is plain arithmetic, functions and constants.
	KindMath Kind = iota + 1
	// KindUnit is a unit conversion such as "10 ft in m".
	KindUnit
	// KindPercent is arithmetic with a percent sign, such as "15% of 240" or "19m + 47%".
	KindPercent
	// KindRate is a currency conversion from the cached exchange rates.
	KindRate
	// KindDate is a calendar answer such as "tomorrow" or "days until 25 Dec".
	KindDate
	// KindTimezone is a clock time in another zone such as "time in Tokyo".
	KindTimezone
)

// String names the kind for logs and tests.
func (k Kind) String() string {
	switch k {
	case KindMath:
		return "math"
	case KindUnit:
		return "unit"
	case KindPercent:
		return "percent"
	case KindRate:
		return "rate"
	case KindDate:
		return "date"
	case KindTimezone:
		return "timezone"
	}
	return fmt.Sprintf("kind(%d)", uint8(k))
}

// DetailLine is one labelled fact about a result, shown in a detail view.
type DetailLine struct {
	Label string
	Value string
}

// Result is one answer to a calculator query. Display is what the palette shows; Copy is what "Copy Result"
// and "Paste Result" put on the clipboard; Raw is the unformatted value used by "Copy Unformatted Result" and
// "Continue Calculation". Raw equals Display when no formatting was applied.
type Result struct {
	Kind       Kind
	Query      string
	Expression string
	Value      float64
	Display    string
	Copy       string
	Raw        string
	Title      string
	Subtitle   string
	Unit       string
	TargetUnit string
	RateUnit   string
	Timezone   string
	Date       string
	Source     string
	SwapQuery  string
	// Stale is set on rate results whose cached quote is older than the refresh window. CacheAge is the quote's
	// age; it is set only on rate results.
	Stale    bool
	CacheAge time.Duration
	Detail   []DetailLine
}

// newResult builds the fields every kind shares: the title is "expression = display" and copy is the display.
func newResult(kind Kind, query, expression string, value float64, raw, display, subtitle string) Result {
	return Result{
		Kind:       kind,
		Query:      query,
		Expression: expression,
		Value:      value,
		Display:    display,
		Copy:       display,
		Raw:        raw,
		Title:      expression + " = " + display,
		Subtitle:   subtitle,
	}
}

// describe fills Detail from the result's fields. Builders call it last, after the kind-specific fields are set.
func describe(r Result) Result {
	lines := []DetailLine{{Label: "Expression", Value: r.Expression}, {Label: "Result", Value: r.Display}}
	add := func(label, value string) {
		if value != "" {
			lines = append(lines, DetailLine{Label: label, Value: value})
		}
	}
	if r.Raw != r.Display {
		add("Unformatted", r.Raw)
	}
	add("Unit", r.Unit)
	add("Target unit", r.TargetUnit)
	add("Per", r.RateUnit)
	add("Date", r.Date)
	add("Timezone", r.Timezone)
	add("Source", r.Source)
	if r.Kind == KindRate && r.CacheAge > 0 {
		add("Rate age", ageLabel(r.CacheAge))
	}
	add("Swap", r.SwapQuery)
	if r.Stale {
		lines = append(lines, DetailLine{Label: "Stale", Value: "cached rate is older than a day"})
	}
	r.Detail = lines
	return r
}

// ageLabel describes how old a cached quote is in the words the palette uses. It is empty for a quote dated in
// the future.
func ageLabel(age time.Duration) string {
	if age < 0 {
		return ""
	}
	hours := int64(age / time.Hour)
	switch {
	case hours < 1:
		return "fresh"
	case hours < 48:
		return fmt.Sprintf("%dh old", hours)
	}
	return fmt.Sprintf("%dd old", hours/24)
}
