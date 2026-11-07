package cleanup

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Screen term limits: the settings offer these sizes, and the default is the one a new install starts with.
const (
	DefaultScreenTermLimit = 30
	maxScreenTermLimit     = 100
	screenTermMinLength    = 3
)

// ScreenTermLimits are the sizes the settings let the user choose.
var ScreenTermLimits = []int{10, 30, 50}

var (
	screenWordPattern  = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'_-]*`)
	screenEdgePattern  = regexp.MustCompile(`^['_-]+|['_-]+$`)
	allDigitsPattern   = regexp.MustCompile(`^[0-9]+$`)
	screenStopWordList = "a an and are as at be by for from in is it of on or that the this to was were with"
	screenStopWords    = wordSet(screenStopWordList)
)

// NormalizeScreenTermLimit returns limit when the settings offer it, and the default otherwise.
func NormalizeScreenTermLimit(limit int) int {
	for _, allowed := range ScreenTermLimits {
		if limit == allowed {
			return limit
		}
	}
	return DefaultScreenTermLimit
}

// ExtractScreenTerms picks up to limit distinct words from text that the model may need to spell as they appear on
// screen: at least three characters, not all digits, not a stop word. The first occurrence of each word is kept.
func ExtractScreenTerms(text string, limit int) []string {
	limit = min(maxScreenTermLimit, max(1, limit))
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
		if len(terms) >= limit {
			break
		}
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
