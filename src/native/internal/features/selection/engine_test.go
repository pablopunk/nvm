package selection

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// engineRig is an engine over fakes whose source window the test can move.
type engineRig struct {
	set    *fake.Set
	engine *Engine
	source platform.FocusTarget
}

func newEngineRig(t *testing.T) *engineRig {
	t.Helper()
	set := fake.NewSet()
	rig := &engineRig{set: set, source: fixSource}
	rig.engine = New(Deps{
		Clock:        set.Clock,
		Clipboard:    set.Clipboard,
		SelectedText: set.SelectedText,
		Paster:       set.Paster,
		Focus:        set.Focus,
		Source:       func() platform.FocusTarget { return rig.source },
		RestoreDelay: 500 * time.Millisecond,
	})
	return rig
}

// TestReplacePastesAndPutsTheDisplacedClipboardBack: the paste lands, the replacement text is on the clipboard until
// the restore delay passes, and then the clipboard the user had comes back.
func TestReplacePastesAndPutsTheDisplacedClipboardBack(t *testing.T) {
	rig := newEngineRig(t)
	if err := rig.set.Clipboard.WriteText("keep me"); err != nil {
		t.Fatal(err)
	}
	replaced, err := rig.engine.Replace("This is fixed.")
	if err != nil || !replaced {
		t.Fatalf("Replace = %v, %v; want the paste to land", replaced, err)
	}
	if got := rig.set.Paster.Requests(); len(got) != 1 {
		t.Fatalf("paste requests = %v, want one", got)
	}
	rig.set.Clock.AwaitTimerDueIn(500 * time.Millisecond)
	rig.set.Clock.Advance(500 * time.Millisecond)
	waitClipboard(t, rig.set.Clipboard, "keep me")
}

// TestUndoOnlyReachesTheWindowTheReplacementLandedIn: a source window that changed after the replacement is refused.
func TestUndoOnlyReachesTheWindowTheReplacementLandedIn(t *testing.T) {
	rig := newEngineRig(t)
	if _, err := rig.engine.Replace("fixed"); err != nil {
		t.Fatal(err)
	}
	rig.source = platform.FocusTarget{Window: "0x9999", PID: 1}
	if err := rig.engine.Undo(); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("Undo in another window = %v, want ErrSourceChanged", err)
	}
	if len(rig.set.Paster.Undos()) != 0 {
		t.Fatal("an undo was sent to a window it did not land in")
	}
}

// TestUndoSendsOncePerReplacement: a replacement can be undone once; a second undo has nothing to put back.
func TestUndoSendsOncePerReplacement(t *testing.T) {
	rig := newEngineRig(t)
	if _, err := rig.engine.Replace("fixed"); err != nil {
		t.Fatal(err)
	}
	if err := rig.engine.Undo(); err != nil {
		t.Fatalf("Undo = %v, want the undo gesture sent", err)
	}
	if err := rig.engine.Undo(); !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("second Undo = %v, want ErrNothingToUndo", err)
	}
	if got := len(rig.set.Paster.Undos()); got != 1 {
		t.Fatalf("undo gestures = %d, want 1", got)
	}
}

// waitClipboard polls until the clipboard holds want, which a restore goroutine sets once its timer fires.
func waitClipboard(t *testing.T, clip platform.Clipboard, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, _ := clip.ReadText()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("clipboard = %q, want %q", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}
