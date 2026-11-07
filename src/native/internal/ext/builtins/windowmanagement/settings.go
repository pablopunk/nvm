package windowmanagement

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Settings are the window management settings the layouts read: the gaps, the sizes of the sized layouts, how repeated
// halves cycle and how displays are walked. Each one is a declared setting of the built-in, so the settings view and
// the host's storage hold it.
type Settings struct {
	OuterGap                    float64
	OuterGapTop                 *float64
	OuterGapRight               *float64
	OuterGapBottom              *float64
	OuterGapLeft                *float64
	InnerGap                    float64
	ApplyGapToMaximize          bool
	AlmostMaximizeWidthPercent  float64
	AlmostMaximizeHeightPercent float64
	ReasonableSizeWidthPercent  float64
	ReasonableSizeHeightPercent float64
	CenterHalfWidthPercent      float64
	ResizeStepPercent           float64
	Cycle                       bool
	CycleOrder                  string
	DisplayOrder                string
	DisplayMoveSizing           string
}

// DefaultSettings are the defaults of a fresh install, as the Electron app shipped them.
func DefaultSettings() Settings {
	return Settings{
		OuterGap: 8, InnerGap: 4, ApplyGapToMaximize: true,
		AlmostMaximizeWidthPercent: 90, AlmostMaximizeHeightPercent: 90,
		ReasonableSizeWidthPercent: 70, ReasonableSizeHeightPercent: 70,
		CenterHalfWidthPercent: 50, ResizeStepPercent: 5,
		Cycle: true, CycleOrder: cycleHalfTwoThirdsThird, DisplayOrder: "left-to-right", DisplayMoveSizing: "proportional",
	}
}

// Cycle orders and display choices, as the settings declare them.
const (
	cycleHalfTwoThirdsThird = "half-two-thirds-third"
	cycleHalfThirdTwoThirds = "half-third-two-thirds"
	displayTopToBottom      = "top-to-bottom"
	moveKeepSize            = "keep-size"
)

// fieldKind is how a setting is stored and edited: a number, a flag, one of a list of text choices, or an optional number
// that inherits the outer gap while it is blank.
type fieldKind int

const (
	kindNumber fieldKind = iota
	kindFlag
	kindChoice
	kindOptionalGap
)

// binding ties one declared setting to its field in a Settings value, so reading, writing and declaring share one list.
type binding struct {
	id       string
	title    string
	section  string
	kind     fieldKind
	unit     string
	min, max float64
	options  []string

	number *float64
	flag   *bool
	text   *string
	gap    **float64
}

const (
	sectionGaps     = "Gaps"
	sectionSizes    = "Sizes"
	sectionBehavior = "Behavior"
	sectionDisplays = "Displays"
	maxGap          = 40
)

// bindings lists the settings in the order the settings view shows them, each bound to its field of s.
func bindings(s *Settings) []binding {
	return []binding{
		{id: "outerGap", title: "Outer Gap", section: sectionGaps, kind: kindNumber, unit: "px", min: 0, max: maxGap, number: &s.OuterGap},
		{id: "outerGapTop", title: "Top Gap", section: sectionGaps, kind: kindOptionalGap, unit: "px", min: 0, max: maxGap, gap: &s.OuterGapTop},
		{id: "outerGapRight", title: "Right Gap", section: sectionGaps, kind: kindOptionalGap, unit: "px", min: 0, max: maxGap, gap: &s.OuterGapRight},
		{id: "outerGapBottom", title: "Bottom Gap", section: sectionGaps, kind: kindOptionalGap, unit: "px", min: 0, max: maxGap, gap: &s.OuterGapBottom},
		{id: "outerGapLeft", title: "Left Gap", section: sectionGaps, kind: kindOptionalGap, unit: "px", min: 0, max: maxGap, gap: &s.OuterGapLeft},
		{id: "innerGap", title: "Inner Gap", section: sectionGaps, kind: kindNumber, unit: "px", min: 0, max: maxGap, number: &s.InnerGap},
		{id: "applyGapToMaximize", title: "Gap on Maximize", section: sectionGaps, kind: kindFlag, flag: &s.ApplyGapToMaximize},
		{id: "almostMaximizeWidthPercent", title: "Almost Maximize Width", section: sectionSizes, kind: kindNumber, unit: "%", min: 50, max: 100, number: &s.AlmostMaximizeWidthPercent},
		{id: "almostMaximizeHeightPercent", title: "Almost Maximize Height", section: sectionSizes, kind: kindNumber, unit: "%", min: 50, max: 100, number: &s.AlmostMaximizeHeightPercent},
		{id: "reasonableSizeWidthPercent", title: "Reasonable Size Width", section: sectionSizes, kind: kindNumber, unit: "%", min: 30, max: 100, number: &s.ReasonableSizeWidthPercent},
		{id: "reasonableSizeHeightPercent", title: "Reasonable Size Height", section: sectionSizes, kind: kindNumber, unit: "%", min: 30, max: 100, number: &s.ReasonableSizeHeightPercent},
		{id: "centerHalfWidthPercent", title: "Center Half Width", section: sectionSizes, kind: kindNumber, unit: "%", min: 20, max: 90, number: &s.CenterHalfWidthPercent},
		{id: "resizeStepPercent", title: "Resize Step", section: sectionSizes, kind: kindNumber, unit: "%", min: 1, max: 25, number: &s.ResizeStepPercent},
		{id: "cycle", title: "Cycle Half Commands", section: sectionBehavior, kind: kindFlag, flag: &s.Cycle},
		{id: "cycleOrder", title: "Cycle Order", section: sectionBehavior, kind: kindChoice, options: []string{cycleHalfTwoThirdsThird, cycleHalfThirdTwoThirds}, text: &s.CycleOrder},
		{id: "displayOrder", title: "Display Order", section: sectionDisplays, kind: kindChoice, options: []string{"left-to-right", displayTopToBottom}, text: &s.DisplayOrder},
		{id: "displayMoveSizing", title: "Display Move Sizing", section: sectionDisplays, kind: kindChoice, options: []string{"proportional", moveKeepSize}, text: &s.DisplayMoveSizing},
	}
}

// SettingDefinitions declares the settings with the values the built-in starts from: the defaults, or the values the
// settings file held before the built-in kept its own.
func SettingDefinitions(start Settings) []api.SettingDefinition {
	fields := bindings(&start)
	definitions := make([]api.SettingDefinition, 0, len(fields))
	for _, field := range fields {
		definitions = append(definitions, definitionOf(field))
	}
	return definitions
}

func definitionOf(field binding) api.SettingDefinition {
	definition := api.SettingDefinition{ID: field.id, Title: field.title, Value: valueJSON(field)}
	switch field.kind {
	case kindNumber:
		definition.Type = api.SettingNumber
	case kindFlag:
		definition.Type = api.SettingBoolean
	case kindChoice:
		definition.Type = api.SettingString
	}
	return definition
}

// fieldValue is the value a field holds, as the host stores it: a number, a flag, a text choice, or nil for a blank gap.
func fieldValue(field binding) any {
	switch field.kind {
	case kindNumber:
		return *field.number
	case kindFlag:
		return *field.flag
	case kindChoice:
		return *field.text
	case kindOptionalGap:
		if *field.gap != nil {
			return **field.gap
		}
	}
	return nil
}

func valueJSON(field binding) json.RawMessage {
	raw, _ := json.Marshal(fieldValue(field))
	return raw
}

// ResetSettings writes every setting of a section, or every setting when section is empty, back to its default.
func ResetSettings(ctx api.Context, section string) error {
	defaults := DefaultSettings()
	var firstErr error
	for _, field := range bindings(&defaults) {
		if section != "" && field.section != section {
			continue
		}
		if err := ctx.Settings().Set(field.id, fieldValue(field)); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// LoadSettings reads each declared setting from the host, keeping the start value of any it cannot read.
func LoadSettings(ctx api.Context, start Settings) Settings {
	loaded := start
	for _, field := range bindings(&loaded) {
		readInto(ctx, field)
	}
	return loaded
}

func readInto(ctx api.Context, field binding) {
	switch field.kind {
	case kindNumber:
		_ = ctx.Settings().Get(field.id, field.number)
	case kindFlag:
		_ = ctx.Settings().Get(field.id, field.flag)
	case kindChoice:
		_ = ctx.Settings().Get(field.id, field.text)
	case kindOptionalGap:
		_ = ctx.Settings().Get(field.id, field.gap)
	}
}

// optionsFor are the wire options a layout takes from the settings, as 1.x sent them.
func optionsFor(s Settings) api.WindowManagementOptions {
	gap := api.Edges{Top: edgeGap(s.OuterGapTop, s.OuterGap), Right: edgeGap(s.OuterGapRight, s.OuterGap),
		Bottom: edgeGap(s.OuterGapBottom, s.OuterGap), Left: edgeGap(s.OuterGapLeft, s.OuterGap)}
	inner := int(s.InnerGap)
	applyGap, cycle := s.ApplyGapToMaximize, s.Cycle
	return api.WindowManagementOptions{
		OuterGap: &gap, InnerGap: &inner, ApplyGapToMaximize: &applyGap, Cycle: &cycle,
		CycleOrder: s.CycleOrder, DisplayOrder: s.DisplayOrder, DisplayMoveSizing: s.DisplayMoveSizing,
	}
}

// edgeGap is one edge's gap: its own value, else the outer gap, whole points.
func edgeGap(edge *float64, outer float64) int {
	if edge != nil {
		return int(*edge)
	}
	return int(outer)
}
