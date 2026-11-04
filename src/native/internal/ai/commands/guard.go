package commands

import (
	"strings"
	"unicode"
)

// MinimumWordOverlap and MaximumEditRatio decide whether a proofread reply is still the input, corrected. A reply
// passes when it keeps at least this share of the input's words, or when it differs by no more than this share of
// the characters. Either one is enough, as the desktop app's guard does.
const (
	MinimumWordOverlap = 0.7
	MaximumEditRatio   = 0.35
)

// PreservesInput reports whether output is a proofread of input rather than an answer to it. Letters and digits are
// compared, case and punctuation are not, and lengths count runes.
func PreservesInput(input, output string) bool {
	normalizedInput := normalizedText(input)
	normalizedOutput := normalizedText(output)
	if normalizedInput == "" || normalizedOutput == "" {
		return false
	}
	inputWords := strings.Fields(normalizedInput)
	outputWords := strings.Fields(normalizedOutput)
	return wordOverlap(inputWords, outputWords) >= MinimumWordOverlap ||
		editRatio(normalizedInput, normalizedOutput) <= MaximumEditRatio
}

// normalizedText lower-cases text and turns every run of characters that are neither letters nor digits into one space.
func normalizedText(text string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range strings.ToLower(text) {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			pendingSpace = true
			continue
		}
		if pendingSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		pendingSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// wordOverlap is the share of input words found in output, each output word matching once.
func wordOverlap(inputWords, outputWords []string) float64 {
	available := make(map[string]int, len(outputWords))
	for _, word := range outputWords {
		available[word]++
	}
	matching := 0
	for _, word := range inputWords {
		if available[word] > 0 {
			matching++
			available[word]--
		}
	}
	return float64(matching) / float64(max(len(inputWords), len(outputWords)))
}

// editRatio is the Levenshtein distance between the two texts over the length of the longer one.
func editRatio(left, right string) float64 {
	a, b := []rune(left), []rune(right)
	return float64(levenshtein(a, b)) / float64(max(len(a), len(b)))
}

// levenshtein returns the number of single-rune insertions, deletions and substitutions that turn a into b.
func levenshtein(a, b []rune) int {
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		diagonal := previous[0]
		previous[0] = i
		for j := 1; j <= len(b); j++ {
			above := previous[j]
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			previous[j] = min(previous[j]+1, previous[j-1]+1, diagonal+cost)
			diagonal = above
		}
	}
	return previous[len(b)]
}
