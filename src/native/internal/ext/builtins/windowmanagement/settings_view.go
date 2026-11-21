package windowmanagement

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// settingsSections are the sections the settings view groups its rows under, in order.
var settingsSections = []string{sectionGaps, sectionSizes, sectionBehavior, sectionDisplays}

// settingsView groups the settings under their sections, each row showing its value at the edge. A flag toggles, a
// choice cycles, and a number opens a prompt. The reset row closes the page, as 1.x's page does.
func settingsView(ctx api.Context, start Settings) api.View {
	current := LoadSettings(ctx, start)
	fields := bindings(&current)
	sections := make([]api.ItemSection, 0, len(settingsSections)+1)
	for _, name := range settingsSections {
		section := api.ItemSection{Title: name}
		for _, field := range fields {
			if field.section == name {
				section.Items = append(section.Items, settingItem(ctx, field))
			}
		}
		sections = append(sections, section)
	}
	sections = append(sections, api.ItemSection{Items: resetItems()})
	view := api.ListView(settingsTitle)
	view.Placeholder, view.Sections = settingsPlaceholder, sections
	return view
}

// settingsTitle and settingsPlaceholder are the page's title and search placeholder, as 1.x's settings page shows them.
const (
	settingsTitle       = "Window Management Settings"
	settingsPlaceholder = "Search Settings"
)

func settingItem(ctx api.Context, field binding) api.Item {
	item := api.Item{ID: field.id, Title: field.title, Icon: settingIcon(field), Accessories: []api.Accessory{{Text: valueText(field)}}}
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

// settingIcon is the icon of a setting's row by its kind: a ruler for a number, a switch for a flag, a list for a choice.
func settingIcon(field binding) string {
	switch field.kind {
	case kindFlag:
		return "toggle-right"
	case kindChoice:
		return "list-ordered"
	}
	return "ruler"
}

// numberPrompt asks for the value of a number setting. A blank optional gap keeps the outer gap.
func numberPrompt(ctx api.Context, field binding) api.Action {
	hint := fmt.Sprintf("Enter a number from %s to %s%s.", formatNumber(field.min), formatNumber(field.max), unitSuffix(field))
	if field.kind == kindOptionalGap {
		hint = "Blank inherits the outer gap. " + hint
	}
	prompt := ctx.Input().Prompt(api.PromptSpec{
		Title: field.title, Message: hint, SubmitTitle: "Save",
		Fields: []api.Field{{
			ID: "value", Label: field.title, Type: api.FieldNumber, Value: api.TextValue(currentText(field)),
		}},
		Action: api.Run(settingSet, field.title, map[string]string{"id": field.id}),
	})
	prompt.Title = numberRowAction
	return prompt
}

// numberRowAction titles the row action of a number setting, which opens its prompt, as 1.x's settings rows do.
const numberRowAction = "Change…"

// resetItems are the Reset to Defaults row and one row per section.
func resetItems() []api.Item {
	items := []api.Item{{
		ID: "reset-all", Title: "Reset to Defaults", Subtitle: "Restore every window management setting to its default.", Icon: "rotate-ccw",
		Primary: ptrAction(api.Run(settingsReset, "Reset to Defaults", nil)),
	}}
	for _, section := range settingsSections {
		items = append(items, api.Item{
			ID: "reset-" + strings.ToLower(section), Title: "Reset " + section, Icon: "rotate-ccw",
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
