package extview

import (
	"regexp"
	"strings"
)

const maxEditorTitleRunes = 120

var (
	titleMarkdownPrefix = regexp.MustCompile(`^(?:#{1,6}|>|[-+*]|\d+\.)\s*`)
	titleCheckboxPrefix = regexp.MustCompile(`^\[[ xX]\]\s*`)
	titleImage          = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	titleLink           = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	titleDecoration     = regexp.MustCompile("[*_~`]+")
	titleWhitespace     = regexp.MustCompile(`\s+`)
)

// titleFromFirstLine is the first line with text, without its Markdown marks and at most maxEditorTitleRunes long.
// With no such line it is fallback.
func titleFromFirstLine(content, fallback string) string {
	first := ""
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			first = strings.TrimRight(line, "\r")
			break
		}
	}
	title := titleMarkdownPrefix.ReplaceAllString(first, "")
	title = titleCheckboxPrefix.ReplaceAllString(title, "")
	title = titleImage.ReplaceAllString(title, "$1")
	title = titleLink.ReplaceAllString(title, "$1")
	title = titleDecoration.ReplaceAllString(title, "")
	title = titleWhitespace.ReplaceAllString(strings.TrimSpace(title), " ")
	if runes := []rune(title); len(runes) > maxEditorTitleRunes {
		title = string(runes[:maxEditorTitleRunes])
	}
	if title == "" {
		return fallback
	}
	return title
}
