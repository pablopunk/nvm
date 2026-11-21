package commands

import (
	"strings"
	"unicode"
)

// words returns the lower-cased letters and digits of text, one entry per run. Spacing and punctuation are not words,
// so a correction that only changes them keeps every word.
func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// wordsSimilar reports whether two words are one word with a slip of spelling: their edit distance is at most a third
// of the longer word, and at least one edit, so a short word can still take one typo.
func wordsSimilar(a, b string) bool {
	longest := max(len([]rune(a)), len([]rune(b)))
	return editDistance([]rune(a), []rune(b)) <= max(1, longest/3)
}

// alignedWords is the number of input words that line up, in order, with output words: the longest common subsequence
// where two words line up when wordsSimilar holds.
func alignedWords(input, output []string) int {
	previous := make([]int, len(output)+1)
	current := make([]int, len(output)+1)
	for _, in := range input {
		for j, out := range output {
			switch {
			case wordsSimilar(in, out):
				current[j+1] = previous[j] + 1
			default:
				current[j+1] = max(previous[j+1], current[j])
			}
		}
		previous, current = current, previous
		clear(current)
	}
	return previous[len(output)]
}

// editDistance is the optimal string alignment distance between a and b: insertions, deletions, substitutions, and
// swaps of two adjacent runes, each counted once. It keeps three rows, so a long text needs no full matrix.
func editDistance(a, b []rune) int {
	prev2 := make([]int, len(b)+1)
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(b)]
}

// latinShare is the share of the letters in text that are Latin, or 0 when text has no letters.
func latinShare(text string) (share float64, letters int) {
	latin := 0
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.Is(unicode.Latin, r) {
			latin++
		}
	}
	if letters == 0 {
		return 0, 0
	}
	return float64(latin) / float64(letters), letters
}
