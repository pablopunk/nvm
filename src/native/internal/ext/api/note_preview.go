package api

import (
	"regexp"
	"strings"
)

// notePreviewLimit is the most characters of a note's preview, as the 1.x list and conflict rows keep.
const notePreviewLimit = 100

var notePreviewHeading = regexp.MustCompile(`^#{1,6}\s*`)

// NotePreview is the line a note's row shows under its title: the second content line, heading marks removed and cut
// to 100 characters. A note with one content line reads "Markdown note", and an empty one "Empty note".
func NotePreview(markdown string) string {
	var lines []string
	for _, raw := range strings.Split(markdown, "\n") {
		if line := strings.TrimSpace(notePreviewHeading.ReplaceAllString(strings.TrimSuffix(raw, "\r"), "")); line != "" {
			lines = append(lines, line)
		}
	}
	switch {
	case len(lines) > 1:
		return truncateNotePreview(lines[1])
	case len(lines) == 1:
		return "Markdown note"
	}
	return "Empty note"
}

// truncateNotePreview keeps at most notePreviewLimit characters of line.
func truncateNotePreview(line string) string {
	if runes := []rune(line); len(runes) > notePreviewLimit {
		return string(runes[:notePreviewLimit])
	}
	return line
}
