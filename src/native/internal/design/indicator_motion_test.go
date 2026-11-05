package design

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestIndicatorEntrySettlesAndStatusUpdatesDoNotReplayIt(t *testing.T) {
	sequence := uint64(1)
	status := "Listening"
	tester := ui.NewTester(func(c *ui.Context) {
		IndicatorEntrance(c, sequence, func() { ui.Text(c, status) })
	}, 320, 64)
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("newly shown indicator did not animate")
	}
	tester.Advance(250 * time.Millisecond)
	if tester.FramePending() {
		t.Fatal("settled indicator kept requesting frames")
	}
	status = "Transcribing"
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("status update replayed the entry animation")
	}
	sequence++
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("showing the hidden indicator again did not animate")
	}
	tester.Advance(250 * time.Millisecond)
	if tester.FramePending() {
		t.Fatal("reopened indicator kept requesting frames after settling")
	}
}

func TestIndicatorEntryRespectsReducedMotion(t *testing.T) {
	tester := ui.NewTester(func(c *ui.Context) {
		IndicatorEntrance(c, 1, func() { ui.Text(c, "Listening") })
	}, 320, 64)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("reduced-motion indicator kept animating")
	}
}
