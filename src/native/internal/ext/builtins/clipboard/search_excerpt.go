package clipboard

import "strings"

// matchExcerpt explains a match beyond the displayed title, without replacing the copied value.
func matchExcerpt(text, title, query string) string {
	if query == "" || strings.Contains(strings.ToLower(title), strings.ToLower(query)) {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	runes, needle := []rune(text), []rune(query)
	if len(needle) == 0 {
		return ""
	}
	for i := 0; i+len(needle) <= len(runes); i++ {
		if !strings.EqualFold(string(runes[i:i+len(needle)]), query) {
			continue
		}
		start, end := max(0, i-20), min(len(runes), i+len(needle)+40)
		excerpt := string(runes[start:end])
		if start > 0 {
			excerpt = "…" + excerpt
		}
		if end < len(runes) {
			excerpt += "…"
		}
		return excerpt
	}
	return ""
}
