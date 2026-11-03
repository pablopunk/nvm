package geometry

import (
	"errors"
	"fmt"
	"testing"
)

// Ported from src/app/electron/os/window-management-memory.test.ts (the two pure cases; the third
// case drives the service and is deferred to WP4.1b). The remaining tests cover the restore-history
// rules of window-management-history.ts, which have no separate TypeScript test file.

var memoryEntry = History{RestoreFrame: &Rect{X: 1, Y: 2, Width: 3, Height: 4}}

func TestMemoryCapsRememberedWindowsPort(t *testing.T) {
	t.Run("caps remembered windows and forgets the oldest first", func(t *testing.T) {
		memory := NewMemory()
		for index := 0; index <= MaxRememberedWindows; index++ {
			memory.Write(fmt.Sprintf("1:%d", index), memoryEntry)
		}
		if memory.Len() != MaxRememberedWindows {
			t.Fatalf("len = %d, want %d", memory.Len(), MaxRememberedWindows)
		}
		if _, ok := memory.Read("1:0"); ok {
			t.Fatal("oldest entry was not evicted")
		}
		if got, ok := memory.Read(fmt.Sprintf("1:%d", MaxRememberedWindows)); !ok || *got.RestoreFrame != *memoryEntry.RestoreFrame {
			t.Fatalf("newest entry = %+v, %v", got, ok)
		}
	})

	t.Run("drops only the windows that belong to the given pid", func(t *testing.T) {
		memory := NewMemory()
		memory.Write("1:5", memoryEntry)
		memory.Write("12:5", memoryEntry)
		memory.DropProcess(1)
		if _, ok := memory.Read("1:5"); ok {
			t.Fatal("pid 1 window was kept")
		}
		if _, ok := memory.Read("12:5"); !ok {
			t.Fatal("pid 12 window was dropped")
		}
	})
}

func TestRememberAppliedKeepsRestoreFrameRulesPort(t *testing.T) {
	before := rectOf(100, 100, 600, 400)
	after := rectOf(0, 25, 720, 875)
	window := resizableWindow(before)

	t.Run("the first change captures the pre-layout frame as the restore target", func(t *testing.T) {
		next := RememberApplied(nil, window, after, true, CycleRecord{Group: EdgeLeft, Index: 0})
		if next.RestoreFrame == nil || *next.RestoreFrame != before {
			t.Fatalf("restore frame = %v, want %+v", next.RestoreFrame, before)
		}
		if next.LastApplied == nil || next.LastApplied.Frame != after || next.LastApplied.CycleGroup != EdgeLeft {
			t.Fatalf("last applied = %+v", next.LastApplied)
		}
	})

	t.Run("a change that did not move the window keeps the earlier restore frame", func(t *testing.T) {
		earlier := History{RestoreFrame: &Rect{X: 9, Y: 9, Width: 9, Height: 9}}
		next := RememberApplied(&earlier, window, before, false, CycleRecord{})
		if next.RestoreFrame == nil || *next.RestoreFrame != *earlier.RestoreFrame {
			t.Fatalf("restore frame = %v, want the earlier one", next.RestoreFrame)
		}
	})

	t.Run("a window moved by hand after a layout captures a new restore frame", func(t *testing.T) {
		previous := History{RestoreFrame: &Rect{X: 9, Y: 9, Width: 9, Height: 9}, LastApplied: &AppliedLayout{Frame: after}}
		moved := rectOf(300, 300, 500, 400)
		next := RememberApplied(&previous, resizableWindow(moved), rectOf(0, 25, 960, 875), true, CycleRecord{})
		if next.RestoreFrame == nil || *next.RestoreFrame != moved {
			t.Fatalf("restore frame = %v, want %+v", next.RestoreFrame, moved)
		}
	})
}

func TestForgetCycleGroupEndsTheCyclePort(t *testing.T) {
	restore := Rect{X: 1, Y: 1, Width: 1, Height: 1}
	previous := History{RestoreFrame: &restore, LastApplied: &AppliedLayout{Frame: rectOf(0, 0, 720, 900), CycleGroup: EdgeLeft, CycleIndex: 1}}
	next := ForgetCycleGroup(previous)
	if next.LastApplied == nil || next.LastApplied.CycleGroup != "" || next.LastApplied.CycleIndex != 1 {
		t.Fatalf("last applied = %+v", next.LastApplied)
	}
	if next.RestoreFrame == nil || *next.RestoreFrame != restore {
		t.Fatalf("restore frame changed: %v", next.RestoreFrame)
	}
	if previous.LastApplied.CycleGroup != EdgeLeft {
		t.Fatal("ForgetCycleGroup mutated its input")
	}
}

func TestRestoreDisplayFollowsTheStoredFrame(t *testing.T) {
	displays := []Display{geoPrimary, geoLeft}
	stored := Rect{X: -1500, Y: 100, Width: 600, Height: 400}
	history := History{RestoreFrame: &stored}
	picked, err := RestoreDisplay(displays, &history)
	if err != nil || picked.ID != geoLeft.ID {
		t.Fatalf("RestoreDisplay = %q (%v), want the display that holds the stored frame", picked.ID, err)
	}
	if _, err := RestoreDisplay(displays, &History{}); !errors.Is(err, ErrNoHistory) {
		t.Fatalf("error = %v, want ErrNoHistory without a stored frame", err)
	}
}

func TestRestoreWithoutHistoryFailsPort(t *testing.T) {
	_, _, err := Resolve(Layout{Kind: KindRestore}, displayOver("main", kindsWorkArea), resizableWindow(kindsWindow), UniformGaps(0), nil)
	if !errors.Is(err, ErrNoHistory) {
		t.Fatalf("error = %v, want ErrNoHistory", err)
	}
}
