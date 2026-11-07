package emoji

import (
	"strings"
	"unicode"
)

// maxStackWords sizes the word buffers that keep multi-word matching allocation-free for typical fields.
const maxStackWords = 16

// The grid's matching comes from 1.x's grid filter (src/app/palette/filtering.ts): a searched value matches when it
// contains the query, or when the query has several words and each one starts a word of the value, in order or in any
// order. Scattered letters never match.

// searchQuery is a normalized search string prepared once per search: its text, whether it has several words, and the
// rune mask used to skip records early.
type searchQuery struct {
	text      string
	multiWord bool
	mask      charMask
}

func newSearchQuery(text string) searchQuery {
	return searchQuery{text: text, multiWord: hasMultipleWords(text), mask: maskOf(text)}
}

// matchesQuery reports whether one lowercase field matches the query.
func matchesQuery(text string, q searchQuery) bool {
	return strings.Contains(text, q.text) || q.multiWord && wordsMatch(text, q)
}

// wordsMatch reports whether every word of the query starts a word of the text, in the order the query gives them or,
// failing that, in any order with each text word used once.
func wordsMatch(text string, q searchQuery) bool {
	var queryBuf, textBuf [maxStackWords]string
	queryWords := appendWords(queryBuf[:0], q.text)
	textWords := appendWords(textBuf[:0], text)
	return inOrderWordPrefixes(queryWords, textWords) || anyOrderWordPrefixes(queryWords, textWords)
}

func inOrderWordPrefixes(queryWords, textWords []string) bool {
	next := 0
	for _, queryWord := range queryWords {
		relative := indexWordPrefix(textWords[next:], queryWord)
		if relative < 0 {
			return false
		}
		next += relative + 1
	}
	return true
}

func anyOrderWordPrefixes(queryWords, textWords []string) bool {
	var usedBuf [maxStackWords]bool
	var used []bool
	if len(textWords) > maxStackWords {
		used = make([]bool, len(textWords))
	} else {
		used = usedBuf[:len(textWords)]
	}
	for _, queryWord := range queryWords {
		index := indexUnusedWordPrefix(textWords, used, queryWord)
		if index < 0 {
			return false
		}
		used[index] = true
	}
	return true
}

func indexWordPrefix(words []string, prefix string) int {
	for index, word := range words {
		if strings.HasPrefix(word, prefix) {
			return index
		}
	}
	return -1
}

func indexUnusedWordPrefix(words []string, used []bool, prefix string) int {
	for index, word := range words {
		if !used[index] && strings.HasPrefix(word, prefix) {
			return index
		}
	}
	return -1
}

// appendWords appends the word runs of value to dst. It matches strings.FieldsFunc with isNotWordRune but writes into
// dst, so callers can keep the buffer on the stack.
func appendWords(dst []string, value string) []string {
	start := -1
	for index, r := range value {
		if isWordRune(r) {
			if start < 0 {
				start = index
			}
		} else if start >= 0 {
			dst = append(dst, value[start:index])
			start = -1
		}
	}
	if start >= 0 {
		dst = append(dst, value[start:])
	}
	return dst
}

// hasMultipleWords reports whether value has two word runs separated by a non-word rune.
func hasMultipleWords(value string) bool {
	seenWord := false
	seenBreakAfterWord := false
	for _, r := range value {
		if isWordRune(r) {
			if seenBreakAfterWord {
				return true
			}
			seenWord = true
		} else if seenWord {
			seenBreakAfterWord = true
		}
	}
	return false
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}
