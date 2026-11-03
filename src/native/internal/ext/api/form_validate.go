package api

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ValidateFieldValue checks one submitted value against its field's type and rules. The host runs
// it before a submit action fires, and the author package uses it in tests. A nil error means the
// value is acceptable; a non-nil error text is safe to show next to the field.
func ValidateFieldValue(field Field, value FormValue) error {
	switch field.Type {
	case FieldDescription, FieldSeparator:
		return validateNoValue(value)
	case FieldText, FieldTextarea, FieldPassword:
		return validateText(field, value)
	case FieldEmail:
		return validateEmail(field, value)
	case FieldURL:
		return validateURL(field, value)
	case FieldNumber:
		return validateNumber(field, value)
	case FieldDate:
		return validateDate(field, value)
	case FieldCheckbox:
		return validateCheckbox(field, value)
	case FieldDropdown, FieldSelect:
		return validateChoice(field, value)
	case FieldMultiselect:
		return validateMultiselect(field, value)
	case FieldFile, FieldFolder:
		return validatePath(field, value)
	case FieldFiles:
		return validatePaths(field, value)
	}
	return fmt.Errorf("unknown field type %q", field.Type)
}

func validateNoValue(value FormValue) error {
	if value.Kind != "" || value.Text != "" || value.Flag || len(value.List) > 0 {
		return errors.New("this field takes no value")
	}
	return nil
}

func requireKind(value FormValue, want FormValueKind) error {
	if value.Kind != "" && value.Kind != want {
		return fmt.Errorf("expected a %s value, got %s", want, value.Kind)
	}
	return nil
}

func validateText(field Field, value FormValue) error {
	if err := requireKind(value, FormText); err != nil {
		return err
	}
	text := value.Text
	if field.Required && strings.TrimSpace(text) == "" {
		return errors.New("required")
	}
	if field.Rules == nil {
		return nil
	}
	length := utf8.RuneCountInString(text)
	if field.Rules.MinLength > 0 && length < field.Rules.MinLength {
		return fmt.Errorf("at least %d characters", field.Rules.MinLength)
	}
	if field.Rules.MaxLength > 0 && length > field.Rules.MaxLength {
		return fmt.Errorf("at most %d characters", field.Rules.MaxLength)
	}
	if field.Rules.Pattern != "" {
		matched, err := regexp.MatchString("^(?:"+field.Rules.Pattern+")$", text)
		if err != nil {
			return fmt.Errorf("invalid pattern in field rules: %v", err)
		}
		if !matched {
			return errors.New("does not match the required format")
		}
	}
	return nil
}

func validateEmail(field Field, value FormValue) error {
	if err := validateText(field, value); err != nil || value.Text == "" {
		return err
	}
	address, err := mail.ParseAddress(value.Text)
	if err != nil || address.Address != value.Text {
		return errors.New("enter an email address")
	}
	return nil
}

func validateURL(field Field, value FormValue) error {
	if err := validateText(field, value); err != nil || value.Text == "" {
		return err
	}
	parsed, err := url.Parse(value.Text)
	if err != nil || !oneOf(parsed.Scheme, "http", "https") || parsed.Host == "" {
		return errors.New("enter an http or https address")
	}
	return nil
}

func validateNumber(field Field, value FormValue) error {
	if err := requireKind(value, FormText); err != nil {
		return err
	}
	if value.Text == "" {
		if field.Required {
			return errors.New("required")
		}
		return nil
	}
	number, err := strconv.ParseFloat(value.Text, 64)
	if err != nil {
		return errors.New("enter a number")
	}
	if field.Rules == nil {
		return nil
	}
	if field.Rules.Min != nil && number < *field.Rules.Min {
		return fmt.Errorf("must be at least %g", *field.Rules.Min)
	}
	if field.Rules.Max != nil && number > *field.Rules.Max {
		return fmt.Errorf("must be at most %g", *field.Rules.Max)
	}
	return nil
}

func validateDate(field Field, value FormValue) error {
	if err := requireKind(value, FormText); err != nil {
		return err
	}
	if value.Text == "" {
		if field.Required {
			return errors.New("required")
		}
		return nil
	}
	if _, err := time.Parse(time.DateOnly, value.Text); err != nil {
		return errors.New("use the YYYY-MM-DD format")
	}
	return nil
}

func validateCheckbox(field Field, value FormValue) error {
	if err := requireKind(value, FormFlag); err != nil {
		return err
	}
	if field.Required && !value.Flag {
		return errors.New("must be checked")
	}
	return nil
}

func validateChoice(field Field, value FormValue) error {
	if err := requireKind(value, FormText); err != nil {
		return err
	}
	if value.Text == "" {
		if field.Required {
			return errors.New("choose an option")
		}
		return nil
	}
	if !hasOptionValue(field.Options, value.Text) {
		return fmt.Errorf("%q is not one of the options", value.Text)
	}
	return nil
}

func validateMultiselect(field Field, value FormValue) error {
	if err := requireKind(value, FormList); err != nil {
		return err
	}
	for _, choice := range value.List {
		if !hasOptionValue(field.Options, choice) {
			return fmt.Errorf("%q is not one of the options", choice)
		}
	}
	return validateItemCount(field, len(value.List))
}

func hasOptionValue(options []Option, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func validatePath(field Field, value FormValue) error {
	if err := requireKind(value, FormText); err != nil {
		return err
	}
	if value.Text == "" {
		if field.Required {
			return errors.New("choose a path")
		}
		return nil
	}
	if field.Type == FieldFile && !hasAllowedExtension(value.Text, field.Extensions) {
		return fmt.Errorf("choose a file with one of: %s", strings.Join(field.Extensions, ", "))
	}
	return nil
}

func validatePaths(field Field, value FormValue) error {
	if err := requireKind(value, FormList); err != nil {
		return err
	}
	for _, path := range value.List {
		if !hasAllowedExtension(path, field.Extensions) {
			return fmt.Errorf("%s is not an allowed file type", path)
		}
	}
	return validateItemCount(field, len(value.List))
}

func validateItemCount(field Field, count int) error {
	if field.Required && count == 0 {
		return errors.New("choose at least one")
	}
	if field.Rules == nil {
		return nil
	}
	if field.Rules.MinItems > 0 && count < field.Rules.MinItems {
		return fmt.Errorf("choose at least %d", field.Rules.MinItems)
	}
	if field.Rules.MaxItems > 0 && count > field.Rules.MaxItems {
		return fmt.Errorf("choose at most %d", field.Rules.MaxItems)
	}
	return nil
}

// hasAllowedExtension accepts any path when no extensions are listed. Suffixes match
// case-insensitively, with or without a leading dot.
func hasAllowedExtension(path string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	lower := strings.ToLower(path)
	for _, extension := range allowed {
		suffix := strings.ToLower(strings.TrimPrefix(extension, "."))
		if strings.HasSuffix(lower, "."+suffix) {
			return true
		}
	}
	return false
}
