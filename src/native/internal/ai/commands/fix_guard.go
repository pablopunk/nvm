package commands

import "strings"

// Verdict is the guard's decision about one corrected piece of text.
type Verdict int

// The verdicts a correction can get. Only Accept lets the correction replace the selection.
const (
	Accept Verdict = iota
	RejectEmpty
	RejectRewritten
	RejectScript
)

// MaxEditRatio is the largest share of the longer text a correction may change, counted in characters. A spelling or
// grammar fix changes a few characters of a word; when half of the text differs the model has written something else.
const MaxEditRatio = 0.5

// JudgeCorrection decides whether output is a minimal correction of input. It rejects an empty answer, a change of
// script (a translation or transliteration), a correction that keeps fewer than half of the input's words in order
// (at least one word may always change, so a two-word text can still take a fix), and one that changes more than
// MaxEditRatio of the characters. Unchanged text is always accepted.
func JudgeCorrection(input, output string) Verdict {
	if output == input {
		return Accept
	}
	if strings.TrimSpace(output) == "" {
		return RejectEmpty
	}
	if scriptChanged(input, output) {
		return RejectScript
	}
	if !keepsWords(input, output) || charEditRatio(input, output) > MaxEditRatio {
		return RejectRewritten
	}
	return Accept
}

// keepsWords reports whether output keeps enough of input's words, in order, and adds few new ones. Up to half of the
// words may be missing, and at least one, so a short text can take a grammar fix that changes one word; the same
// allowance bounds the words output adds, which catches an opening sentence the prompt forbids.
func keepsWords(input, output string) bool {
	inputWords, outputWords := words(input), words(output)
	if len(inputWords) == 0 {
		return true
	}
	aligned := alignedWords(inputWords, outputWords)
	allowance := max(1, len(inputWords)/2)
	return len(inputWords)-aligned <= allowance && len(outputWords)-aligned <= allowance
}

// charEditRatio is the edit distance between input and output over the length of the longer one, in characters.
func charEditRatio(input, output string) float64 {
	a, b := []rune(input), []rune(output)
	longest := max(len(a), len(b))
	if longest == 0 {
		return 0
	}
	return float64(editDistance(a, b)) / float64(longest)
}

// scriptChanged reports whether input and output were mostly Latin letters on one side and not on the other.
func scriptChanged(input, output string) bool {
	inShare, inLetters := latinShare(input)
	outShare, outLetters := latinShare(output)
	if inLetters == 0 || outLetters == 0 {
		return false
	}
	return (inShare >= 0.5) != (outShare >= 0.5)
}
