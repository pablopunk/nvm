package commands

import (
	"strings"
	"testing"
)

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

// TestClassifyPresentsTheCategoryOfTheChosenImage sends the picture the user chose, typed by its extension, and presents
// the category, which Copy takes.
func TestClassifyPresentsTheCategoryOfTheChosenImage(t *testing.T) {
	h := newHarness(t)
	h.set.FS.Seed("/pics/cat.jpg", pngOfSize(64))

	h.service.ClassifyImageFile("/pics/cat.jpg")
	result := h.waitForPresented()
	if result.Title != "Image Classification" || result.Text() != "photograph" {
		t.Fatalf("result = %q / %q, want the category", result.Title, result.Text())
	}
	if body := h.backend.Requests()[0].Body; !strings.Contains(body, `"mediaType":"image/jpeg"`) {
		t.Fatalf("request body does not carry the jpeg media type: %.200s", body)
	}
	result.Copy(result.Text())
	if text, _ := h.set.Clipboard.ReadText(); text != "photograph" {
		t.Fatalf("clipboard after Copy = %q", text)
	}
}

// TestClassifyNeedsAReadableImage and TestClassifyRefusesLargeImages cover the refusals before any model call.
func TestClassifyNeedsAReadableImage(t *testing.T) {
	h := newHarness(t)
	h.service.ClassifyImageFile("/pics/missing.png")
	h.await("the missing-image status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == errNoImage.Error()
	})
	if got := len(h.backend.Requests()); got != 0 {
		t.Fatalf("backend requests = %d, want none without an image", got)
	}
}

func TestClassifyRefusesOtherFormats(t *testing.T) {
	h := newHarness(t)
	h.set.FS.Seed("/pics/anim.gif", pngOfSize(64))
	h.service.ClassifyImageFile("/pics/anim.gif")
	h.await("the unsupported-format status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == errNoImage.Error()
	})
	if got := len(h.backend.Requests()); got != 0 {
		t.Fatalf("backend requests = %d, want none for a gif", got)
	}
}

func TestClassifyRefusesLargeImages(t *testing.T) {
	h := newHarness(t)
	h.set.FS.Seed("/pics/big.png", pngOfSize(MaximumImageBytes+1))
	h.service.ClassifyImageFile("/pics/big.png")
	h.await("the too-large status", func() bool {
		return len(h.surface.statuses) > 0 && h.surface.last(t).Subtitle == errImageTooLarge.Error()
	})
	if got := len(h.backend.Requests()); got != 0 {
		t.Fatalf("backend requests = %d, want none for a large image", got)
	}
}
