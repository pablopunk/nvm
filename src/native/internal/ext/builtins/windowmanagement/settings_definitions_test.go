package windowmanagement

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func definitionsByID(start Settings) map[string]api.SettingDefinition {
	byID := map[string]api.SettingDefinition{}
	for _, definition := range SettingDefinitions(start) {
		byID[definition.ID] = definition
	}
	return byID
}

// TestSettingsDeclareTheirRangesAndChoices checks the host learns each range and choice list from the definitions.
func TestSettingsDeclareTheirRangesAndChoices(t *testing.T) {
	if err := New(DefaultSettings()).Manifest().ValidateBuiltIn(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	byID := definitionsByID(DefaultSettings())
	step := byID["resizeStepPercent"]
	if step.Min == nil || step.Max == nil || *step.Min != 1 || *step.Max != 25 {
		t.Fatalf("resize step = %+v; want the range 1 to 25", step)
	}
	if top := byID["outerGapTop"]; top.Max == nil || *top.Max != maxGap || top.CheckValue([]byte("null")) != nil {
		t.Fatalf("outer gap top = %+v; want a gap range that still accepts a blank", top)
	}
	if order := byID["cycleOrder"]; len(order.Choices) != 2 || order.CheckValue([]byte(`"thirds-only"`)) == nil {
		t.Fatalf("cycle order = %+v; want two choices and an unknown one refused", order)
	}
}

// TestStartValueOutsideItsRangeDeclaresTheDefault checks a legacy value the range refuses does not break the manifest.
func TestStartValueOutsideItsRangeDeclaresTheDefault(t *testing.T) {
	start := DefaultSettings()
	start.OuterGap, start.CycleOrder = 400, "unknown"
	byID := definitionsByID(start)
	if string(byID["outerGap"].Value) != "8" || string(byID["cycleOrder"].Value) != `"`+cycleHalfTwoThirdsThird+`"` {
		t.Fatalf("defaults = %s, %s; want the built-in defaults", byID["outerGap"].Value, byID["cycleOrder"].Value)
	}
	if err := New(start).Manifest().ValidateBuiltIn(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}
