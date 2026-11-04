package builder

import (
	"strings"
	"unicode/utf8"
)

// maxReasonRunes bounds the reason a model receives. A compile error can run long, and the model needs the first lines.
const maxReasonRunes = 2000

// oneLine joins a message onto one line and bounds it for a transcript notice.
func oneLine(s string) string {
	return truncateRunes(strings.Join(strings.Fields(s), " "), 240)
}

// truncateRunes cuts s to max runes, marking the cut.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}
