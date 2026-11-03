package notes

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Title and preview lengths of the 1.x extension.
const (
	titleLimit    = 120
	previewLimit  = 100
	untitledTitle = "Untitled note"
)

var (
	markdownPrefix    = regexp.MustCompile(`^(?:#{1,6}|>|[-+*]|\d+\.)\s*`)
	checkboxPrefix    = regexp.MustCompile(`^\[[ xX]\]\s*`)
	imagePattern      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	linkPattern       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	decorationPattern = regexp.MustCompile("[*_~`]+")
	whitespacePattern = regexp.MustCompile(`\s+`)
	headingPrefix     = regexp.MustCompile(`^#{1,6}\s*`)
)

// Title derives a note's title from its first non-blank line: list, heading, quote and checkbox markers, images
// and links are reduced to their text, emphasis marks are dropped, and runs of spaces collapse. An empty result
// is "Untitled note".
func Title(markdown string) string {
	line := firstContentLine(markdown)
	title := markdownPrefix.ReplaceAllString(line, "")
	title = checkboxPrefix.ReplaceAllString(title, "")
	title = imagePattern.ReplaceAllString(title, "${1}")
	title = linkPattern.ReplaceAllString(title, "${1}")
	title = decorationPattern.ReplaceAllString(title, "")
	title = whitespacePattern.ReplaceAllString(strings.TrimSpace(title), " ")
	if title = truncateRunes(title, titleLimit); title == "" {
		return untitledTitle
	}
	return title
}

// Preview is the second content line of a note, heading marks removed, as the list shows under the title. A note
// with one line reads "Markdown note"; an empty one reads "Empty note".
func Preview(markdown string) string {
	var lines []string
	for _, raw := range splitLines(markdown) {
		if line := strings.TrimSpace(headingPrefix.ReplaceAllString(raw, "")); line != "" {
			lines = append(lines, line)
		}
	}
	switch {
	case len(lines) > 1:
		return truncateRunes(lines[1], previewLimit)
	case len(lines) == 1:
		return "Markdown note"
	}
	return "Empty note"
}

// RelativeTime says how long ago t was, in the words the list uses: "just now", "5m ago", "2h ago", "3d ago".
// A time in the future reads as "just now".
func RelativeTime(now, t time.Time) string {
	seconds := max(0, int(math.Floor(now.Sub(t).Seconds())))
	switch {
	case seconds < 60:
		return "just now"
	case seconds < 3600:
		return fmt.Sprintf("%dm ago", seconds/60)
	case seconds < 86_400:
		return fmt.Sprintf("%dh ago", seconds/3600)
	}
	return fmt.Sprintf("%dd ago", seconds/86_400)
}

// firstContentLine returns the first line with something other than spaces, or "".
func firstContentLine(markdown string) string {
	for _, line := range splitLines(markdown) {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// splitLines splits on LF, dropping a CR that precedes it, as the 1.x pattern /\r?\n/ does.
func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// truncateRunes keeps at most limit characters of s.
func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}
