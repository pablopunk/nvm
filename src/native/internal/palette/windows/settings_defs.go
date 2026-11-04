package windows

import (
	"fmt"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// settingKind is how a setting is edited: typed as a whole number, flipped, or cycled through its options.
type settingKind int

const (
	kindNumber settingKind = iota
	kindFlag
	kindChoice
)

const (
	gapSection      = "Gaps"
	sizesSection    = "Sizes"
	behaviorSection = "Behavior"
	displaySection  = "Displays"
	maxGap          = 40
)

// settingDef is one window management setting: its copy and how it reads and writes. A number is a whole number
// in its unit. A nullable number (an edge gap) inherits the outer gap while it is blank.
type settingDef struct {
	title    string
	section  string
	kind     settingKind
	unit     string
	min, max int

	number   func(s *geometry.Settings) *float64
	override func(s *geometry.Settings) **float64
	flag     func(s *geometry.Settings) *bool
	options  []string
	get      func(s *geometry.Settings) string
	set      func(s *geometry.Settings, value string)
}

func numberSetting(title, section, unit string, min, max int, field func(*geometry.Settings) *float64) settingDef {
	return settingDef{title: title, section: section, kind: kindNumber, unit: unit, min: min, max: max, number: field}
}

func overrideSetting(title string, field func(*geometry.Settings) **float64) settingDef {
	return settingDef{title: title, section: gapSection, kind: kindNumber, unit: "px", min: 0, max: maxGap, override: field}
}

func flagSetting(title, section string, field func(*geometry.Settings) *bool) settingDef {
	return settingDef{title: title, section: section, kind: kindFlag, flag: field}
}

func choiceSetting(title, section string, options []string, get func(*geometry.Settings) string, set func(*geometry.Settings, string)) settingDef {
	return settingDef{title: title, section: section, kind: kindChoice, options: options, get: get, set: set}
}

// settingDefs lists the settings in the order the Electron settings view shows them.
var settingDefs = []settingDef{
	numberSetting("Outer Gap", gapSection, "px", 0, maxGap, func(s *geometry.Settings) *float64 { return &s.OuterGap }),
	overrideSetting("Top Gap", func(s *geometry.Settings) **float64 { return &s.OuterGapTop }),
	overrideSetting("Right Gap", func(s *geometry.Settings) **float64 { return &s.OuterGapRight }),
	overrideSetting("Bottom Gap", func(s *geometry.Settings) **float64 { return &s.OuterGapBottom }),
	overrideSetting("Left Gap", func(s *geometry.Settings) **float64 { return &s.OuterGapLeft }),
	numberSetting("Inner Gap", gapSection, "px", 0, maxGap, func(s *geometry.Settings) *float64 { return &s.InnerGap }),
	flagSetting("Gap on Maximize", gapSection, func(s *geometry.Settings) *bool { return &s.ApplyGapToMaximize }),
	numberSetting("Almost Maximize Width", sizesSection, "%", 50, 100, func(s *geometry.Settings) *float64 { return &s.AlmostMaximizeWidthPercent }),
	numberSetting("Almost Maximize Height", sizesSection, "%", 50, 100, func(s *geometry.Settings) *float64 { return &s.AlmostMaximizeHeightPercent }),
	numberSetting("Reasonable Size Width", sizesSection, "%", 30, 100, func(s *geometry.Settings) *float64 { return &s.ReasonableSizeWidthPercent }),
	numberSetting("Reasonable Size Height", sizesSection, "%", 30, 100, func(s *geometry.Settings) *float64 { return &s.ReasonableSizeHeightPercent }),
	numberSetting("Center Half Width", sizesSection, "%", 20, 90, func(s *geometry.Settings) *float64 { return &s.CenterHalfWidthPercent }),
	numberSetting("Resize Step", sizesSection, "%", 1, 25, func(s *geometry.Settings) *float64 { return &s.ResizeStepPercent }),
	flagSetting("Cycle Half Commands", behaviorSection, func(s *geometry.Settings) *bool { return &s.Cycle }),
	choiceSetting("Cycle Order", behaviorSection,
		[]string{string(geometry.CycleHalfTwoThirdsThird), string(geometry.CycleHalfThirdTwoThirds)},
		func(s *geometry.Settings) string { return string(s.CycleOrder) },
		func(s *geometry.Settings, value string) { s.CycleOrder = geometry.CycleOrder(value) }),
	choiceSetting("Display Order", displaySection,
		[]string{string(geometry.DisplayOrderLeftToRight), string(geometry.DisplayOrderTopToBottom)},
		func(s *geometry.Settings) string { return string(s.DisplayOrder) },
		func(s *geometry.Settings, value string) { s.DisplayOrder = geometry.DisplayOrder(value) }),
	choiceSetting("Display Move Sizing", displaySection,
		[]string{string(geometry.MoveProportional), string(geometry.MoveKeepSize)},
		func(s *geometry.Settings) string { return string(s.DisplayMoveSizing) },
		func(s *geometry.Settings, value string) { s.DisplayMoveSizing = geometry.DisplayMoveSizing(value) }),
}

// read returns what the row shows for the setting.
func (def settingDef) read(s geometry.Settings) string {
	switch def.kind {
	case kindFlag:
		return onOff(*def.flag(&s))
	case kindChoice:
		return def.get(&s)
	}
	if def.override != nil {
		return overrideText(*def.override(&s))
	}
	return fmt.Sprintf("%g%s", *def.number(&s), def.unit)
}

func overrideText(value *float64) string {
	if value == nil {
		return "Follows Outer Gap"
	}
	return fmt.Sprintf("%gpx", *value)
}

func onOff(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}

// parse reads a typed value for a number setting. It returns the value, or the copy to show when the text is
// not a whole number in range. A nullable setting takes a blank as "follow the outer gap".
func (def settingDef) parse(text string) (*float64, string) {
	if def.override != nil && text == "" {
		return nil, ""
	}
	whole, err := strconv.Atoi(text)
	if err != nil || whole < def.min || whole > def.max {
		return nil, fmt.Sprintf("Use a whole number from %d to %d.", def.min, def.max)
	}
	value := float64(whole)
	return &value, ""
}

// assign writes a parsed number into the settings. A nil value clears an override.
func (def settingDef) assign(s *geometry.Settings, value *float64) {
	if def.override != nil {
		*def.override(s) = value
		return
	}
	if value != nil {
		*def.number(s) = *value
	}
}

// toggle flips a flag setting.
func (def settingDef) toggle(s *geometry.Settings) {
	flag := def.flag(s)
	*flag = !*flag
}

// cycle moves a choice setting to its next option.
func (def settingDef) cycle(s *geometry.Settings) {
	current := def.get(s)
	next := def.options[0]
	for i, option := range def.options {
		if option == current {
			next = def.options[(i+1)%len(def.options)]
		}
	}
	def.set(s, next)
}
