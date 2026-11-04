package geometry

import (
	"reflect"
	"sort"
	"testing"
)

// Ported from src/app/extensions/window-management-exact-values.test.ts.

const (
	unitPointsText  = "points"
	unitPercentText = "percent"
)

func inputsFor(field ExactField, text, unit string) ExactInputs {
	input := &ExactInput{Text: text, Unit: unit}
	var inputs ExactInputs
	switch field {
	case FieldX:
		inputs.X = input
	case FieldY:
		inputs.Y = input
	case FieldWidth:
		inputs.Width = input
	default:
		inputs.Height = input
	}
	return inputs
}

func valueOf(values ExactValues, field ExactField) *Dimension {
	return exactDimensionFor(values, field)
}

func exactDimensionFor(values ExactValues, field ExactField) *Dimension {
	switch field {
	case FieldX:
		return values.X
	case FieldY:
		return values.Y
	case FieldWidth:
		return values.Width
	}
	return values.Height
}

func TestBlankFieldsAreOmittedPort(t *testing.T) {
	result := ParseExactDimensions(ExactInputs{
		X: &ExactInput{Text: "", Unit: unitPointsText},
		Y: &ExactInput{Text: "  ", Unit: unitPointsText},
	})
	if !result.Failed() {
		t.Fatal("expected a failure for a form with no values")
	}
	if result.Message != "Enter at least one value." {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestSingleProvidedValueLeavesOtherDimensionsOutPort(t *testing.T) {
	result := ParseExactDimensions(ExactInputs{
		X: &ExactInput{Text: "120", Unit: unitPointsText},
		Y: &ExactInput{Text: "", Unit: unitPointsText},
	})
	if result.Failed() {
		t.Fatalf("unexpected failure: %+v", result)
	}
	got := ExactLayoutFor(result.Dimensions)
	if got.Kind != KindExact || got.ExactX == nil || *got.ExactX != (Dimension{Value: 120, Unit: UnitPoints}) ||
		got.ExactY != nil || got.ExactWidth != nil || got.ExactHeight != nil {
		t.Fatalf("layout = %+v", got)
	}
}

type exactValueCase struct {
	name     string
	field    ExactField
	text     string
	unit     string
	expected Dimension
}

var validExactValueCases = []exactValueCase{
	{"points offset at zero", FieldX, "0", unitPointsText, Dimension{Value: 0, Unit: UnitPoints}},
	{"points offset at the cap", FieldY, "100000", unitPointsText, Dimension{Value: 100_000, Unit: UnitPoints}},
	{"percent offset at 100", FieldX, "100", unitPercentText, Dimension{Value: 100, Unit: UnitPercent}},
	{"decimal percent size", FieldWidth, "33.5", unitPercentText, Dimension{Value: 33.5, Unit: UnitPercent}},
	{"points size just above zero", FieldHeight, "0.5", unitPointsText, Dimension{Value: 0.5, Unit: UnitPoints}},
	{"padded text", FieldWidth, " 640 ", unitPointsText, Dimension{Value: 640, Unit: UnitPoints}},
}

func TestValidExactValuesPort(t *testing.T) {
	for _, c := range validExactValueCases {
		t.Run("accepts "+c.name, func(t *testing.T) {
			result := ParseExactDimensions(inputsFor(c.field, c.text, c.unit))
			if result.Failed() {
				t.Fatalf("unexpected failure: %+v", result)
			}
			got := valueOf(result.Dimensions, c.field)
			if got == nil || *got != c.expected {
				t.Fatalf("dimension = %+v, want %+v", got, c.expected)
			}
		})
	}
}

type invalidExactValueCase struct {
	name    string
	field   ExactField
	text    string
	unit    string
	message string
}

var invalidExactValueCases = []invalidExactValueCase{
	{"negative points offset", FieldX, "-1", unitPointsText, "Enter a number from 0 to 100000."},
	{"points offset above cap", FieldX, "100001", unitPointsText, "Enter a number from 0 to 100000."},
	{"percent offset above 100", FieldY, "101", unitPercentText, "Enter a number from 0 to 100%."},
	{"zero size", FieldWidth, "0", unitPointsText, "Enter a number above 0, up to 100000."},
	{"zero percent size", FieldHeight, "0", unitPercentText, "Enter a number above 0, up to 100%."},
	{"percent size above 100", FieldWidth, "150", unitPercentText, "Enter a number above 0, up to 100%."},
	{"non-numeric text", FieldX, "abc", unitPointsText, "Enter a number from 0 to 100000."},
	{"scientific notation", FieldX, "1e3", unitPointsText, "Enter a number from 0 to 100000."},
	{"infinite text", FieldWidth, "Infinity", unitPointsText, "Enter a number above 0, up to 100000."},
}

func TestInvalidExactValuesPort(t *testing.T) {
	for _, c := range invalidExactValueCases {
		t.Run("rejects "+c.name+" with a range message", func(t *testing.T) {
			result := ParseExactDimensions(inputsFor(c.field, c.text, c.unit))
			if !result.Failed() {
				t.Fatal("expected a failure")
			}
			if result.Errors[c.field] != c.message {
				t.Fatalf("error = %q, want %q", result.Errors[c.field], c.message)
			}
		})
	}
}

func TestUnknownUnitIsRejectedWithChoiceMessagePort(t *testing.T) {
	result := ParseExactDimensions(inputsFor(FieldWidth, "50", "inches"))
	if result.Errors[FieldWidth] != "Choose points or percent." {
		t.Fatalf("error = %q", result.Errors[FieldWidth])
	}
}

func TestUnitIsIgnoredWhenItsValueIsBlankPort(t *testing.T) {
	result := ParseExactDimensions(ExactInputs{
		Width:  &ExactInput{Text: "", Unit: "inches"},
		Height: &ExactInput{Text: "50", Unit: unitPercentText},
	})
	if result.Failed() {
		t.Fatalf("unexpected failure: %+v", result)
	}
	want := ExactValues{Height: &Dimension{Value: 50, Unit: UnitPercent}}
	if !reflect.DeepEqual(result.Dimensions, want) {
		t.Fatalf("dimensions = %+v, want %+v", result.Dimensions, want)
	}
}

func TestEveryInvalidFieldIsReportedPort(t *testing.T) {
	result := ParseExactDimensions(ExactInputs{
		X: &ExactInput{Text: "-4", Unit: unitPointsText},
		Y: &ExactInput{Text: "200", Unit: unitPercentText},
	})
	keys := make([]string, 0, len(result.Errors))
	for field := range result.Errors {
		keys = append(keys, string(field))
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"x", "y"}) {
		t.Fatalf("error keys = %v, want [x y]", keys)
	}
}

func TestFormatExactDimensionCopy(t *testing.T) {
	if got := FormatExactDimension(Dimension{Value: 50, Unit: UnitPercent}); got != "50%" {
		t.Fatalf("percent = %q", got)
	}
	if got := FormatExactDimension(Dimension{Value: 120, Unit: UnitPoints}); got != "120 pt" {
		t.Fatalf("points = %q", got)
	}
	if got := FormatExactDimension(Dimension{Value: 33.5, Unit: UnitPercent}); got != "33.5%" {
		t.Fatalf("decimal = %q", got)
	}
}
