package search

import (
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// normalize appends the matching form of s to dst: lowercase, without accents, with every run of white space one space
// and none at either end. Matching compares only normalized text. An ASCII string, which is nearly every title, is
// normalized in place without allocating; other text is decomposed first.
func normalize(dst []byte, s string) []byte {
	if isASCII(s) {
		return normalizeASCII(dst, s)
	}
	return normalizeRunes(dst, norm.NFD.String(s))
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func normalizeASCII(dst []byte, s string) []byte {
	space := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || (c >= '\t' && c <= '\r') {
			space = len(dst) > 0
			continue
		}
		if space {
			dst, space = append(dst, ' '), false
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		dst = append(dst, c)
	}
	return dst
}

func normalizeRunes(dst []byte, s string) []byte {
	space := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case unicode.IsSpace(r):
			space = len(dst) > 0
			continue
		}
		if space {
			dst, space = append(dst, ' '), false
		}
		dst = utf8.AppendRune(dst, unicode.ToLower(r))
	}
	return dst
}
