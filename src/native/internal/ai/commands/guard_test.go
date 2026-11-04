package commands

import "testing"

// TestPreservesInputAcceptsCorrections covers replies that only fix the input: spelling and grammar, punctuation
// and case.
func TestPreservesInputAcceptsCorrections(t *testing.T) {
	cases := []struct{ input, output string }{
		{"this are selected text", "This is selected text."},
		{"hey whats up ma dude", "Hey, what's up, ma dude?"},
		{"teh quick brown fox", "The quick brown fox."},
	}
	for _, c := range cases {
		if !PreservesInput(c.input, c.output) {
			t.Errorf("PreservesInput(%q, %q) = false, want a correction", c.input, c.output)
		}
	}
}

// TestPreservesInputRejectsAnswers covers replies that answer or continue the text instead of proofreading it, as a
// model does when it treats the selection as a question.
func TestPreservesInputRejectsAnswers(t *testing.T) {
	cases := []struct{ input, output string }{
		{"hey whats up ma dude", "Could you provide the full sentence?"},
		{"hey whats up ma dude", "What would you like me to fix?"},
		{"this are selected text", "Sure, here is an explanation of the grammar rules involved."},
		{"hey whats up ma dude", ""},
		{"   ", "anything"},
	}
	for _, c := range cases {
		if PreservesInput(c.input, c.output) {
			t.Errorf("PreservesInput(%q, %q) = true, want a rejection", c.input, c.output)
		}
	}
}

// TestNormalizedTextFoldsCaseAndPunctuation pins what the guard compares: lower-case letters and digits, one space
// between runs.
func TestNormalizedTextFoldsCaseAndPunctuation(t *testing.T) {
	if got, want := normalizedText("  Hey,  what's UP?? 2 apples "), "hey what s up 2 apples"; got != want {
		t.Fatalf("normalizedText = %q, want %q", got, want)
	}
}

// TestEditRatioCountsRunes checks that a multibyte letter costs one edit, not its byte width.
func TestEditRatioCountsRunes(t *testing.T) {
	if got := levenshtein([]rune("café"), []rune("cafe")); got != 1 {
		t.Fatalf("levenshtein(café, cafe) = %d, want 1", got)
	}
}
