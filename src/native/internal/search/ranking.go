package search

import (
	"regexp"
	"strings"
	"unicode"
)

// Port of src/app/palette/search-ranking.ts and search-utils.ts: the score bands are the ranking contract.
// Exported: ScoreFuzzy, Score and URLFromQuery; the rest is the band logic they share.

func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}

func queryWordPrefixScore(text, query string) int {
	qw := words(query)
	if len(qw) < 2 {
		return 0
	}
	tw := words(text)
	var idx []int
	next := 0
	ordered := true
	for _, q := range qw {
		found := -1
		for i := next; i < len(tw); i++ {
			if strings.HasPrefix(tw[i], q) {
				found = i
				break
			}
		}
		if found < 0 {
			ordered = false
			break
		}
		idx = append(idx, found)
		next = found + 1
	}
	if ordered {
		if idx[0] == 0 {
			return 95
		}
		return 70
	}
	avail := append([]string(nil), tw...)
	for _, q := range qw {
		m := -1
		for i, w := range avail {
			if strings.HasPrefix(w, q) {
				m = i
				break
			}
		}
		if m < 0 {
			return 0
		}
		avail = append(avail[:m], avail[m+1:]...)
	}
	return 60
}

func isWordChar(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func scoreSequential(text, query string) int {
	if text == query {
		return 100
	}
	phrase := 0
	rt, rq := []rune(text), []rune(query)
	for i := 0; i+len(rq) <= len(rt); i++ {
		if string(rt[i:i+len(rq)]) != query {
			continue
		}
		beforeOK := i == 0 || !isWordChar(rt[i-1])
		afterOK := i+len(rq) == len(rt) || !isWordChar(rt[i+len(rq)])
		if beforeOK && afterOK {
			phrase = 90
			break
		}
	}
	if strings.HasPrefix(text, query) {
		phrase = max(phrase, 80)
	}
	if strings.Contains(text, query) {
		phrase = max(phrase, 50)
	}
	phrase = max(phrase, queryWordPrefixScore(text, query))
	if phrase > 0 {
		return phrase
	}
	pos := 0
	for _, ch := range query {
		i := strings.IndexRune(text[pos:], ch)
		if i < 0 {
			return 0
		}
		pos += i + len(string(ch))
	}
	return 20
}

// ScoreFuzzy scores lowercase text against a lowercase, non-empty query: a sequential band when one
// matches, else 10 when every query character occurs in the text.
func ScoreFuzzy(text, query string) int {
	if s := scoreSequential(text, query); s > 0 {
		return s
	}
	counts := map[rune]int{}
	for _, ch := range text {
		counts[ch]++
	}
	for _, ch := range query {
		if counts[ch] <= 0 {
			return 0
		}
		counts[ch]--
	}
	return 10
}

// Score is ScoreFuzzy for text and query in any case, trimmed of surrounding space.
func Score(text, query string) int {
	return ScoreFuzzy(strings.ToLower(strings.TrimSpace(text)), strings.ToLower(strings.TrimSpace(query)))
}

var urlRe = regexp.MustCompile(`(?i)^[\w-]+(\.[\w-]+)+([/:?#].*)?$`)
var openRe = regexp.MustCompile(`(?i)^open\s+(.+)$`)

// URLFromQuery mirrors getUrlFromQuery in search-utils.ts: the URL a query names, or "" when it names none.
func URLFromQuery(q string) string {
	q = strings.TrimSpace(q)
	if m := openRe.FindStringSubmatch(q); m != nil {
		q = strings.TrimSpace(m[1])
	}
	if q == "" || strings.Contains(q, " ") {
		return ""
	}
	lq := strings.ToLower(q)
	if strings.HasPrefix(lq, "http://") || strings.HasPrefix(lq, "https://") {
		return q
	}
	if urlRe.MatchString(q) {
		return "https://" + q
	}
	return ""
}

// PrioritizedTitleScore is the 1.x action ranking of a title (prioritizedTitleSearchScore in search-utils.ts),
// on the 1.x scale of ten times this package's bands. It differs from Score in one place: a title that starts
// with the query scores 950, above a whole-word match inside a longer title (90). The root keeps Score's
// order, which is the parity contract of the root table; this is the ranking the 1.x action menu uses.
func PrioritizedTitleScore(title, query string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	t := strings.ToLower(strings.TrimSpace(title))
	if q == "" {
		return 0
	}
	score := ScoreFuzzy(t, q)
	switch {
	case score == 100:
		return 1000
	case strings.HasPrefix(t, q):
		return 950
	case score == 95:
		return 925
	case score == 90:
		return 900
	case score == 70:
		return 875
	case score == 60:
		return 860
	case score == 50:
		return 850
	case score == 20:
		return 820
	case score == 10:
		return 810
	}
	return 0
}
