package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	inputText     = "this are selected text"
	correctedText = "This is selected text."
	answerText    = "Could you provide the full sentence?"
)

// TestFixPresentsTheCorrectionAndReplacePastesIt runs the whole proofread: the selection is read, the reply streams
// into a proofread chat, the result page offers it, and Replace puts it into the source window and restores the
// clipboard.
func TestFixPresentsTheCorrectionAndReplacePastesIt(t *testing.T) {
	h := newHarness(t)
	h.backend.SetReply(correctedText)
	h.set.Clipboard.WriteText("keep me")
	h.set.SelectedText.Select(inputText)

	h.service.Fix(fixSource)
	result := h.waitForPresented()

	if got := result.Text(); got != correctedText {
		t.Fatalf("result text = %q, want %q", got, correctedText)
	}
	if result.Replace == nil || result.Copy == nil {
		t.Fatal("a proofread result offers both Replace and Copy")
	}
	if h.surface.hidden == 0 {
		t.Fatal("the indicator was not hidden when the result opened")
	}
	if got := h.backend.Requests(); len(got) == 0 || !strings.Contains(got[len(got)-1].Body, inputText) {
		t.Fatal("the proofread request did not carry the selected text")
	}

	result.Replace(result.Text())
	h.await("the paste is sent", func() bool { return len(h.set.Paster.Requests()) == 1 })
	if text, _ := h.set.Clipboard.ReadText(); text != correctedText {
		t.Fatalf("clipboard before the restore = %q, want the replacement", text)
	}
	h.fireAfter()
	if text, _ := h.set.Clipboard.ReadText(); text != "keep me" {
		t.Fatalf("clipboard after the restore = %q, want the previous text", text)
	}
	if h.surface.dismissed != 1 {
		t.Fatalf("palette dismissed %d times, want once before the paste", h.surface.dismissed)
	}
	if focus := h.set.Focus.Restored(); focus[len(focus)-1] != fixSource {
		t.Fatalf("focus restored to %v, want the source window before the paste", focus)
	}
}

// TestFixWithoutAChangeSaysSo: a reply equal to the input is a success with nothing to replace.
func TestFixWithoutAChangeSaysSo(t *testing.T) {
	h := newHarness(t)
	h.backend.SetReply(inputText)
	h.set.SelectedText.Select(inputText)

	h.service.Fix(fixSource)
	h.await("the no-change status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == "No changes needed"
	})
	if status := h.surface.last(t); status.Tone != ToneSuccess {
		t.Fatalf("tone = %q, want success", status.Tone)
	}
	if len(h.surface.presented) != 0 {
		t.Fatal("a reply with no change was presented")
	}
}

// TestFixRetriesAnAnswerOnceThenPresentsTheCorrection: the first reply answers the question instead of proofreading
// it; the retry is a proper correction.
func TestFixRetriesAnAnswerOnceThenPresentsTheCorrection(t *testing.T) {
	h := newHarness(t)
	h.useReplies(answerText, correctedText)
	h.set.SelectedText.Select(inputText)

	h.service.Fix(fixSource)
	result := h.waitForPresented()
	if result.Text() != correctedText {
		t.Fatalf("result = %q, want the retried correction", result.Text())
	}
	if got := chatRequests(h.backend); got != 2 {
		t.Fatalf("chat requests = %d, want the proofread and one retry", got)
	}
}

// TestFixGivesUpWhenTheRetryAnswersToo reports the invalid correction and presents nothing.
func TestFixGivesUpWhenTheRetryAnswersToo(t *testing.T) {
	h := newHarness(t)
	h.useReplies(answerText, answerText)
	h.set.SelectedText.Select(inputText)

	h.service.Fix(fixSource)
	h.await("the invalid-correction status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == errInvalidCorrection.Error()
	})
	if h.surface.last(t).Tone != ToneError {
		t.Fatal("the failure is not shown as an error")
	}
	if len(h.surface.presented) != 0 {
		t.Fatal("an invalid correction was presented")
	}
}

// TestFixWithoutSelectionAsksForOne: nothing selected means no model call and the selection copy.
func TestFixWithoutSelectionAsksForOne(t *testing.T) {
	h := newHarness(t)
	h.set.Clipboard.WriteText("before")

	h.service.Fix(fixSource)
	h.awaitTimer(25 * time.Millisecond)
	for i := 2; i < pollAttempts; i++ {
		h.awaitTimer(25 * time.Millisecond)
	}
	h.await("the no-selection status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == errNoSelection.Error()
	})
	if got := len(h.backend.Requests()); got != 0 {
		t.Fatalf("backend requests = %d, want none without a selection", got)
	}
}

// TestFixWhileSignedOutSaysToSignIn: no account means no read and no request.
func TestFixWhileSignedOutSaysToSignIn(t *testing.T) {
	h := newHarness(t)
	h.connected = false
	h.set.SelectedText.Select(inputText)

	h.service.Fix(fixSource)
	if got := h.surface.last(t).Subtitle; got != signInCopy {
		t.Fatalf("subtitle = %q, want %q", got, signInCopy)
	}
	if reqs := h.set.SelectedText.Requests(); len(reqs) != 0 {
		t.Fatalf("copy gestures = %v, want none while signed out", reqs)
	}
}

// TestFixShowsTheBackendsFailure: a failed proofread explains itself in the indicator, not on the model's words.
func TestFixShowsTheBackendsFailure(t *testing.T) {
	h := newHarness(t)
	h.backend.SetMode("server_error")
	h.set.SelectedText.Select(inputText)

	h.service.Fix(fixSource)
	h.await("the failure status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Tone == ToneError && h.surface.last(t).Subtitle != "Fixing Text"
	})
	if h.surface.last(t).Subtitle == "" {
		t.Fatal("the failure has no copy")
	}
}

// TestTriageShowsTheAnswers asks the decision route about the selection and shows the summary.
func TestTriageShowsTheAnswers(t *testing.T) {
	h := newHarness(t)
	h.backend.SetDecisionBody(`{"id":"d1","model":"fixture-fast","answers":{` +
		`"needs_reply":{"type":"predicate","value":true,"probability":0.9},` +
		`"category":{"type":"choice","value":"bug report"},` +
		`"urgency":{"type":"score","value":4}},"usage":{"inputTokens":1,"outputTokens":1}}`)
	h.set.SelectedText.Select("The export crashes on save, please fix")

	h.service.Triage(fixSource)
	h.await("the triage summary", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Tone == ToneSuccess
	})
	if got, want := h.surface.last(t).Subtitle, "bug report · urgency 4/5 · needs reply"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if len(h.surface.presented) != 0 {
		t.Fatal("triage presents a page; it only shows the indicator")
	}
}

// TestClassifyPresentsTheCategoryOfTheClipboardImage sends the clipboard image and presents the category, which Copy
// takes.
func TestClassifyPresentsTheCategoryOfTheClipboardImage(t *testing.T) {
	h := newHarness(t)
	h.set.Clipboard.WriteImagePNG(pngOfSize(64))

	h.service.ClassifyClipboardImage()
	result := h.waitForPresented()
	if result.Title != "Image Classification" || result.Text() != "photograph" {
		t.Fatalf("result = %q / %q, want the category", result.Title, result.Text())
	}
	if result.Replace != nil {
		t.Fatal("a classification cannot replace the selection")
	}
	result.Copy(result.Text())
	if text, _ := h.set.Clipboard.ReadText(); text != "photograph" {
		t.Fatalf("clipboard after Copy = %q", text)
	}
}

// TestClassifyNeedsAnImage and TestClassifyRefusesLargeImages cover the two refusals before any model call.
func TestClassifyNeedsAnImage(t *testing.T) {
	h := newHarness(t)
	h.service.ClassifyClipboardImage()
	h.await("the missing-image status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == errNoImage.Error()
	})
	if got := len(h.backend.Requests()); got != 0 {
		t.Fatalf("backend requests = %d, want none without an image", got)
	}
}

func TestClassifyRefusesLargeImages(t *testing.T) {
	h := newHarness(t)
	h.set.Clipboard.WriteImagePNG(pngOfSize(MaximumImageBytes + 1))
	h.service.ClassifyClipboardImage()
	h.await("the too-large status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == errImageTooLarge.Error()
	})
	if got := len(h.backend.Requests()); got != 0 {
		t.Fatalf("backend requests = %d, want none for a large image", got)
	}
}

// TestReplaceFailureKeepsTheTextOnTheClipboard: when the paste is refused, the replacement stays on the clipboard for
// a manual paste and the user is told so.
func TestReplaceFailureKeepsTheTextOnTheClipboard(t *testing.T) {
	h := newHarness(t)
	h.set.Paster.FailNext("PasteIntoFrontmost", platform.ErrUnsupported)

	h.service.Replace("fixed words", fixSource)
	h.await("the failure toast", func() bool { return len(h.surface.toasts) > 0 })
	if !strings.Contains(h.surface.toasts[0], "on the clipboard") {
		t.Fatalf("toast = %q, want the clipboard hint", h.surface.toasts[0])
	}
	if text, _ := h.set.Clipboard.ReadText(); text != "fixed words" {
		t.Fatalf("clipboard = %q, want the replacement left for a manual paste", text)
	}
}

// TestSettledIgnoresOtherChats: a chat that is not a proofread of this service is none of its business.
func TestSettledIgnoresOtherChats(t *testing.T) {
	h := newHarness(t)
	h.service.Settled("some-conversation", chat.Outcome{Ending: chat.EndFinished})
	if len(h.surface.statuses) != 0 || len(h.surface.presented) != 0 {
		t.Fatal("a chat the service did not start changed the surface")
	}
}

// chatRequests counts the chat completions the fixture backend received; each turn also asks for its route first.
func chatRequests(b *fixtures.Backend) int {
	count := 0
	for _, request := range b.Requests() {
		if request.Path == fixtures.ChatCompletionsPath {
			count++
		}
	}
	return count
}
