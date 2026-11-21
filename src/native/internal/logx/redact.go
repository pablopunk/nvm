package logx

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"
)

// RedactedValue replaces every removed value, in keyed fields and in free text alike.
const RedactedValue = "[REDACTED]"

// sensitiveKeyWords are substrings of a normalized field name that mark its whole value as secret.
// Names are lowercased and stripped of "_", "-" and "." first, so api_key, API-Key and apiKey all match.
var sensitiveKeyWords = []string{"token", "secret", "password", "passwd", "authorization", "cookie", "apikey", "key"}

// Redactor removes secrets from log messages and fields. A field whose name contains a sensitive word
// loses its whole value. Free text is scrubbed of authorization and cookie headers, bearer tokens,
// well-known provider key prefixes, URL query and fragment values, key=value pairs and long random-looking
// runs. The rules are conservative on purpose: a false positive costs a readable word, a leak costs a
// token. The zero value is ready to use; a nil Redactor is not.
type Redactor struct{}

// NewRedactor returns the default Redactor.
func NewRedactor() *Redactor {
	return &Redactor{}
}

// IsSensitiveKey reports whether a field named key must lose its value.
func (r *Redactor) IsSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	for _, word := range sensitiveKeyWords {
		if strings.Contains(normalized, word) {
			return true
		}
	}
	return false
}

// Value returns a redacted, JSON-compatible copy of v stored under key.
func (r *Redactor) Value(key string, v any) any {
	if v != nil && r.IsSensitiveKey(key) {
		return RedactedValue
	}
	return r.scrub(v)
}

// Fields returns a redacted copy of fields, with every value passed through Value.
func (r *Redactor) Fields(fields F) map[string]any {
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		out[key] = r.Value(key, value)
	}
	return out
}

// scrub normalizes v to JSON-compatible data and redacts the text inside it. Values of unknown types
// go through a JSON round trip, so structs and maps are handled without reflection of our own.
func (r *Redactor) scrub(v any) any {
	switch typed := v.(type) {
	case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return typed
	case float32:
		return finiteNumber(float64(typed))
	case float64:
		return finiteNumber(typed)
	case string:
		return r.Text(typed)
	case error:
		return r.Text(typed.Error())
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = r.Value(key, item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = r.scrub(item)
		}
		return out
	}
	data, err := json.Marshal(v)
	if err != nil {
		return r.Text(fmt.Sprint(v))
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return r.Text(fmt.Sprint(v))
	}
	return r.scrub(decoded)
}

// finiteNumber keeps numbers JSON can represent; NaN and infinities become text.
func finiteNumber(n float64) any {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return fmt.Sprint(n)
	}
	return n
}

// normalizeKey lowercases a field name and drops the separators used to spell the same word differently.
func normalizeKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '_', '-', '.':
			return -1
		}
		return unicode.ToLower(r)
	}, key)
}
