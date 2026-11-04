package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestNotifyGoesToTheHUDOnlyWhenThePaletteIsHidden pins where a message raised with a title and a tone lands: under
// the field while the palette is up, and in the HUD, with its title and tone, once it has hidden.
func TestNotifyGoesToTheHUDOnlyWhenThePaletteIsHidden(t *testing.T) {
	var shown []Feedback
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true, Feedback: func(f Feedback) { shown = append(shown, f) }})
	win := newFakeWindow()
	p.Attach(win)

	win.visible = true
	p.Notify(Feedback{Title: "Paste", Message: "Paste failed.", Tone: feedback.ToneError})
	if len(shown) != 0 || p.toastMsg != "Paste failed." {
		t.Fatalf("with the palette up the message went to the HUD (%d) or the field holds %q", len(shown), p.toastMsg)
	}

	win.visible = false
	p.Notify(Feedback{Title: "Paste", Message: "Pasted", Tone: feedback.ToneDefault})
	if len(shown) != 1 || shown[0].Title != "Paste" || shown[0].Message != "Pasted" || shown[0].Tone != feedback.ToneDefault {
		t.Fatalf("hidden palette sent %+v to the HUD, want the Paste title and default tone", shown)
	}
}

// TestToastIsAFeedbackWithoutATitle keeps the plain toast of the palette an untitled message, which the app titles
// with its name.
func TestToastIsAFeedbackWithoutATitle(t *testing.T) {
	var shown []Feedback
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true, Feedback: func(f Feedback) { shown = append(shown, f) }})
	win := newFakeWindow()
	p.Attach(win)
	win.visible = true
	p.Toast("Quitting Mail…")
	if len(shown) != 0 {
		t.Fatalf("a visible window with a toast sent it to the HUD: %+v", shown)
	}
	win.visible = false
	p.Toast("Quitting Mail…")
	if len(shown) != 1 || shown[0] != (Feedback{Message: "Quitting Mail…"}) {
		t.Fatalf("hidden toast = %+v, want an untitled default message", shown)
	}
}
