package cleanup

import "strings"

// DictionaryTerms lists the custom dictionary's terms: one per line, trimmed, without blank lines.
func DictionaryTerms(dictionary string) []string {
	var terms []string
	for _, line := range strings.Split(dictionary, "\n") {
		if term := strings.TrimSpace(line); term != "" {
			terms = append(terms, term)
		}
	}
	return terms
}

// AddTerm adds term to the dictionary unless it is blank, holds a line break, or is already there (ignoring case). It
// reports whether the dictionary changed.
func AddTerm(dictionary, term string) (string, bool) {
	term = strings.TrimSpace(term)
	if term == "" || strings.ContainsRune(term, '\n') || hasTerm(dictionary, term) {
		return dictionary, false
	}
	return strings.Join(append(DictionaryTerms(dictionary), term), "\n"), true
}

// RemoveTerm drops every line that names term, ignoring case.
func RemoveTerm(dictionary, term string) string {
	key := strings.ToLower(strings.TrimSpace(term))
	var kept []string
	for _, line := range DictionaryTerms(dictionary) {
		if strings.ToLower(line) != key {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func hasTerm(dictionary, term string) bool {
	key := strings.ToLower(term)
	for _, line := range DictionaryTerms(dictionary) {
		if strings.ToLower(line) == key {
			return true
		}
	}
	return false
}
