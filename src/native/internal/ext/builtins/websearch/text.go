package websearch

import "strings"

// maxTitleRunes is how many characters a search row's title keeps before it is cut.
const maxTitleRunes = 90

// oneLine collapses s to one line of at most maxTitleRunes characters, as the palette titles its rows.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxTitleRunes {
		s = string(r[:maxTitleRunes]) + "…"
	}
	return s
}
