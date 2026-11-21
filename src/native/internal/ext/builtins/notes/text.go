package notes

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// titleLimit and untitledTitle are the title length and the title of a note with no text, as the 1.x extension has them.
const (
	titleLimit    = 120
	untitledTitle = "Untitled note"
)

var (
	markdownPrefix    = regexp.MustCompile(`^(?:#{1,6}|>|[-+*]|\d+\.)\s*`)
	checkboxPrefix    = regexp.MustCompile(`^\[[ xX]\]\s*`)
	imagePattern      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	linkPattern       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	decorationPattern = regexp.MustCompile("[*_~`]+")
	whitespacePattern = regexp.MustCompile(`\s+`)
)

// noteTitle derives a note's title from its first non-blank line: list, heading, quote and checkbox markers, images and
// links are reduced to their text, emphasis marks are dropped, and runs of spaces collapse. An empty result is
// "Untitled note".
func noteTitle(n api.Note) string {
	line := firstContentLine(n.Markdown)
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

// notePreview is the line a note's row shows under its title.
func notePreview(n api.Note) string {
	return api.NotePreview(n.Markdown)
}

// relativeTime says how long ago the note was saved, in the words the list uses: "just now", "5m ago", "2h ago",
// "3d ago". A time in the future reads as "just now".
func relativeTime(ctx api.Context, n api.Note) string {
	seconds := max(0, int(math.Floor(ctx.Clock().Now().Sub(n.UpdatedAt).Seconds())))
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
