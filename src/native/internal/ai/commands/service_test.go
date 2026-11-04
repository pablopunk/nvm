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
