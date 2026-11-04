package tools

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// decodeArgs reads the arguments of a call. The registry has already checked them against the schema, so a decode
// failure here is a programming error and is reported as one.
func decodeArgs(args json.RawMessage, into any) error {
	if err := json.Unmarshal(args, into); err != nil {
		return fmt.Errorf("the arguments could not be read: %w", err)
	}
	return nil
}

// clip keeps at most max runes of text, for lines a tool writes into its result.
func clip(text string, max int) string {
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	return string([]rune(text)[:max]) + "…"
}

// schemaRequiredString is the schema of an object with one required text argument of the given length bounds.
func schemaRequiredString(name string, minLength, maxLength int) string {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			name: map[string]any{"type": "string", "minLength": minLength, "maxLength": maxLength},
		},
		"required":             []string{name},
		"additionalProperties": false,
	}
	data, err := json.Marshal(schema)
	if err != nil {
		panic("tools: encoding a schema failed: " + err.Error())
	}
	return string(data)
}
