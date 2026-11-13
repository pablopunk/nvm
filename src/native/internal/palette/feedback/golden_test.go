package feedback

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// feedbackStates lists the messages the HUD must render: a short result, a long one that wraps, and the title alone.
var feedbackStates = []struct {
	name, title, message string
	tone                 Tone
}{
	{"feedback-pasted", "Nevermind", "Pasted", ToneDefault},
	{"feedback-long-message", "Nevermind", "Paste is not available on this system yet. The item is on the clipboard; press Ctrl+V to paste it.", ToneDefault},
	{"feedback-title-only", "Paste Text", "", ToneDefault},
	{"feedback-success", "Paste", "Pasted", ToneSuccess},
	{"feedback-error", "Paste", "Paste failed. The item is on the clipboard.", ToneError},
}

// TestFeedbackGoldens renders every HUD state in light and dark at 1x and 2x, at the height the panel gives it.
func TestFeedbackGoldens(t *testing.T) {
	for _, entry := range feedbackStates {
		t.Run(entry.name, func(t *testing.T) {
			options := golden.Options{Group: "feedback", Width: Width, Height: Height(entry.message, entry.tone)}
			golden.AssertMatrix(t, entry.name, func(c *ui.Context) { Indicator(c, entry.title, entry.message, entry.tone) }, options)
		})
	}
}
