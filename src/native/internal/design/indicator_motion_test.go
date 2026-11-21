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

func TestIndicatorActivityLoopsWhileMountedAndStopsWhenIdle(t *testing.T) {
	active := true
	tester := ui.NewTester(func(c *ui.Context) {
		if active {
			IndicatorActivity(c, "recording", c.Preferences().ReduceMotion)
		}
	}, 40, 40)
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("active phase did not request another frame")
	}
	tester.Advance(2100 * time.Millisecond)
	if !tester.FramePending() {
		t.Fatal("active phase stopped requesting frames after two seconds")
	}
	active = false
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("idle phase kept requesting frames after unmount")
	}
}

func TestIndicatorActivityRespectsReducedMotion(t *testing.T) {
	tester := ui.NewTester(func(c *ui.Context) { IndicatorActivity(c, "transcribing", c.Preferences().ReduceMotion) }, 40, 40)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()
	if tester.FramePending() {
		t.Fatal("reduced-motion activity kept requesting frames")
	}
}
