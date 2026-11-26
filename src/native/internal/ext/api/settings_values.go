package api

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// CheckValue refuses a value the setting does not accept: invalid JSON, a value of another type, a string that is not
// one of Choices, or a number outside Min and Max. An untyped setting accepts any other JSON.
func (d SettingDefinition) CheckValue(value json.RawMessage) error {
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return fmt.Errorf("the value of setting %q is not valid JSON", d.ID)
	}
	if !matchesSettingType(d.Type, decoded) {
		return fmt.Errorf("setting %q expects a %s", d.ID, d.Type)
	}
	if text, isText := decoded.(string); isText && len(d.Choices) > 0 && !d.hasChoice(text) {
		return fmt.Errorf("setting %q accepts only %s", d.ID, d.choiceList())
	}
	if number, isNumber := decoded.(float64); isNumber && !d.inRange(number) {
		return fmt.Errorf("setting %q accepts a number %s", d.ID, d.rangeText())
	}
	return nil
}

func matchesSettingType(settingType string, decoded any) bool {
	switch settingType {
	case SettingBoolean:
		_, ok := decoded.(bool)
		return ok
	case SettingString:
		_, ok := decoded.(string)
		return ok
	case SettingNumber:
		_, ok := decoded.(float64)
		return ok
	}
	return true
}

func (d SettingDefinition) hasChoice(value string) bool {
	return slices.ContainsFunc(d.Choices, func(choice SettingChoice) bool { return choice.Value == value })
}

func (d SettingDefinition) choiceList() string {
	values := make([]string, 0, len(d.Choices))
	for _, choice := range d.Choices {
		values = append(values, strconv.Quote(choice.Value))
	}
	return strings.Join(values, ", ")
}

func (d SettingDefinition) inRange(number float64) bool {
	return (d.Min == nil || number >= *d.Min) && (d.Max == nil || number <= *d.Max)
}

func (d SettingDefinition) rangeText() string {
	switch {
	case d.Min != nil && d.Max != nil:
		return "from " + formatSettingNumber(*d.Min) + " to " + formatSettingNumber(*d.Max)
	case d.Min != nil:
		return "of at least " + formatSettingNumber(*d.Min)
	}
	return "of at most " + formatSettingNumber(*d.Max)
}

func formatSettingNumber(number float64) string {
	return strconv.FormatFloat(number, 'f', -1, 64)
}

// validateDeclaredValues checks that a setting's Choices, Min, Max and default agree with each other and its type.
func validateDeclaredValues(d SettingDefinition) error {
	if len(d.Choices) > 0 {
		if d.Type != SettingString {
			return fmt.Errorf("setting %q declares choices, which need type string", d.ID)
		}
		if err := validateChoiceList(fmt.Sprintf("setting %q", d.ID), d.Choices); err != nil {
			return err
		}
	}
	if d.Min != nil || d.Max != nil {
		if d.Type != SettingNumber && d.Type != "" {
			return fmt.Errorf("setting %q declares min or max, which need type number or no type", d.ID)
		}
		if d.Min != nil && d.Max != nil && *d.Min > *d.Max {
			return fmt.Errorf("setting %q declares min %s above max %s", d.ID, formatSettingNumber(*d.Min), formatSettingNumber(*d.Max))
		}
	}
	if declaresDefault(d) {
		if err := d.CheckValue(d.Value); err != nil {
			return fmt.Errorf("default: %w", err)
		}
	}
	return nil
}

// declaresDefault is whether the setting declares a default other than null, which reads as no default.
func declaresDefault(d SettingDefinition) bool {
	return len(d.Value) > 0 && strings.TrimSpace(string(d.Value)) != "null"
}
