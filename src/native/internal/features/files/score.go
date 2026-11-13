package files

import (
	"bytes"
	"unicode/utf8"
)

// The score bands of the palette ranking contract (src/backend/src/palette-ui/search-ranking.ts). This package scores file
// names with the same bands; WP1.1's shared search package replaces this copy.
//
//	100 exact; 95 ordered word prefixes starting at the first word; 90 whole-word phrase; 80 starts with;
//	70 ordered word prefixes; 60 unordered word prefixes; 50 substring; 20 subsequence; 10 character set; 0 no match.

// Bands of the ranking contract.
const (
	scoreExact         = 100
	scoreWordPrefixAt0 = 95
	scorePhrase        = 90
	scoreStartsWith    = 80
	scoreWordPrefix    = 70
	scoreWordUnordered = 60
	scoreContains      = 50
	scoreSubsequence   = 20
	scoreCharset       = 10
)

// scorer scores names against one query. It keeps the query and the scratch buffers scoring reuses, so scoring a
// name allocates nothing unless the name is not ASCII.
type scorer struct {
	query []byte
	// mask has one bit for every byte of the query, keyed by its low six bits. A name whose mask lacks one of those bits
	// lacks that byte, so it cannot match in any band and is rejected before any byte is compared.
	mask uint64
	// words are the query's words, nil unless the query has two or more.
	words [][]byte
	// lower is scratch for the lowercased name.
	lower []byte
	// textWords and avail are scratch for the word-prefix bands.
	textWords [][]byte
	avail     [][]byte
}

// newScorer returns a scorer for query, which must already be lowercased and trimmed.
func newScorer(query []byte) *scorer {
	s := &scorer{query: query, mask: charMask(query)}
	if words := splitWords(query); len(words) >= 2 {
		s.words = words
	}
	return s
}

// scoreFuzzy ranks text against query. Both must already be lowercased and trimmed, and query must not be empty.
// qWords are the query's words when it has two or more, nil otherwise. It is the scorer without its scratch reuse, for
// callers that score a few names.
func scoreFuzzy(text, query []byte, qWords [][]byte) int {
	s := &scorer{query: query, words: qWords}
	return s.scoreText(text)
}

// charMask returns the mask of the bytes of b; see scorer.mask.
func charMask(b []byte) uint64 {
	var mask uint64
	for _, c := range b {
		mask |= 1 << (c & 63)
	}
	return mask
}

// score returns the band of name, a file name in its original case. It returns 0 when the name does not match.
func (s *scorer) score(name []byte) int {
	text, mask := s.lowerName(name)
	if s.mask&^mask != 0 {
		return 0
	}
	return s.scoreText(text)
}

// lowerName returns the lowercase form of name and the mask of that form. ASCII is lowered into the scorer's scratch
// buffer; other names go through the Unicode case mapping.
func (s *scorer) lowerName(name []byte) ([]byte, uint64) {
	s.lower = s.lower[:0]
	var mask uint64
	for _, b := range name {
		if b >= utf8.RuneSelf {
			text := bytes.ToLower(name)
			return text, charMask(text)
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		s.lower = append(s.lower, b)
		mask |= 1 << (b & 63)
	}
	return s.lower, mask
}

// scoreText applies the bands to a lowercased name. The substring search runs once and the bands read its result, so
// a name without the query costs one search before the subsequence and character-set checks.
func (s *scorer) scoreText(text []byte) int {
	query := s.query
	if bytes.Equal(text, query) {
		return scoreExact
	}
	best := 0
	if idx := bytes.Index(text, query); idx >= 0 {
		best = scoreContains
		if idx == 0 {
			best = scoreStartsWith
		}
		if phraseFrom(text, query, idx) {
			best = max(best, scorePhrase)
		}
	}
	if s.words != nil && s.wordsPresent(text) {
		best = max(best, s.wordPrefixScore(text))
	}
	if best > 0 {
		return best
	}
	if subsequenceMatches(text, query) {
		return scoreSubsequence
	}
	if charsetMatches(text, query) {
		return scoreCharset
	}
	return 0
}
