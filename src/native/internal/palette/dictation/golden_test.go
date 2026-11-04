package dictation

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// goldenOptions sizes the indicator the way its window does; the goldens cover light and dark at 1x and 2x.
var goldenOptions = golden.Options{Group: "dictation", Width: 300, Height: 64}

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
	{"indicator-failed-long-detail", State{Mode: ModeFailed, Detail: "Not enough credits for API dictation. Add credits or try again later."}},
}

// TestIndicatorGoldens renders every indicator state in light and dark at 1x and 2x.
func TestIndicatorGoldens(t *testing.T) {
	for _, entry := range indicatorStates {
		t.Run(entry.name, func(t *testing.T) {
			state := entry.state
			golden.AssertMatrix(t, entry.name, func(c *ui.Context) { Indicator(c, state) }, goldenOptions)
		})
	}
}
