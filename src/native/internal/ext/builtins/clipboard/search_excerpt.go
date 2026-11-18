package clipboard

import (
	"strings"
	"unicode/utf8"
)

const excerptRunes = 96

// matchExcerpt explains a match beyond the displayed title, without replacing the copied value.
func matchExcerpt(text, title, query string) string {
	if query == "" || strings.Contains(strings.ToLower(title), strings.ToLower(query)) {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	lower := strings.ToLower(text)
	index := strings.Index(lower, strings.ToLower(query))
	if index < 0 {
		return ""
	}
	// Unicode lowercasing preserves rune count, but can change UTF-8 byte widths.
	runes := []rune(text)
	match := utf8.RuneCountInString(lower[:index])
	start := max(0, match-20)
	end := min(len(runes), start+excerptRunes)
	excerpt := string(runes[start:end])
	if start > 0 {
		excerpt = "…" + excerpt
	}
	if end < len(runes) {
		excerpt += "…"
	}
	return excerpt
}
