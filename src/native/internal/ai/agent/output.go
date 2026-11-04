package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// capUTF8 keeps at most limit bytes of text, cut on a rune boundary, and says how much was dropped.
func capUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return fmt.Sprintf("%s\n[output truncated: kept %d of %d bytes]", text[:cut], cut, len(text))
}

// normalizedArguments treats an empty argument text as an empty object, which is what a tool without arguments gets.
func normalizedArguments(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return "{}"
	}
	return arguments
}

// isJSONObject reports whether text is one JSON object.
func isJSONObject(text string) bool {
	var object map[string]json.RawMessage
	return json.Unmarshal([]byte(text), &object) == nil && object != nil
}
