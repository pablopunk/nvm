package app

import "strings"

// oneLineLimit is the longest a one-line label is kept, in characters.
const oneLineLimit = 90

// oneLine collapses s to one line of at most oneLineLimit characters.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > oneLineLimit {
		s = string(r[:oneLineLimit]) + "…"
	}
	return s
}
