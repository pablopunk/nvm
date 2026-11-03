package emoji

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Score bands of the palette's shared fuzzy scorer (src/app/palette/search-ranking.ts). A field
// scores with the highest band it satisfies; an entry scores with its best field. Keep these in step
// with the TypeScript until WP1.1's search package replaces this file.
const (
	scoreExact               = 100
	scoreOrderedWordStart    = 95
	scoreWholePhrase         = 90
	scoreStartsWith          = 80
	scoreOrderedWordPrefix   = 70
	scoreUnorderedWordPrefix = 60
	scoreContains            = 50
	scoreSequential          = 20
	scoreCharacterSet        = 10
)

// maxStackWords sizes the word buffers that keep multi-word scoring allocation-free for typical fields.
const maxStackWords = 16

// searchQuery is a normalized search string prepared once per search: its text, whether it has several
// words, and the rune mask used to skip records early.
type searchQuery struct {
	text      string
	multiWord bool
	mask      charMask
}

func newSearchQuery(text string) searchQuery {
	return searchQuery{text: text, multiWord: hasMultipleWords(text), mask: maskOf(text)}
}

// orderedScore covers the bands that keep the query's characters in order: exact, whole phrase,
// starts with, contains, word prefixes, and subsequence. Text is one lowercase field.
func orderedScore(text string, q searchQuery) int {
	needle := q.text
	if text == needle {
		return scoreExact
	}
	phrase := 0
	if first := strings.Index(text, needle); first >= 0 {
		phrase = max(wholePhraseScore(text, needle, first), scoreContains)
		if first == 0 {
			phrase = max(phrase, scoreStartsWith)
		}
	}
	phrase = max(phrase, queryWordPrefixScore(text, q))
	if phrase > 0 {
		return phrase
	}
	if isSubsequence(text, needle) {
		return scoreSequential
	}
	return 0
}

// wholePhraseScore reports the whole-phrase band when some occurrence of needle, starting at or after
// first (its first occurrence), is bounded by non-word characters.
func wholePhraseScore(text, needle string, first int) int {
	for start := first; start >= 0; {
		if isBoundaryBefore(text, start) && isBoundaryAfter(text, start+len(needle)) {
			return scoreWholePhrase
		}
		next := strings.Index(text[start+1:], needle)
		if next < 0 {
			break
		}
		start += 1 + next
	}
	return 0
}

// queryWordPrefixScore matches multi-word queries whose words are prefixes of text words, in order
// (95 when the first match is the first word, else 70) or in any order (60). Single-word queries and
// single-word fields return before any splitting.
func queryWordPrefixScore(text string, q searchQuery) int {
	if !q.multiWord || !hasMultipleWords(text) {
		return 0
	}
	var queryBuf, textBuf [maxStackWords]string
	queryWords := appendWords(queryBuf[:0], q.text)
	textWords := appendWords(textBuf[:0], text)
	return wordPrefixBand(queryWords, textWords)
}

func wordPrefixBand(queryWords, textWords []string) int {
	matched := 0
	next := 0
	firstIndex := -1
	for _, queryWord := range queryWords {
		relative := indexWordPrefix(textWords[next:], queryWord)
		if relative < 0 {
			break
		}
		next += relative
		if firstIndex < 0 {
			firstIndex = next
		}
		matched++
		next++
	}
	if matched == len(queryWords) {
		if firstIndex == 0 {
			return scoreOrderedWordStart
		}
		return scoreOrderedWordPrefix
	}
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
			return 0
		}
		used[index] = true
	}
	return scoreUnorderedWordPrefix
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

// appendWords appends the word runs of value to dst. It matches strings.FieldsFunc with isNotWordRune
// but writes into dst, so callers can keep the buffer on the stack.
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

// isBoundaryBefore and isBoundaryAfter read the characters around a phrase. They follow the
// TypeScript, which reads UTF-16 code units: a non-BMP character is never a word character there.
func isBoundaryBefore(text string, start int) bool {
	if start == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:start])
	return !isBMPWordRune(r)
}

func isBoundaryAfter(text string, end int) bool {
	if end >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[end:])
	return !isBMPWordRune(r)
}

func isBMPWordRune(r rune) bool {
	return r <= 0xFFFF && isWordRune(r)
}

// isSubsequence reports whether the query's characters appear in text in order.
func isSubsequence(text, query string) bool {
	position := 0
	for _, r := range query {
		index := strings.IndexRune(text[position:], r)
		if index < 0 {
			return false
		}
		position += index + utf8.RuneLen(r)
	}
	return true
}

// containsEveryRune is the character-set fallback: every query character must occur in text, in any
// order, with repeats counted. It checks presence first, which rejects most fields cheaply.
func containsEveryRune(text, query string) bool {
	for _, r := range query {
		if !strings.ContainsRune(text, r) {
			return false
		}
	}
	for index, r := range query {
		if strings.ContainsRune(query[:index], r) {
			continue
		}
		if strings.Count(text, string(r)) < strings.Count(query, string(r)) {
			return false
		}
	}
	return true
}
