package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"
)

// schemaNode is the subset of JSON Schema the tools use: type, properties, required, items, enum, length and range
// bounds, and additionalProperties. Anything else in a schema is ignored, so a schema must not rely on it.
type schemaNode struct {
	Type                 string                     `json:"type"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	Items                json.RawMessage            `json:"items"`
	Enum                 []any                      `json:"enum"`
	MinLength            *int                       `json:"minLength"`
	MaxLength            *int                       `json:"maxLength"`
	Minimum              *float64                   `json:"minimum"`
	Maximum              *float64                   `json:"maximum"`
	MaxItems             *int                       `json:"maxItems"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
}

// ErrArguments is the failure of arguments that do not match their tool's schema. The model reads its message.
var ErrArguments = errors.New("the arguments do not match the tool's schema")

// ValidateArguments checks that args, a JSON document, matches schema. It returns an error wrapping ErrArguments that
// names the first field that does not match.
func ValidateArguments(schema, args json.RawMessage) error {
	value, err := decodeNumbers(args)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrArguments, err)
	}
	var node schemaNode
	if len(schema) > 0 {
		if err := json.Unmarshal(schema, &node); err != nil {
			return fmt.Errorf("agent: the tool schema is invalid: %w", err)
		}
	}
	if err := node.check(value, "arguments"); err != nil {
		return fmt.Errorf("%w: %v", ErrArguments, err)
	}
	return nil
}

// decodeNumbers decodes a JSON document keeping numbers exact, so integer checks see what the model sent.
func decodeNumbers(data json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("more than one JSON value")
	}
	return value, nil
}

// check validates value against the node at path.
func (n schemaNode) check(value any, path string) error {
	if err := n.checkType(value, path); err != nil {
		return err
	}
	if len(n.Enum) > 0 && !slicesContainValue(n.Enum, value) {
		return fmt.Errorf("%s must be one of the listed values", path)
	}
	switch typed := value.(type) {
	case map[string]any:
		return n.checkObject(typed, path)
	case string:
		return n.checkString(typed, path)
	case []any:
		return n.checkArray(typed, path)
	case json.Number:
		return n.checkNumber(typed, path)
	}
	return nil
}

// checkType enforces the declared type, if any.
func (n schemaNode) checkType(value any, path string) error {
	switch n.Type {
	case "":
		return nil
	case "object":
		if _, ok := value.(map[string]any); ok {
			return nil
		}
	case "array":
		if _, ok := value.([]any); ok {
			return nil
		}
	case "string":
		if _, ok := value.(string); ok {
			return nil
		}
	case "boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "number":
		if _, ok := value.(json.Number); ok {
			return nil
		}
	case "integer":
		if number, ok := value.(json.Number); ok && isInteger(number) {
			return nil
		}
	default:
		return fmt.Errorf("%s uses an unsupported type %q", path, n.Type)
	}
	return fmt.Errorf("%s must be %s", path, article(n.Type))
}

// checkObject enforces required names, the property schemas and additionalProperties.
func (n schemaNode) checkObject(object map[string]any, path string) error {
	for _, name := range n.Required {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s.%s is required", path, name)
		}
	}
	for name, raw := range object {
		property, declared := n.Properties[name]
		if !declared {
			if n.AdditionalProperties != nil && !*n.AdditionalProperties {
				return fmt.Errorf("%s.%s is not a known argument", path, name)
			}
			continue
		}
		child, err := parseNode(property)
		if err != nil {
			return err
		}
		if err := child.check(raw, path+"."+name); err != nil {
			return err
		}
	}
	return nil
}

// checkString enforces the length bounds, counted in runes.
func (n schemaNode) checkString(text, path string) error {
	length := utf8.RuneCountInString(text)
	if n.MinLength != nil && length < *n.MinLength {
		return fmt.Errorf("%s must have at least %d characters", path, *n.MinLength)
	}
	if n.MaxLength != nil && length > *n.MaxLength {
		return fmt.Errorf("%s must have at most %d characters", path, *n.MaxLength)
	}
	return nil
}

// checkArray enforces the item bound and the item schema.
func (n schemaNode) checkArray(items []any, path string) error {
	if n.MaxItems != nil && len(items) > *n.MaxItems {
		return fmt.Errorf("%s must have at most %d items", path, *n.MaxItems)
	}
	if len(n.Items) == 0 {
		return nil
	}
	child, err := parseNode(n.Items)
	if err != nil {
		return err
	}
	for index, item := range items {
		if err := child.check(item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
	}
	return nil
}

// checkNumber enforces the range bounds.
func (n schemaNode) checkNumber(number json.Number, path string) error {
	value, err := number.Float64()
	if err != nil {
		return fmt.Errorf("%s must be a number", path)
	}
	if n.Minimum != nil && value < *n.Minimum {
		return fmt.Errorf("%s must be at least %v", path, *n.Minimum)
	}
	if n.Maximum != nil && value > *n.Maximum {
		return fmt.Errorf("%s must be at most %v", path, *n.Maximum)
	}
	return nil
}

// parseNode reads a sub-schema.
func parseNode(raw json.RawMessage) (schemaNode, error) {
	var node schemaNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return node, fmt.Errorf("agent: the tool schema is invalid: %w", err)
	}
	return node, nil
}

// isInteger reports whether a JSON number has no fraction.
func isInteger(number json.Number) bool {
	if _, err := number.Int64(); err == nil {
		return true
	}
	value, err := number.Float64()
	return err == nil && !math.IsInf(value, 0) && value == math.Trunc(value)
}

// slicesContainValue reports whether one of the enum values equals value.
func slicesContainValue(enum []any, value any) bool {
	for _, candidate := range enum {
		if reflect.DeepEqual(candidate, value) {
			return true
		}
		if number, ok := value.(json.Number); ok {
			if text, isText := candidate.(string); isText && text == number.String() {
				return true
			}
		}
	}
	return false
}

// article names a schema type for an error message.
func article(typeName string) string {
	if strings.HasPrefix(typeName, "a") || strings.HasPrefix(typeName, "i") {
		return "an " + typeName
	}
	return "a " + typeName
}
