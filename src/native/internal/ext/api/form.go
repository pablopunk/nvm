package api

import (
	"errors"
	"fmt"
)

// FieldType is one of the sixteen form field kinds. Dropdown and select validate identically;
// dropdown is kept only for source compatibility with the TypeScript API.
type FieldType string

const (
	FieldText        FieldType = "text"
	FieldTextarea    FieldType = "textarea"
	FieldPassword    FieldType = "password"
	FieldEmail       FieldType = "email"
	FieldURL         FieldType = "url"
	FieldNumber      FieldType = "number"
	FieldDate        FieldType = "date"
	FieldCheckbox    FieldType = "checkbox"
	FieldDropdown    FieldType = "dropdown"
	FieldSelect      FieldType = "select"
	FieldMultiselect FieldType = "multiselect"
	FieldFile        FieldType = "file"
	FieldFiles       FieldType = "files"
	FieldFolder      FieldType = "folder"
	FieldDescription FieldType = "description"
	FieldSeparator   FieldType = "separator"
)

// AllFieldTypes lists the sixteen field kinds in declaration order.
func AllFieldTypes() []FieldType {
	return []FieldType{
		FieldText, FieldTextarea, FieldPassword, FieldEmail, FieldURL, FieldNumber, FieldDate,
		FieldCheckbox, FieldDropdown, FieldSelect, FieldMultiselect, FieldFile, FieldFiles,
		FieldFolder, FieldDescription, FieldSeparator,
	}
}

// Option is one choice of a dropdown, select or multiselect field.
type Option struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

// Rules are the declarative validation constraints of a field. A nil bound means "no bound".
type Rules struct {
	// MinLength and MaxLength count runes for text-like fields.
	MinLength int `json:"minLength,omitempty"`
	MaxLength int `json:"maxLength,omitempty"`
	// Min and Max bound number fields.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
	// Pattern is an RE2 expression the text must match in full.
	Pattern string `json:"pattern,omitempty"`
	// MinItems and MaxItems bound multiselect and files values.
	MinItems int `json:"minItems,omitempty"`
	MaxItems int `json:"maxItems,omitempty"`
}

// Field is one input in a form or in a palette prompt. Required on a checkbox means "must be true".
type Field struct {
	ID          string    `json:"id"`
	Label       string    `json:"label,omitempty"`
	Type        FieldType `json:"type"`
	Value       FormValue `json:"value,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Description string    `json:"description,omitempty"`
	Error       string    `json:"error,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Disabled    bool      `json:"disabled,omitempty"`
	Options     []Option  `json:"options,omitempty"`
	Rows        int       `json:"rows,omitempty"`
	Rules       *Rules    `json:"rules,omitempty"`
	// Extensions lists allowed file suffixes for file and files fields, such as "png" or ".md".
	Extensions []string `json:"extensions,omitempty"`
	FilterName string   `json:"filterName,omitempty"`
	// ButtonLabel and DefaultPath configure the native picker for file, files and folder.
	ButtonLabel string `json:"buttonLabel,omitempty"`
	DefaultPath string `json:"defaultPath,omitempty"`
	// DisableDirectoryCreation stops folder pickers from offering "New Folder". The default allows it.
	DisableDirectoryCreation bool `json:"disableDirectoryCreation,omitempty"`
}

// FormValueKind is the kind of a FormValue.
type FormValueKind string

const (
	FormText FormValueKind = "text"
	FormFlag FormValueKind = "flag"
	FormList FormValueKind = "list"
)

// FormValue is the value of one field. Go has no union types, so the Kind says which field is
// meaningful: text fields use Text, checkboxes use Flag, multiselect and files use List.
type FormValue struct {
	Kind FormValueKind `json:"kind,omitempty"`
	Text string        `json:"text,omitempty"`
	Flag bool          `json:"flag,omitempty"`
	List []string      `json:"list,omitempty"`
}

// TextValue, FlagValue and ListValue build FormValue literals for the three kinds.
func TextValue(text string) FormValue { return FormValue{Kind: FormText, Text: text} }

// FlagValue builds a checkbox value.
func FlagValue(flag bool) FormValue { return FormValue{Kind: FormFlag, Flag: flag} }

// ListValue builds a multiselect or files value.
func ListValue(items ...string) FormValue { return FormValue{Kind: FormList, List: items} }

// FormValues maps field ids to submitted values. The host injects it into the submit action.
type FormValues map[string]FormValue

// FormSpec is a structured multi-field view. The host shows fields as searchable rows; Submit
// runs with Invocation.Form holding every field's value.
type FormSpec struct {
	Fields []Field `json:"fields"`
	Submit *Action `json:"submit,omitempty"`
}

// Validate checks field ids and types. Value validation is ValidateFieldValue.
func (f FormSpec) Validate() error {
	seen := map[string]bool{}
	for _, field := range f.Fields {
		if field.ID == "" {
			return errors.New("every form field needs an id")
		}
		if seen[field.ID] {
			return fmt.Errorf("duplicate form field id %q", field.ID)
		}
		seen[field.ID] = true
		if !oneOf(field.Type, AllFieldTypes()...) {
			return fmt.Errorf("form field %q has unknown type %q", field.ID, field.Type)
		}
	}
	return nil
}
