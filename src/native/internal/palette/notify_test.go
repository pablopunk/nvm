package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestNotifyGoesToTheHUDWhetherThePaletteIsUpOrHidden pins that every message reaches the HUD with its title and tone,
// as 1.x's HUD shows every action result.
func TestNotifyGoesToTheHUDWhetherThePaletteIsUpOrHidden(t *testing.T) {
	var shown []Feedback
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true, Feedback: func(f Feedback) { shown = append(shown, f) }})
	win := newFakeWindow()
	p.Attach(win)

	win.visible = true
	p.Notify(Feedback{Title: "Paste", Message: "Paste failed.", Tone: feedback.ToneError})
	win.visible = false
	p.Notify(Feedback{Title: "Paste", Message: "Pasted", Tone: feedback.ToneDefault})
	if len(shown) != 2 || shown[0].Tone != feedback.ToneError || shown[0].Title != "Paste" || shown[1].Message != "Pasted" {
		t.Fatalf("the HUD got %+v, want both messages with their titles and tones", shown)
	}
}

// TestToastIsAFeedbackWithoutATitle keeps the plain toast of the palette an untitled message, which the app titles
// with its name.
func TestToastIsAFeedbackWithoutATitle(t *testing.T) {
	var shown []Feedback
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true, Feedback: func(f Feedback) { shown = append(shown, f) }})
	p.Attach(newFakeWindow())
	p.Toast("Quitting Mail…")
	if len(shown) != 1 || shown[0] != (Feedback{Message: "Quitting Mail…"}) {
		t.Fatalf("toast = %+v, want an untitled default message", shown)
	}
}

// TestAToastRaisedByAnActionTakesItsTitle pins 1.x's action feedback: a toast an action raises while the palette is
// hidden carries that action's title, and a toast raised afterwards carries none.
func TestAToastRaisedByAnActionTakesItsTitle(t *testing.T) {
	var shown []Feedback
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true, Feedback: func(f Feedback) { shown = append(shown, f) }})
	p.Attach(newFakeWindow())
	p.runAction(search.Action{Title: "Quit Mail", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Toast("Quitting Mail…") }})
	if len(shown) != 1 || shown[0].Title != "Quit Mail" || shown[0].Message != "Quitting Mail…" {
		t.Fatalf("the action's toast = %+v, want the Quit Mail title", shown)
	}
	p.Toast("later")
	if shown[1].Title != "" {
		t.Fatalf("a toast after the action carries %q, want none", shown[1].Title)
	}
}
