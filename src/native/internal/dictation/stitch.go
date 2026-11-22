package dictation

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// overlapMillis is how much of the previous piece a live piece repeats at its start. The model hears the sentence run
// across the cut, so it does not end a sentence at every pause, and stitch takes its words for the seam.
const overlapMillis = 3000

// overlapFrames is overlapMillis in Opus frames.
const overlapFrames = overlapMillis / FrameMillis

// Stitching bounds: the seam is looked for among the last maxSeamWords words of the text so far, and may start up to
// maxLeadingNoise words past the overlap's likely length into the next piece, as when the overlap cut a word in half.
const (
	maxSeamWords    = 12
	maxLeadingNoise = 3
)

// stitch joins the text so far with the next piece's text, which opens with the words of the overlap. It finds the
// longest run of words the two share and writes them as the next piece heard them, with what followed, so a pause that
// was not a sentence end loses its period and the case after it stays consistent. Only the first shared word keeps its
// case from the text so far, since the next piece may capitalize it for opening the piece. Without a shared run it drops as many leading words of the next piece as the
// overlap likely held, so no words repeat.
func stitch(sofar, next string, overlapWords int) string {
	left, right := strings.Fields(sofar), strings.Fields(next)
	if len(left) == 0 {
		return next
	}
	if len(right) == 0 {
		return sofar
	}
	cut, from, ok := seam(left, right, overlapWords)
	if !ok {
		return strings.Join(append(append([]string{}, left...), right[min(overlapWords, len(right)):]...), " ")
	}
	words := append([]string{}, left[:cut]...)
	shared := len(left) - cut
	words = append(words, mergeWord(left[cut], right[from]))
	words = append(words, right[from+1:from+shared]...)
	return strings.Join(append(words, right[from+shared:]...), " ")
}

// seam returns where the shared run starts in left and in right, preferring the longest run and then the earliest
// start in right. A run must reach the end of left, since the overlap is the end of the text so far, and a run of one
// word counts only for a word long enough not to match by chance.
func seam(left, right []string, overlapWords int) (cut, from int, ok bool) {
	best := 0
	window := min(maxSeamWords, len(left))
	for start := 0; start <= overlapWords+maxLeadingNoise && start < len(right); start++ {
		for length := min(window, len(right)-start); length > best; length-- {
			if !sameWords(left[len(left)-length:], right[start:start+length]) {
				continue
			}
			if length == 1 && len([]rune(normalWord(right[start]))) < 4 {
				break
			}
			best, cut, from, ok = length, len(left)-length, start, true
			break
		}
	}
	return cut, from, ok
}

// mergeWord writes a shared word with the leading marks and letters of mine and the trailing marks of theirs.
func mergeWord(mine, theirs string) string {
	return mine[:wordEnd(mine)] + theirs[wordEnd(theirs):]
}

// wordEnd is the byte offset just past the last letter or digit of word.
func wordEnd(word string) int {
	end := 0
	for i, r := range word {
		if isWordRune(r) {
			end = i + utf8.RuneLen(r)
		}
	}
	return end
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func sameWords(a, b []string) bool {
	for i := range a {
		if normalWord(a[i]) != normalWord(b[i]) || normalWord(a[i]) == "" {
			return false
		}
	}
	return true
}

// normalWord compares words without case or punctuation, so "not." matches "not".
func normalWord(word string) string {
	return strings.ToLower(strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }))
}

// overlapWordEstimate is how many words the overlap of a piece likely holds, from the pace of the text so far.
func overlapWordEstimate(sofar string, sofarMillis int) int {
	if sofarMillis <= 0 {
		return 0
	}
	return len(strings.Fields(sofar)) * overlapMillis / sofarMillis
}
