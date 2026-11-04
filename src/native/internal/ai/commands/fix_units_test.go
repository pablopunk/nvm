package commands

import (
	"errors"
	"strings"
	"testing"
)

// TestCleanReplyRemovesWhatAModelAdds checks each thing a reply can add around the text, and that input's own edges and
// quotes survive.
func TestCleanReplyRemovesWhatAModelAdds(t *testing.T) {
	cases := []struct{ name, input, reply, want string }{
		{"tags", "helo", "<text>hello</text>", "hello"},
		{"fence", "helo", "```\nhello\n```", "hello"},
		{"fence with language", "helo", "```text\nhello\n```", "hello"},
		{"preamble line", "helo", "Here is the corrected text:\nhello", "hello"},
		{"polite preamble", "helo", "Sure!\nhello", "hello"},
		{"acknowledgement that is text", "helo\nworld", "Sure, hello\nworld", "Sure, hello\nworld"},
		{"wrapping quotes", "helo", "“hello”", "hello"},
		{"input keeps its quotes", "\"helo\"", "\"hello\"", "\"hello\""},
		{"input keeps its fence", "```\nhelo\n```", "```\nhello\n```", "```\nhello\n```"},
		{"input keeps a preamble-like line", "Sure, helo", "Sure, hello", "Sure, hello"},
		{"input edges kept", "  helo\n", "hello", "  hello\n"},
		{"crlf kept", "a\r\nhelo\r\n", "a\nhello\n", "a\r\nhello\r\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CleanReply(c.input, c.reply); got != c.want {
				t.Fatalf("CleanReply(%q, %q) = %q, want %q", c.input, c.reply, got, c.want)
			}
		})
	}
}

// TestJudgeCorrectionAcceptsMinimalFixes and TestJudgeCorrectionRefusesRewrites hold the guard's threshold: spelling and
// grammar fixes pass, while answers, translations and rewrites do not.
func TestJudgeCorrectionAcceptsMinimalFixes(t *testing.T) {
	for _, pair := range [][2]string{
		{"I recieve the pakage tomorow.", "I receive the package tomorrow."},
		{"he go", "he goes"},
		{"I has two cats.", "I have two cats."},
		{"helo wrold", "hello world"},
		{"same text", "same text"},
	} {
		if got := JudgeCorrection(pair[0], pair[1]); got != Accept {
			t.Errorf("JudgeCorrection(%q, %q) = %v, want Accept", pair[0], pair[1], got)
		}
	}
}

func TestJudgeCorrectionRefusesRewrites(t *testing.T) {
	cases := []struct {
		name, input, output string
		want                Verdict
	}{
		{"empty answer", "hello there", "  ", RejectEmpty},
		{"translation to another script", "Good morning, my friend.", "Доброе утро, мой друг.", RejectScript},
		{"translation in the same script", "Hola, ¿cómo estás?", "Hello, how are you?", RejectRewritten},
		{"answer to a question", "Can you help me?", "Sure! Here is my answer: yes, I can help you with that today, of course.", RejectRewritten},
		{"rewritten sentence", "We shipped it on monday.", "The team delivered the product on the first Monday of the quarter.", RejectRewritten},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := JudgeCorrection(c.input, c.output); got != c.want {
				t.Fatalf("JudgeCorrection = %v, want %v", got, c.want)
			}
		})
	}
}

// TestEditDistanceIsOptimalStringAlignment pins the distance the guard counts with: a swapped pair of letters is one
// edit, not two.
func TestEditDistanceIsOptimalStringAlignment(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"teh", "the", 1},
		{"recieve", "receive", 1},
		{"kitten", "sitting", 3},
		{"ca", "abc", 3},
	}
	for _, c := range cases {
		if got := editDistance([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestSplitForCorrectionRejoinsExactly checks that pieces join back to the text and respect the limit, and that a cut
// prefers a paragraph break.
func TestSplitForCorrectionRejoinsExactly(t *testing.T) {
	paragraph := strings.Repeat("A sentence of ordinary length goes here. ", 20)
	text := paragraph + "\n\n" + paragraph + "\n\n" + paragraph
	for _, limit := range []int{50, 400, 1500} {
		pieces := SplitForCorrection(text, limit)
		if strings.Join(pieces, "") != text {
			t.Fatalf("limit %d: pieces do not rejoin to the text", limit)
		}
		for _, piece := range pieces {
			if n := len([]rune(piece)); n > limit {
				t.Fatalf("limit %d: piece of %d runes", limit, n)
			}
		}
	}
	pieces := SplitForCorrection(text, 1500)
	if !strings.HasSuffix(pieces[0], "\n\n") {
		t.Fatalf("the first cut did not land after a paragraph break: %q", pieces[0][len(pieces[0])-10:])
	}
}

// TestCorrectTextRetriesOnceThenRefuses: a refused first reply gets one retry; two refusals fail the correction.
func TestCorrectTextRetriesOnceThenRefuses(t *testing.T) {
	var replies []string
	ask := func(_, user string) (string, error) {
		replies = append(replies, user)
		if len(replies) == 1 {
			return "Sure, I can help with that and much more besides.", nil
		}
		return "I have dogs.", nil
	}
	got, err := CorrectText("i has dogs", ask, nil)
	if err != nil || got != "I have dogs." {
		t.Fatalf("CorrectText = %q, %v; want the retry's correction", got, err)
	}
	if len(replies) != 2 || !strings.Contains(replies[1], "Your previous answer") {
		t.Fatalf("the retry did not say the previous answer was off: %d calls", len(replies))
	}
	always := func(_, _ string) (string, error) { return "Sure! Here is an answer that is not a correction at all.", nil }
	if _, err := CorrectText("i has dogs", always, nil); !errors.Is(err, ErrCorrectionRefused) {
		t.Fatalf("two refusals returned %v, want ErrCorrectionRefused", err)
	}
}

// TestCorrectTextFailsTheWholeSelectionWhenOnePieceFails: a selection is either fully corrected or left alone, and
// progress is reported per piece.
func TestCorrectTextFailsTheWholeSelectionWhenOnePieceFails(t *testing.T) {
	long := strings.Repeat("This is fine text here. ", 80) + "\n\n" + strings.Repeat("More fine text here. ", 80)
	calls := 0
	ask := func(_, user string) (string, error) {
		calls++
		if calls <= CorrectionAttempts {
			return "a rewrite that is nothing like the input at all, not even close", nil
		}
		return pieceOf(user), nil
	}
	var progress []int
	_, err := CorrectText(long, ask, func(done, _ int) { progress = append(progress, done) })
	if !errors.Is(err, ErrCorrectionRefused) {
		t.Fatalf("CorrectText error = %v, want ErrCorrectionRefused", err)
	}
	if len(progress) != 0 {
		t.Fatalf("progress %v reported for a piece that failed", progress)
	}
}

// TestCorrectTextReportsProgressPerPiece checks progress for a selection that splits into several pieces.
func TestCorrectTextReportsProgressPerPiece(t *testing.T) {
	long := strings.Repeat("This is fine text here. ", 80) + "\n\n" + strings.Repeat("More fine text here. ", 80)
	var done, total []int
	got, err := CorrectText(long, func(_, user string) (string, error) { return pieceOf(user), nil }, func(d, n int) {
		done, total = append(done, d), append(total, n)
	})
	if err != nil || got != long {
		t.Fatalf("CorrectText = %v; want the text back unchanged", err)
	}
	pieces := len(SplitForCorrection(long, MaxChunkRunes))
	if pieces < 2 || len(done) != pieces || done[pieces-1] != pieces || total[0] != pieces {
		t.Fatalf("progress = %v of %v, want 1 to %d of %d", done, total, pieces, pieces)
	}
}
