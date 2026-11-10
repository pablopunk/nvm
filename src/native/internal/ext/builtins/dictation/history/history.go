// Package history keeps the transcripts of finished dictations, newest first, up to MaxEntries. The rules are the
// Electron app's: a transcript keeps its text and time, and a list title is the first line of the text, cut to
// TitleChars. The store that holds the list is the app's; this package decides what the list becomes.
package history

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// MaxEntries is how many transcripts are kept.
const MaxEntries = 100

// TitleChars is how many characters of a transcript a list title shows.
const TitleChars = 100

// Entry is one finished dictation.
type Entry struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	CreatedAt int64  `json:"createdAt"`
}

// Add returns entries with a new transcript in front. Blank text is not kept, and the list never grows past MaxEntries.
func Add(entries []Entry, entry Entry) []Entry {
	if strings.TrimSpace(entry.Text) == "" {
		return entries
	}
	updated := append([]Entry{entry}, entries...)
	return updated[:min(len(updated), MaxEntries)]
}

// Remove returns entries without the one with id.
func Remove(entries []Entry, id string) []Entry {
	return slices.DeleteFunc(slices.Clone(entries), func(entry Entry) bool { return entry.ID == id })
}

// Clean drops the entries that cannot be shown: blank text, no ID or no time. It keeps the order it was given.
func Clean(entries []Entry) []Entry {
	var kept []Entry
	for _, entry := range entries {
		if entry.ID != "" && entry.CreatedAt > 0 && strings.TrimSpace(entry.Text) != "" {
			kept = append(kept, entry)
		}
	}
	slices.SortStableFunc(kept, func(a, b Entry) int {
		switch {
		case a.CreatedAt > b.CreatedAt:
			return -1
		case a.CreatedAt < b.CreatedAt:
			return 1
		}
		return 0
	})
	return kept[:min(len(kept), MaxEntries)]
}

// Title is the list title of a transcript: its whitespace collapsed into single spaces and cut to TitleChars.
func Title(text string) string {
	collapsed := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(collapsed) <= TitleChars {
		return collapsed
	}
	return string([]rune(collapsed)[:TitleChars])
}
