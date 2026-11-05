package cleanup

import "strings"

// Terms are the preferred spellings the model may use: the custom dictionary's terms, and the words read from the
// screen that the dictionary does not already name.
type Terms struct {
	Dictionary []string
	Screen     []string
}

// NewTerms splits the dictionary and the screen words into the two groups the prompt names. A word in both is a
// dictionary term, so the prompt presents the user's own spelling as the one to keep.
func NewTerms(dictionary string, screen []string) Terms {
	dictionaryKeys := map[string]bool{}
	for _, term := range DictionaryTerms(dictionary) {
		dictionaryKeys[strings.ToLower(term)] = true
	}
	var terms Terms
	for _, term := range mergeTerms(dictionary, screen) {
		if dictionaryKeys[strings.ToLower(term)] {
			terms.Dictionary = append(terms.Dictionary, term)
		} else {
			terms.Screen = append(terms.Screen, term)
		}
	}
	return terms
}

// mergeTerms lists the dictionary's terms, then the screen words, without blanks or repeats (ignoring case).
func mergeTerms(dictionary string, screen []string) []string {
	seen := map[string]bool{}
	var merged []string
	for _, term := range append(DictionaryTerms(dictionary), screen...) {
		term = strings.TrimSpace(term)
		key := strings.ToLower(term)
		if term == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, term)
	}
	return merged
}
