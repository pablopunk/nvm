package cleanup

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const screenTermMinLength = 3

var (
	screenWordPattern  = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'_-]*`)
	screenEdgePattern  = regexp.MustCompile(`^['_-]+|['_-]+$`)
	allDigitsPattern   = regexp.MustCompile(`^[0-9]+$`)
	screenStopWordList = "a an and are as at be by for from in is it of on or that the this to was were with"
	screenStopWords    = wordSet(screenStopWordList)
)

// ExtractScreenTerms picks every distinct word from text that the model may need to spell as they appear on
// screen: at least three characters, not all digits, not a stop word. The first occurrence of each word is kept.
func ExtractScreenTerms(text string) []string {
	seen := map[string]bool{}
	var terms []string
	for _, match := range screenWordPattern.FindAllString(text, -1) {
		term := screenEdgePattern.ReplaceAllString(match, "")
		key := strings.ToLower(term)
		if utf8.RuneCountInString(term) < screenTermMinLength || allDigitsPattern.MatchString(term) {
			continue
		}
		if screenStopWords[key] || seen[key] {
			continue
		}
		seen[key] = true
		terms = append(terms, term)
	}
	return terms
}

func wordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}
