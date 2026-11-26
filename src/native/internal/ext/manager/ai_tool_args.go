package manager

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// aiToolInput is the part of a tool's JSON Schema the host checks: the properties, their types, which are required and
// whether others are allowed.
type aiToolInput struct {
	Properties map[string]struct {
		Type string `json:"type"`
	} `json:"properties"`
	Required             []string `json:"required"`
	AdditionalProperties *bool    `json:"additionalProperties"`
}

// checkAIToolArgs returns args as a JSON object that has every required property, no property the schema forbids and
// a value of the declared type for each one it names. Empty or null arguments are the empty object.
func checkAIToolArgs(schema, args json.RawMessage) (json.RawMessage, error) {
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(args, &values) != nil || values == nil {
		return args, aiToolArgsRefusal("the arguments must be a JSON object")
	}
	var input aiToolInput
	if len(schema) > 0 {
		_ = json.Unmarshal(schema, &input)
	}
	for _, name := range input.Required {
		if _, ok := values[name]; !ok {
			return args, aiToolArgsRefusal("the argument %q is required", name)
		}
	}
	for _, name := range sortedKeys(values) {
		property, known := input.Properties[name]
		if !known && input.AdditionalProperties != nil && !*input.AdditionalProperties {
			return args, aiToolArgsRefusal("the tool takes no argument %q; it takes %v", name, sortedKeys(input.Properties))
		}
		if known && !jsonValueHasType(values[name], property.Type) {
			return args, aiToolArgsRefusal("the argument %q must be of type %s", name, property.Type)
		}
	}
	return args, nil
}

// jsonValueHasType reports whether value is of the JSON Schema type; an empty or unknown type accepts anything.
func jsonValueHasType(value json.RawMessage, schemaType string) bool {
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return false
	}
	switch schemaType {
	case "string":
		_, ok := decoded.(string)
		return ok
	case "boolean":
		_, ok := decoded.(bool)
		return ok
	case "number":
		_, ok := decoded.(float64)
		return ok
	case "integer":
		number, ok := decoded.(float64)
		return ok && number == float64(int64(number))
	case "array":
		_, ok := decoded.([]any)
		return ok
	case "object":
		_, ok := decoded.(map[string]any)
		return ok
	}
	return true
}

func sortedKeys[V any](values map[string]V) []string {
	return slices.Sorted(maps.Keys(values))
}

func aiToolArgsRefusal(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, api.ErrInvalidInput)...)
}
