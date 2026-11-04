package extview

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// concealedValue stands for a set password in the rows; the text itself is never drawn.
const concealedValue = "••••••"

// takesValue reports whether a field type holds a value. Description and separator rows are context only.
func takesValue(fieldType api.FieldType) bool {
	return fieldType != api.FieldDescription && fieldType != api.FieldSeparator
}

func isChoice(fieldType api.FieldType) bool {
	return fieldType == api.FieldDropdown || fieldType == api.FieldSelect || fieldType == api.FieldMultiselect
}

func isPath(fieldType api.FieldType) bool {
	return fieldType == api.FieldFile || fieldType == api.FieldFiles || fieldType == api.FieldFolder
}

func isList(fieldType api.FieldType) bool {
	return fieldType == api.FieldMultiselect || fieldType == api.FieldFiles
}

// defaultFieldValue is the value a field starts with: the author's value when it set one, else the empty value of
// the field's kind.
func defaultFieldValue(field api.Field) api.FormValue {
	if field.Value.Kind != "" {
		return field.Value
	}
	switch {
	case field.Type == api.FieldCheckbox:
		return api.FlagValue(false)
	case isList(field.Type):
		return api.ListValue()
	}
	return api.TextValue("")
}

// fieldValueLabel is the short description of a value that a row shows. Secrets and long text are summarised.
func fieldValueLabel(field api.Field, value api.FormValue) string {
	switch {
	case field.Type == api.FieldCheckbox:
		if value.Flag {
			return "On"
		}
		return "Off"
	case field.Type == api.FieldPassword:
		if value.Text != "" {
			return concealedValue
		}
		return "Not set"
	case isList(field.Type):
		return listValueLabel(value.List)
	case field.Type == api.FieldTextarea:
		return lineCountLabel(value.Text)
	case isChoice(field.Type):
		return choiceTitle(field, value.Text)
	}
	if value.Text == "" {
		return "Not set"
	}
	return value.Text
}

func listValueLabel(items []string) string {
	switch len(items) {
	case 0:
		return "None"
	case 1:
		return "1 selected"
	}
	return strconv.Itoa(len(items)) + " selected"
}

func lineCountLabel(text string) string {
	lines := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	switch lines {
	case 0:
		return "Not set"
	case 1:
		return "1 line"
	}
	return strconv.Itoa(lines) + " lines"
}

// choiceTitle is the title of the option whose value is value, or the value itself when no option matches it.
func choiceTitle(field api.Field, value string) string {
	for _, option := range field.Options {
		if option.Value == value {
			return option.Title
		}
	}
	if value == "" {
		return "Not set"
	}
	return value
}

// fieldErrorCopy turns a validator's message into the sentence a row shows. Required fields say which field is missing.
func fieldErrorCopy(field api.Field, err error) string {
	if err.Error() == "required" {
		return fieldName(field) + " is required"
	}
	return capitalize(err.Error())
}

func fieldName(field api.Field) string {
	if field.Label != "" {
		return field.Label
	}
	return field.ID
}

func capitalize(text string) string {
	first, size := utf8.DecodeRuneInString(text)
	if size == 0 {
		return text
	}
	return string(unicode.ToUpper(first)) + text[size:]
}

// fieldPlaceholder is the search field's hint while a field is being edited.
func fieldPlaceholder(field api.Field) string {
	name := fieldName(field)
	switch {
	case isChoice(field.Type):
		return "Filter " + name
	case field.Placeholder != "":
		return field.Placeholder
	case field.Type == api.FieldDate:
		return "YYYY-MM-DD"
	}
	return "Enter " + name
}

// fieldIcon is the Lucide icon of a row, chosen by the kind of input it stands for.
func fieldIcon(fieldType api.FieldType) string {
	switch fieldType {
	case api.FieldCheckbox:
		return "check"
	case api.FieldPassword:
		return "lock"
	case api.FieldURL:
		return "link"
	case api.FieldNumber, api.FieldDate:
		return "hash"
	case api.FieldTextarea, api.FieldFile, api.FieldFiles:
		return "file-text"
	case api.FieldFolder:
		return "folder"
	case api.FieldDescription, api.FieldSeparator:
		return ""
	}
	return "type"
}

// prefillFor is the text the search field starts with when a field is edited: its current value, or nothing for
// choices and paths, whose search field filters options instead.
func prefillFor(field api.Field, value api.FormValue) string {
	if isChoice(field.Type) || isPath(field.Type) || field.Type == api.FieldPassword {
		return ""
	}
	return value.Text
}

// matchesFilter reports whether a field row stays visible for the search text q.
func matchesFilter(field api.Field, value api.FormValue, q string) bool {
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return true
	}
	haystack := strings.ToLower(fieldName(field) + " " + field.Description + " " + fieldValueLabel(field, value))
	return strings.Contains(haystack, needle)
}

// matchesOption reports whether a choice option stays listed for the search text q.
func matchesOption(option api.Option, q string) bool {
	return matchesText(option.Title+" "+option.Value, q)
}

// matchesText is a case-insensitive contains test; an empty query matches everything.
func matchesText(text, q string) bool {
	needle := strings.ToLower(strings.TrimSpace(q))
	return needle == "" || strings.Contains(strings.ToLower(text), needle)
}

func containsString(items []string, want string) bool {
	return slices.Contains(items, want)
}

func onOff(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}
