package design

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestLoadingPlaceholdersNeverScheduleFrames(t *testing.T) {
	for _, reduced := range []bool{false, true} {
		tester := ui.NewTester(func(c *ui.Context) { LoadingRow(c); LoadingCell(c, false); LoadingCell(c, true) }, 720, 470)
		tester.SetPreferences(ui.Preferences{ReduceMotion: reduced})
		tester.Frame()
		if tester.FramePending() {
			t.Fatal("static placeholders requested animation frames")
		}
	}
}
