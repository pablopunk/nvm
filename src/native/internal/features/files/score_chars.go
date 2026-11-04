package files

import (
	"bytes"
	"unicode/utf8"
)

// subsequenceMatches reports whether the characters of query appear in text in order, not necessarily adjacent.
func subsequenceMatches(text, query []byte) bool {
	pos := 0
	for len(query) > 0 {
		r, size := utf8.DecodeRune(query)
		idx := bytes.IndexRune(text[pos:], r)
		if idx < 0 {
			return false
		}
		pos += idx + utf8.RuneLen(r)
		query = query[size:]
	}
	return true
}

// charsetMatches reports whether every character of query occurs in text as many times as it occurs in query, in any
// order.
func charsetMatches(text, query []byte) bool {
	if !isASCII(text) || !isASCII(query) {
		return charsetMatchesUnicode(text, query)
	}
	for i, b := range query {
		if bytes.IndexByte(query[:i], b) >= 0 {
			continue
		}
		if countByte(text, b) < countByte(query, b) {
			return false
		}
	}
	return true
}

// charsetMatchesUnicode is charsetMatches for text with non-ASCII characters.
func charsetMatchesUnicode(text, query []byte) bool {
	counts := map[rune]int{}
	for _, r := range string(text) {
		counts[r]++
	}
	for _, r := range string(query) {
		if counts[r] <= 0 {
			return false
		}
		counts[r]--
	}
	return true
}

// countByte counts occurrences of b in s.
func countByte(s []byte, b byte) int {
	n := 0
	for _, c := range s {
		if c == b {
			n++
		}
	}
	return n
}

// isASCII reports whether every byte of s is 7-bit.
func isASCII(s []byte) bool {
	for _, b := range s {
		if b >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// lowerInto writes the lowercase form of src into dst, reusing dst's storage. ASCII is lowered in place; other text
// goes through the Unicode case mapping.
func lowerInto(dst, src []byte) []byte {
	if !isASCII(src) {
		return bytes.ToLower(src)
	}
	dst = dst[:0]
	for _, b := range src {
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		dst = append(dst, b)
	}
	return dst
}
