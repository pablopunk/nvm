package files

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// phraseFrom reports whether query occurs in text with a word boundary on both sides, looking from the first occurrence
// at index first onwards.
func phraseFrom(text, query []byte, first int) bool {
	start := first
	for {
		if boundaryBefore(text, start) && boundaryAfter(text, start+len(query)) {
			return true
		}
		next := bytes.Index(text[start+1:], query)
		if next < 0 {
			return false
		}
		start += 1 + next
	}
}

// boundaryBefore reports whether the character before index i is absent or not a letter or digit.
func boundaryBefore(text []byte, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRune(text[:i])
	return !isWordRune(r)
}

// boundaryAfter reports whether the character at index i is absent or not a letter or digit.
func boundaryAfter(text []byte, i int) bool {
	if i >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRune(text[i:])
	return !isWordRune(r)
}

// isWordRune reports whether r is a letter or a number, the characters that make up words.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

// wordsPresent reports whether every query word occurs in text. A word prefix can only match where its word occurs,
// so a missing word rules out the word bands without splitting the name.
func (s *scorer) wordsPresent(text []byte) bool {
	for _, word := range s.words {
		if !bytes.Contains(text, word) {
			return false
		}
	}
	return true
}

// splitWords returns the words of text as sub-slices of it, split at every character that is not a letter or a number.
func splitWords(text []byte) [][]byte {
	return splitWordsInto(text, nil)
}

// splitWordsInto is splitWords appending to dst, so a caller can reuse its storage.
func splitWordsInto(text []byte, dst [][]byte) [][]byte {
	words := dst[:0]
	start := -1
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		if isWordRune(r) {
			if start < 0 {
				start = i
			}
		} else if start >= 0 {
			words = append(words, text[start:i])
			start = -1
		}
		i += size
	}
	if start >= 0 {
		words = append(words, text[start:])
	}
	return words
}

// wordPrefixScore scores query words that each prefix a word of text. Words matched in order from the first word score
// 95, in order elsewhere 70, and in any order 60. It returns 0 when some query word has no match.
func (s *scorer) wordPrefixScore(text []byte) int {
	s.textWords = splitWordsInto(text, s.textWords)
	textWords := s.textWords
	var ordered [8]int
	matched := ordered[:0]
	next := 0
	for _, qWord := range s.words {
		rel := indexWordPrefix(textWords[next:], qWord)
		if rel < 0 {
			break
		}
		next += rel
		matched = append(matched, next)
		next++
	}
	if len(matched) == len(s.words) {
		if matched[0] == 0 {
			return scoreWordPrefixAt0
		}
		return scoreWordPrefix
	}
	s.avail = append(s.avail[:0], textWords...)
	for _, qWord := range s.words {
		i := indexWordPrefix(s.avail, qWord)
		if i < 0 {
			return 0
		}
		s.avail = append(s.avail[:i], s.avail[i+1:]...)
	}
	return scoreWordUnordered
}

// indexWordPrefix returns the index of the first word that starts with prefix, or -1.
func indexWordPrefix(words [][]byte, prefix []byte) int {
	for i, word := range words {
		if bytes.HasPrefix(word, prefix) {
			return i
		}
	}
	return -1
}
