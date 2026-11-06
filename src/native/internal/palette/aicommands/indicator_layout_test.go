package aicommands

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ai/commands"
)

func TestIndicatorLiveStatesKeepTheirCompactGeometry(t *testing.T) {
	for _, activity := range []commands.Activity{commands.ActivityRecording, commands.ActivityTranscribing, commands.ActivityLoading} {
		status := commands.Status{Title: "Dictate", Subtitle: "Listening", Tone: commands.ToneLoading, Activity: activity}
		if Width(status) >= 300 || Height(status) >= 64 {
			t.Fatalf("live state still uses a full toast: %dx%d", Width(status), Height(status))
		}
		tester := ui.NewTester(func(c *ui.Context) { Indicator(c, status) }, Width(status), Height(status))
		tester.SetPreferences(ui.Preferences{ReduceMotion: true})
		tester.Frame()
		for _, text := range []string{status.Title, status.Subtitle} {
			r, found := tester.Find(text)
			if !found || r.Y+r.H > float32(Height(status)-3) || r.X+r.W > float32(Width(status)-3) {
				t.Fatalf("%q is outside the compact surface: %+v", text, r)
			}
		}
		if tester.FramePending() {
			t.Fatal("reduced-motion indicator kept requesting frames")
		}
		tester.SetVibrancy(true)
		tester.Frame()
		for _, text := range []string{status.Title, status.Subtitle} {
			r, found := tester.Find(text)
			if !found || r.Y+r.H > float32(Height(status)) || r.X+r.W > float32(Width(status)) {
				t.Fatalf("%q is outside the native material: %+v", text, r)
			}
		}
	}
}

func TestIndicatorExplanationFitsWithoutTruncation(t *testing.T) {
	for _, message := range []string{
		"Dictation unavailable: Allow microphone access in System Settings, then try again.",
		"Microphone unavailable.\nChoose a connected input device and try again.",
		strings.Repeat("无法访问麦克风，请检查系统权限。", 3),
	} {
		status := commands.Status{Title: "Dictate", Subtitle: message, Tone: commands.ToneError}
		tester := ui.NewTester(func(c *ui.Context) { Indicator(c, status) }, Width(status), Height(status))
		tester.SetPreferences(ui.Preferences{ReduceMotion: true})
		tester.Frame()
		r, found := tester.Find(message)
		if !found || r.Y+r.H > float32(Height(status)-3) || r.X+r.W > float32(Width(status)-3) {
			t.Fatalf("explanation is clipped: %+v in %dx%d", r, Width(status), Height(status))
		}
	}
}

func TestIndicatorActivityStopsOnTerminalStatus(t *testing.T) {
	status := commands.Status{Title: "Dictate", Subtitle: "Listening", Tone: commands.ToneLoading, Activity: commands.ActivityRecording}
	tester := ui.NewTester(func(c *ui.Context) { Indicator(c, status) }, BaseWidth, BaseHeight)
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("recording did not show activity")
	}
	status = commands.Status{Title: "Dictate", Subtitle: "Inserted", Tone: commands.ToneSuccess}
	tester.Frame()
	tester.Advance(300 * time.Millisecond)
	if tester.FramePending() {
		t.Fatal("completed indicator kept its activity loop alive")
	}
}

func TestIndicatorWithoutLiveActivityStaysStill(t *testing.T) {
	status := commands.Status{Title: "Dictate", Subtitle: "No speech detected", Tone: commands.ToneLoading}
	tester := ui.NewTester(func(c *ui.Context) { Indicator(c, status) }, Width(status), Height(status))
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("a neutral result claimed live activity")
	}
}
