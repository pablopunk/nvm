package emoji

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// UnicodeDetails describes one code point for the Copy Unicode and character info actions.
type UnicodeDetails struct {
	Glyph     string // the character as UTF-8
	Codepoint string // "U+2713"
	Decimal   int    // 10003
	UTF8      string // space-separated uppercase bytes, "E2 9C 93"
	Name      string // catalog name, empty when the character is not in the catalog
	Kind      Kind   // the catalog kind when known, otherwise KindUnicode
}

// UnicodeInfo describes r. It reports false for values that are not Unicode scalar values
// (surrogates and anything above U+10FFFF).
func (c *Catalog) UnicodeInfo(r rune) (UnicodeDetails, bool) {
	if !utf8.ValidRune(r) {
		return UnicodeDetails{}, false
	}
	glyph := string(r)
	details := UnicodeDetails{
		Glyph:     glyph,
		Codepoint: fmt.Sprintf("U+%04X", r),
		Decimal:   int(r),
		UTF8:      fmt.Sprintf("% X", []byte(glyph)),
		Kind:      KindUnicode,
	}
	if index, ok := builtinLookups().byRune[r]; ok {
		entry := entryAt(int(index))
		details.Name = entry.Name()
		details.Kind = entry.Kind()
	}
	return details, true
}

// CodepointsOf formats each code point of s as "U+XXXX", separated by spaces. It is the text the
// Copy Unicode action produces, and it keeps zero-width joiners and variation selectors visible.
func CodepointsOf(s string) string {
	out := make([]byte, 0, len(s)*4)
	for index, r := range s {
		if index > 0 {
			out = append(out, ' ')
		}
		out = appendCodepoint(out, r)
	}
	return string(out)
}

// appendCodepoint writes "U+" and at least four uppercase hex digits, matching fmt's %04X.
func appendCodepoint(dst []byte, r rune) []byte {
	var digits [8]byte
	hex := strconv.AppendUint(digits[:0], uint64(r), 16)
	dst = append(dst, 'U', '+')
	for padding := len(hex); padding < 4; padding++ {
		dst = append(dst, '0')
	}
	for _, c := range hex {
		if c >= 'a' && c <= 'f' {
			c -= 'a' - 'A'
		}
		dst = append(dst, c)
	}
	return dst
}

// normalizeQuery lowercases and trims the query, and rewrites the 0x form of a code point into the
// U+ form the catalog indexes, so "0x1F600" searches like "U+1F600".
func normalizeQuery(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	hex, ok := strings.CutPrefix(normalized, "0x")
	if ok && len(hex) >= 1 && len(hex) <= 6 && isHexDigits(hex) {
		return "u+" + hex
	}
	return normalized
}

func isHexDigits(value string) bool {
	for index := range len(value) {
		c := value[index]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
