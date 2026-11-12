package notes

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// Search limits: a search with no limit returns DefaultSearchLimit matches, and no search returns more than
// MaxSearchLimit.
const (
	DefaultSearchLimit = 20
	MaxSearchLimit     = 50
	snippetRunes       = 140
)

// Match is one note a search found, with the first line as its title and the text around the first hit as a snippet.
// A match carries no full text, so a caller reads the note with Get when it needs it.
type Match struct {
	Note    Note
	Title   string
	Snippet string
}

// Search finds the notes whose text contains every word of query, ignoring case. Notes whose title holds every word come
// first; within each group the most recently saved note comes first. An empty query matches every note, so it lists the
// most recent ones. A limit outside 1..MaxSearchLimit selects DefaultSearchLimit or MaxSearchLimit.
func (s *Store) Search(query string, limit int) []Match {
	words := strings.Fields(strings.ToLower(query))
	var titled, other []Match
	for _, note := range s.List() {
		lowered := strings.ToLower(note.Markdown)
		if !containsEvery(lowered, words) {
			continue
		}
		match := Match{Note: note, Title: Title(note.Markdown), Snippet: snippetAround(note.Markdown, lowered, words)}
		if containsEvery(strings.ToLower(match.Title), words) {
			titled = append(titled, match)
		} else {
			other = append(other, match)
		}
	}
	found := slices.Concat(titled, other)
	if limit = boundedLimit(limit); len(found) > limit {
		found = found[:limit]
	}
	return found
}

func boundedLimit(limit int) int {
	switch {
	case limit <= 0:
		return DefaultSearchLimit
	case limit > MaxSearchLimit:
		return MaxSearchLimit
	}
	return limit
}

func containsEvery(lowered string, words []string) bool {
	for _, word := range words {
		if !strings.Contains(lowered, word) {
			return false
		}
	}
	return true
}

// Title is the first line of a note that has text, without heading marks. An empty note has the title "Empty note".
func Title(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		if text := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#")); text != "" {
			return clipRunes(text, snippetRunes)
		}
	}
	return "Empty note"
}

// snippetAround is the text around the first hit of the first word, on one line. Without a word it is the start of
// the note after its title.
func snippetAround(markdown, lowered string, words []string) string {
	start := 0
	if len(words) > 0 {
		if at := strings.Index(lowered, words[0]); at >= 0 {
			start = max(0, at-snippetRunes/3)
		}
	}
	for start > 0 && !utf8.RuneStart(markdown[start]) {
		start--
	}
	return clipRunes(strings.Join(strings.Fields(markdown[start:]), " "), snippetRunes)
}

func clipRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit]) + "…"
}
