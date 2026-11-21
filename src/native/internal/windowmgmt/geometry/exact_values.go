package geometry

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ExactInput is one raw form field: the text the user typed and the unit they chose.
type ExactInput struct {
	Text string
	Unit string
}

// ExactInputs are the four raw exact fields. A nil field was not shown or not filled in.
type ExactInputs struct {
	X, Y, Width, Height *ExactInput
}

// ExactValues are the parsed dimensions. A nil field was left blank and keeps the window's value.
type ExactValues struct {
	X, Y, Width, Height *Dimension
}

// ExactParseResult is the outcome of parsing the exact fields. Errors are keyed by field; Message is
// the whole-form message used when nothing was entered.
type ExactParseResult struct {
	Dimensions ExactValues
	Errors     map[ExactField]string
	Message    string
}

// decimalTextPattern accepts plain decimal numbers only: no sign, no exponent, no words.
var decimalTextPattern = regexp.MustCompile(`^\d+(\.\d+)?$`)

const (
	chooseUnitCopy = "Choose points or percent."
	enterOneCopy   = "Enter at least one value."
)

// formatNumber renders a number the way JavaScript prints an integer-valued number.
func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func isBlankText(text string) bool {
	return strings.TrimSpace(text) == ""
}

// parseDimensionText converts entered text to a number, or NaN when it is not a plain decimal.
func parseDimensionText(text string) float64 {
	trimmed := strings.TrimSpace(text)
	if !decimalTextPattern.MatchString(trimmed) {
		return math.NaN()
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return math.NaN()
	}
	return value
}

// exactDimensionRangeMessage is the range hint shown under a field, e.g. "Enter a number from 0 to 100%.".
func exactDimensionRangeMessage(field ExactField, unit DimensionUnit) string {
	axis := kindSize
	if field == FieldX || field == FieldY {
		axis = kindOffset
	}
	limit := formatNumber(maxExactValue(unit))
	if unit == UnitPercent {
		limit += "%"
	}
	if axis == kindSize {
		return "Enter a number above 0, up to " + limit + "."
	}
	return "Enter a number from 0 to " + limit + "."
}

// exactDimensionInRange reports whether a parsed value is allowed for its field and unit.
func exactDimensionInRange(field ExactField, dimension Dimension) bool {
	if math.IsNaN(dimension.Value) || math.IsInf(dimension.Value, 0) {
		return false
	}
	if dimension.Value > maxExactValue(dimension.Unit) {
		return false
	}
	if exactKindOf[field] == kindSize {
		return dimension.Value > 0
	}
	return dimension.Value >= 0
}

func parseExactField(field ExactField, input ExactInput) (*Dimension, string) {
	if isBlankText(input.Text) {
		return nil, ""
	}
	unit := DimensionUnit(input.Unit)
	if unit != UnitPoints && unit != UnitPercent {
		return nil, chooseUnitCopy
	}
	dimension := Dimension{Value: parseDimensionText(input.Text), Unit: unit}
	if !exactDimensionInRange(field, dimension) {
		return nil, exactDimensionRangeMessage(field, unit)
	}
	return &dimension, ""
}

// ParseExactDimensions parses the four form fields. Blank fields are skipped; every invalid field is
// reported, not only the first. With no errors and no values it asks for at least one value.
func ParseExactDimensions(inputs ExactInputs) ExactParseResult {
	result := ExactParseResult{Errors: map[ExactField]string{}}
	fields := map[ExactField]*ExactInput{
		FieldX: inputs.X, FieldY: inputs.Y, FieldWidth: inputs.Width, FieldHeight: inputs.Height,
	}
	for _, field := range exactFields {
		input := fields[field]
		if input == nil {
			continue
		}
		dimension, message := parseExactField(field, *input)
		if message != "" {
			result.Errors[field] = message
		}
		if dimension != nil {
			setExactValue(&result.Dimensions, field, dimension)
		}
	}
	if len(result.Errors) > 0 {
		return result
	}
	if result.Dimensions.count() == 0 {
		result.Message = enterOneCopy
	}
	return result
}

func setExactValue(values *ExactValues, field ExactField, dimension *Dimension) {
	switch field {
	case FieldX:
		values.X = dimension
	case FieldY:
		values.Y = dimension
	case FieldWidth:
		values.Width = dimension
	default:
		values.Height = dimension
	}
}

func (v ExactValues) count() int {
	count := 0
	for _, dimension := range []*Dimension{v.X, v.Y, v.Width, v.Height} {
		if dimension != nil {
			count++
		}
	}
	return count
}

// Failed reports whether the form has a field error or a whole-form message.
func (r ExactParseResult) Failed() bool {
	return len(r.Errors) > 0 || r.Message != ""
}

// ExactLayoutFor builds the exact layout for parsed values.
func ExactLayoutFor(values ExactValues) Layout {
	return exactLayout(values.X, values.Y, values.Width, values.Height)
}

// FormatExactDimension renders a value for display: "50%" or "120 pt".
func FormatExactDimension(dimension Dimension) string {
	if dimension.Unit == UnitPercent {
		return formatNumber(dimension.Value) + "%"
	}
	return formatNumber(dimension.Value) + " pt"
}
