package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// centreTolerance is how far, in DIPs, the stage may sit from the middle of the page and still count as centred.
const centreTolerance = 20

// stageBounds is the union of the labelled texts on the page: the keys and the caption the test looks for.
func stageBounds(t *testing.T, tt *ui.Tester, labels ...string) (left, top, right, bottom float32) {
	t.Helper()
	for i, label := range labels {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the page has no %q", label)
		}
		if i == 0 || r.X < left {
			left = r.X
		}
		if i == 0 || r.Y < top {
			top = r.Y
		}
		if i == 0 || r.X+r.W > right {
			right = r.X + r.W
		}
		if i == 0 || r.Y+r.H > bottom {
			bottom = r.Y + r.H
		}
	}
	return left, top, right, bottom
}

// TestRecordedKeysAreLargeAndCentred records Super+Alt+V and checks the keys are large, centred on the page, and that
// the caption sits under them.
func TestRecordedKeysAreLargeAndCentred(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(ui.Super|ui.Alt, ui.KeyV)
	tt.Frame()

	keyLeft, keyTop, keyRight, keyBottom := stageBounds(t, tt, "Super", "Alt", "V")
	if keyBottom-keyTop < 30 {
		t.Fatalf("key labels are %.0f DIPs tall, want large keys", keyBottom-keyTop)
	}
	if centre := (keyLeft + keyRight) / 2; centre < width/2-centreTolerance || centre > width/2+centreTolerance {
		t.Fatalf("keys centred at x=%.0f, want the page centre x=%.0f", centre, float32(width)/2)
	}

	_, captionTop, _, captionBottom := stageBounds(t, tt, "Save shortcut for “Clipboard History”")
	if captionTop < keyBottom {
		t.Fatalf("caption starts at y=%.0f, above the keys that end at y=%.0f", captionTop, keyBottom)
	}
	stageCentre := (keyTop + captionBottom) / 2
	if stageCentre < height/2-centreTolerance || stageCentre > height/2+centreTolerance {
		t.Fatalf("keys and caption centred at y=%.0f, want the page centre y=%.0f", stageCentre, float32(height)/2)
	}
}

// TestPromptIsCentredOnAnEmptyRecorder checks the empty prompt sits in the middle of the page, not at its top.
func TestPromptIsCentredOnAnEmptyRecorder(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()

	_, top, _, bottom := stageBounds(t, tt, "Press a keyboard shortcut", "Use at least one modifier, then press Enter")
	if centre := (top + bottom) / 2; centre < height/2-centreTolerance || centre > height/2+centreTolerance {
		t.Fatalf("prompt centred at y=%.0f, want the page centre y=%.0f", centre, float32(height)/2)
	}
}
