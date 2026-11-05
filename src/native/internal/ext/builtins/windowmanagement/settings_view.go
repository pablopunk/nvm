package windowmanagement

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// settingsSections are the sections the settings view groups its rows under, in order.
var settingsSections = []string{sectionGaps, sectionSizes, sectionBehavior, sectionDisplays}

// settingsView lists every setting with its value. A flag toggles, a choice cycles, and a number opens a prompt.
func settingsView(ctx api.Context, start Settings) api.View {
	current := LoadSettings(ctx, start)
	fields := bindings(&current)
	items := make([]api.Item, 0, len(fields)+len(settingsSections)+1)
	for _, field := range fields {
		items = append(items, settingItem(ctx, field))
	}
	items = append(items, resetItems()...)
	return api.ListView("Window Management Settings", items...)
}

func settingItem(ctx api.Context, field binding) api.Item {
	item := api.Item{ID: field.id, Title: field.title, Subtitle: field.section + " · " + valueText(field), Icon: "sliders-horizontal"}
	switch field.kind {
	case kindFlag:
		toggle := api.Run(settingToggle, "Toggle", map[string]string{"id": field.id})
		item.Primary = &toggle
	case kindChoice:
		cycle := api.Run(settingCycle, "Change", map[string]string{"id": field.id})
		item.Primary = &cycle
	default:
		prompt := numberPrompt(ctx, field)
		item.Primary = &prompt
	}
	return item
}

// numberPrompt asks for the value of a number setting. A blank optional gap keeps the outer gap.
func numberPrompt(ctx api.Context, field binding) api.Action {
	hint := fmt.Sprintf("Enter a number from %s to %s%s.", formatNumber(field.min), formatNumber(field.max), unitSuffix(field))
	if field.kind == kindOptionalGap {
		hint = "Blank inherits the outer gap. " + hint
	}
	return ctx.Input().Prompt(api.PromptSpec{
		Title: field.title, Message: hint, SubmitTitle: "Save",
		Fields: []api.Field{{
			ID: "value", Label: field.title, Type: api.FieldNumber, Value: api.TextValue(currentText(field)),
		}},
		Action: api.Run(settingSet, field.title, map[string]string{"id": field.id}),
	})
}

// resetItems are the Reset to Defaults row and one row per section.
func resetItems() []api.Item {
	items := []api.Item{{
		ID: "reset-all", Title: "Reset to Defaults", Subtitle: "Restore every window management setting to its default.", Icon: "undo-2",
		Primary: ptrAction(api.Run(settingsReset, "Reset to Defaults", nil)),
	}}
	for _, section := range settingsSections {
		items = append(items, api.Item{
			ID: "reset-" + strings.ToLower(section), Title: "Reset " + section, Icon: "undo-2",
			Subtitle: fmt.Sprintf("Restore %s settings to their defaults.", strings.ToLower(section)),
			Primary:  ptrAction(api.Run(settingsReset, "Reset "+section, map[string]string{"section": section})),
		})
	}
	return items
}

func ptrAction(action api.Action) *api.Action {
	return &action
}

// valueText is the value of a setting as the row shows it.
func valueText(field binding) string {
	switch field.kind {
	case kindFlag:
		if *field.flag {
			return "On"
		}
		return "Off"
	case kindChoice:
		return *field.text
	case kindOptionalGap:
		if *field.gap == nil {
			return "Inherits the outer gap"
		}
	}
	return formatNumber(currentNumber(field)) + unitSuffix(field)
}

// currentText is the value a prompt starts from: the number, or nothing for a blank optional gap.
func currentText(field binding) string {
	if field.kind == kindOptionalGap && *field.gap == nil {
		return ""
	}
	return formatNumber(currentNumber(field))
}

func currentNumber(field binding) float64 {
	if field.kind == kindOptionalGap {
		return **field.gap
	}
	return *field.number
}

func unitSuffix(field binding) string {
	if field.unit == "" {
		return ""
	}
	return " " + field.unit
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// parseSettingText reads a typed value for field: a number inside its range, or nil for a blank optional gap. A message
// says why the text does not fit.
func parseSettingText(field binding, text string) (any, string) {
	trimmed := strings.TrimSpace(text)
	if field.kind == kindOptionalGap && trimmed == "" {
		return nil, ""
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || !(value >= field.min && value <= field.max) {
		return nil, fmt.Sprintf("Enter a number from %s to %s.", formatNumber(field.min), formatNumber(field.max))
	}
	return value, ""
}
