package calc

import (
	"strings"
	"unicode/utf8"
)

// isJSSpace reports whether r is whitespace to JavaScript's trim and \s, which is wider than Go's.
func isJSSpace(r rune) bool {
	switch {
	case r == ' ', r >= '\t' && r <= '\r':
		return true
	case r == 0xa0, r == 0x1680, r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000, r == 0xfeff:
		return true
	}
	return false
}

// canonicalSpace maps every JavaScript whitespace rune to an ASCII space, so the patterns below can use the
// ASCII \s class and still match what the TypeScript original matched.
func canonicalSpace(text string) string {
	return strings.Map(func(r rune) rune {
		if isJSSpace(r) {
			return ' '
		}
		return r
	}, text)
}

// jsLength counts text the way JavaScript's string length does: UTF-16 code units.
func jsLength(text string) int {
	n := 0
	for _, r := range text {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// hasASCIILetter reports whether text contains an ASCII letter. Every date, time, conversion and unit
// pattern needs one, so expressions without letters skip those checks.
func hasASCIILetter(text string) bool {
	for i := 0; i < len(text); i++ {
		if isASCIILetter(text[i]) {
			return true
		}
	}
	return false
}

// isASCIILetter reports whether c is a-z or A-Z.
func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isDigit reports whether r is an ASCII digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isASCIILetterRune reports whether r is an ASCII letter.
func isASCIILetterRune(r rune) bool {
	return r < utf8.RuneSelf && isASCIILetter(byte(r))
}

// isLetterRune reports whether r is an ASCII letter or the pi sign, the characters identifiers are made of.
func isLetterRune(r rune) bool {
	return r == 'π' || isASCIILetterRune(r)
}

// collapseSpaces replaces each run of ASCII spaces with one space and does not trim, as the TypeScript
// timezone alias normalizer does.
func collapseSpaces(text string) string {
	var b strings.Builder
	previousSpace := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == ' ' {
			if previousSpace {
				continue
			}
			previousSpace = true
		} else {
			previousSpace = false
		}
		b.WriteByte(c)
	}
	return b.String()
}

// normalizeUnitAlias lower-cases an alias and drops dots, so "Ft." and "ft" name the same unit.
func normalizeUnitAlias(alias string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(alias)), ".", "")
}
