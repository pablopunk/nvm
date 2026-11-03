package main

import (
	"regexp"
	"strings"
	"unicode"
)

// Port of src/app/palette/search-ranking.ts: the score bands are the ranking contract.

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

// scoreFuzzy scores lowercase text against a lowercase, non-empty query.
func scoreFuzzy(text, query string) int {
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

func score(text, query string) int {
	return scoreFuzzy(strings.ToLower(strings.TrimSpace(text)), strings.ToLower(strings.TrimSpace(query)))
}

var urlRe = regexp.MustCompile(`(?i)^[\w-]+(\.[\w-]+)+([/:?#].*)?$`)
var openRe = regexp.MustCompile(`(?i)^open\s+(.+)$`)

// urlFromQuery mirrors getUrlFromQuery in search-utils.ts.
func urlFromQuery(q string) string {
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
