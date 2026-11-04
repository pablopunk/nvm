package dictation

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// goldenWidth is the panel width the window uses; each golden takes the height Height gives its state.
const goldenWidth = 300

// indicatorStates lists every state the indicator must render, in the order the goldens are written.
var indicatorStates = []struct {
	name  string
	state State
}{
	{"indicator-listening-quiet", State{Mode: ModeListening, Level: 0.05}},
	{"indicator-listening-loud", State{Mode: ModeListening, Level: 0.8}},
	{"indicator-waiting-microphone", State{Mode: ModeWaitingForMicrophone}},
	{"indicator-transcribing", State{Mode: ModeTranscribing}},
	{"indicator-no-speech", State{Mode: ModeNoSpeech}},
	{"indicator-failed-short", State{Mode: ModeFailed, Detail: "Sign in to use API dictation."}},
	{"indicator-failed-long-detail", State{Mode: ModeFailed, Detail: "Not enough credits for API dictation. Add credits or try again later."}},
	{"indicator-failed-full-message", State{Mode: ModeFailed, Detail: "This recording exceeds the 30-credit per-request limit. Split it into shorter clips and retry, or record the rest in a separate dictation so each part stays under the limit."}},
}

// TestIndicatorGoldens renders every indicator state in light and dark at 1x and 2x, and the failure states at the
// height the window gives them.
func TestIndicatorGoldens(t *testing.T) {
	for _, entry := range indicatorStates {
		t.Run(entry.name, func(t *testing.T) {
			state := entry.state
			options := golden.Options{Group: "dictation", Width: goldenWidth, Height: Height(state)}
			golden.AssertMatrix(t, entry.name, func(c *ui.Context) { Indicator(c, state) }, options)
		})
	}
}
