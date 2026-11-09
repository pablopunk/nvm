package aicommands

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// indicatorStatuses lists every status the indicator renders, in the order the goldens are written.
var indicatorStatuses = []struct {
	name   string
	status commands.Status
}{
	{"indicator-reading", commands.Status{Title: fixTitle, Subtitle: "Reading Selection", Tone: commands.ToneLoading}},
	{"indicator-fixing", commands.Status{Title: fixTitle, Subtitle: "Fixing Text", Tone: commands.ToneLoading}},
	{"indicator-dictation-recording", commands.Status{Title: "Dictate", Subtitle: "Listening", Tone: commands.ToneLoading, Activity: commands.ActivityRecording, Waveform: [commands.WaveformSamples]float64{0, 0.1, 0.2, 0.4, 0.7, 1, 0.6, 0.3, 0.1}}},
	{"indicator-dictation-transcribing", commands.Status{Title: "Dictate", Subtitle: "Transcribing", Tone: commands.ToneLoading, Activity: commands.ActivityTranscribing}},
	{"indicator-dictation-waiting", commands.Status{Title: "Dictate", Subtitle: "Waiting for microphone", Tone: commands.ToneLoading, Activity: commands.ActivityLoading}},
	{"indicator-dictation-no-speech", commands.Status{Title: "Dictate", Subtitle: "No speech detected", Tone: commands.ToneLoading}},
	{"indicator-dictation-permission", commands.Status{Title: "Dictate", Subtitle: "Dictation unavailable: Allow microphone access in System Settings, then try again.", Tone: commands.ToneError}},
	{"indicator-no-changes", commands.Status{Title: fixTitle, Subtitle: "No changes needed", Tone: commands.ToneSuccess}},
	{"indicator-triage", commands.Status{Title: triageTitle, Subtitle: "bug report · urgency 4/5 · needs reply", Tone: commands.ToneSuccess}},
	{"indicator-failed-short", commands.Status{Title: fixTitle, Subtitle: "Sign in to use AI commands.", Tone: commands.ToneError}},
	{"indicator-failed-long", commands.Status{Title: "Classify Image with AI", Subtitle: "That image is too large to classify. Choose a smaller one and try again from the palette", Tone: commands.ToneError}},
}

// TestIndicatorGoldens renders every indicator status in light and dark at 1x and 2x, at the height the panel gives it.
func TestIndicatorGoldens(t *testing.T) {
	for _, entry := range indicatorStatuses {
		t.Run(entry.name, func(t *testing.T) {
			status := entry.status
			options := golden.Options{Group: "aicommands", Width: Width(status), Height: Height(status)}
			golden.AssertMatrix(t, entry.name, func(c *ui.Context) { Indicator(c, status) }, options)
		})
	}
}

// fixTitle and triageTitle are the titles of the AI commands, which the AI commands built-in owns.
const (
	fixTitle    = "Fix Selected Text with AI"
	triageTitle = "Triage Selected Text with AI"
)

// TestRecordingReducedMotionGoldens keeps the accessible static trace in the visual contract.
func TestRecordingReducedMotionGoldens(t *testing.T) {
	status := commands.Status{Title: "Dictate", Subtitle: "Listening", Tone: commands.ToneLoading, Activity: commands.ActivityRecording}
	for i := range status.Waveform {
		status.Waveform[i] = 0.8
	}
	options := golden.Options{Group: "aicommands", Width: Width(status), Height: Height(status), Interact: reduceIndicatorMotion}
	golden.AssertMatrix(t, "indicator-dictation-reduced-motion", func(c *ui.Context) { Indicator(c, status) }, options)
}

func reduceIndicatorMotion(tester *ui.Tester) {
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
}
