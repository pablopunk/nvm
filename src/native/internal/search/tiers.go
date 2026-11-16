package search

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// Tier is how closely a row's name answers a typed query. A lower tier always ranks above a higher one, whatever the
// usage of the rows: tiers are the first key of the order of a tiered search (see compareTiered).
type Tier uint8

// The tiers, best first. Matching is case- and accent-insensitive, and runs of spaces count as one.
const (
	// TierNone marks a row with no tier: the empty query's rows, which rank by their Score.
	TierNone Tier = iota
	// TierExact: the whole title, an alias or a keyword equals the query.
	TierExact
	// TierPrefix: the title starts with the query.
	TierPrefix
	// TierWordStart: a word of the title starts with the query, the query's words start successive words of the title,
	// or the query is the title's initials ("gc" for Google Chrome).
	TierWordStart
	// TierSubstring: the title contains the query.
	TierSubstring
	// TierContent: the row matches in something other than its name: its subtitle, a keyword, its path, or the content it
	// stands for. Letters scattered through a name never match.
	TierContent
	// TierFallback: a row that echoes the query, such as a web search. It follows every other row.
	TierFallback
)

// queryMatcher matches the rows of one typed query. It normalizes the query once and keeps scratch space for the
// rows, so matching a row allocates nothing for an ASCII title. It is not safe for concurrent use.
type queryMatcher struct {
	query   []byte   // the normalized query
	words   [][]byte // the words of the query, when it has two or more
	compact []byte   // the query without its spaces
	title   []byte   // scratch: the normalized title
	text    []byte   // scratch: a normalized alias or keyword
	initial []byte   // scratch: the initials of the title
}

// newQueryMatcher returns the matcher of query. An empty or blank query has no matcher.
func newQueryMatcher(query string) *queryMatcher {
	q := normalize(nil, query)
	if len(q) == 0 {
		return nil
	}
	m := &queryMatcher{query: q, compact: bytes.ReplaceAll(q, []byte(" "), nil)}
	if bytes.IndexByte(q, ' ') >= 0 {
		m.words = bytes.Fields(q)
	}
	return m
}

// tierOf is the tier a row's name has for the query: exact against the title or one of its other names, then the
// closest way the title contains the query. A row whose name does not contain the query is TierContent.
func (m *queryMatcher) tierOf(title string, names []string) Tier {
	m.title = normalize(m.title[:0], title)
	if bytes.Equal(m.title, m.query) || m.anyEquals(names) {
		return TierExact
	}
	switch {
	case bytes.HasPrefix(m.title, m.query):
		return TierPrefix
	case m.startsWord(m.title) || m.isInitials(m.title) || m.startsWordsInOrder(m.title):
		return TierWordStart
	case bytes.Contains(m.title, m.query):
		return TierSubstring
	}
	return TierContent
}

// anyEquals reports whether one of texts, normalized, is the query.
func (m *queryMatcher) anyEquals(texts []string) bool {
	for _, text := range texts {
		m.text = normalize(m.text[:0], text)
		if bytes.Equal(m.text, m.query) {
			return true
		}
	}
	return false
}

// startsWord reports whether the query occurs in t at the start of a word.
func (m *queryMatcher) startsWord(t []byte) bool {
	for from := 0; from < len(t); {
		i := bytes.Index(t[from:], m.query)
		if i < 0 {
			return false
		}
		if i += from; atWordStart(t, i) {
			return true
		}
		from = i + 1
	}
	return false
}

// isInitials reports whether the query, without spaces, is at least two letters and starts the initials of t's words.
func (m *queryMatcher) isInitials(t []byte) bool {
	if utf8.RuneCount(m.compact) < 2 {
		return false
	}
	m.initial = m.initial[:0]
	for i := 0; i < len(t); {
		r, size := utf8.DecodeRune(t[i:])
		if isWordRune(r) && atWordStart(t, i) {
			m.initial = utf8.AppendRune(m.initial, r)
		}
		i += size
	}
	return bytes.HasPrefix(m.initial, m.compact)
}

// startsWordsInOrder reports whether each word of a multi-word query starts a word of t, in the query's order.
func (m *queryMatcher) startsWordsInOrder(t []byte) bool {
	if len(m.words) < 2 {
		return false
	}
	pos := 0
	for _, word := range m.words {
		at := -1
		for i := pos; i < len(t); i++ {
			if atWordStart(t, i) && bytes.HasPrefix(t[i:], word) {
				at = i
				break
			}
		}
		if at < 0 {
			return false
		}
		pos = at + len(word)
	}
	return true
}

// atWordStart reports whether position i of t starts a word: it is the start of t, or follows a character that is no
// letter or digit.
func atWordStart(t []byte, i int) bool {
	if i == 0 {
		return true
	}
	if c := t[i-1]; c < utf8.RuneSelf {
		return !isWordByte(c)
	}
	r, _ := utf8.DecodeLastRune(t[:i])
	return !isWordRune(r)
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z'
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// NameTier is the tier the name of a row, with the other names it answers to, has for query: TierExact to TierSubstring
// when the name answers the query, TierContent when it does not, and TierNone for a blank query.
func NameTier(query, title string, names ...string) Tier {
	m := newQueryMatcher(query)
	if m == nil {
		return TierNone
	}
	return m.tierOf(title, names)
}

// MatchesName reports whether the name of a row answers query: it is equal to it, starts with it, has a word that starts
// with it, has it as initials, or contains it. Letters scattered through the name do not answer a query.
func MatchesName(query, title string) bool {
	tier := NameTier(query, title)
	return tier != TierNone && tier < TierContent
}
