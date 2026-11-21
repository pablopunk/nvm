package real

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const testHelperBinary = "/unused/macos-selected-text"

// scriptedSelection scripts lsappinfo for a frontmost Slack (pid 777), then the helper's own answer.
func scriptedSelection(pid int, answer fake.HelperScript) *fake.Helper {
	helper := fake.NewHelper()
	helper.Script(lsappinfoPath,
		fake.HelperScript{Stdout: []byte("ASN:0x0-0x1234:")},
		fake.HelperScript{Stdout: []byte(`"CFBundleIdentifier"="com.tinyspeck.slackmacgap"` + "\n" + `"pid"=` + strconv.Itoa(pid))},
	)
	helper.Script(testHelperBinary, answer)
	return helper
}

func TestAccessibleSelectionReadsTheFrontmostAppThroughTheHelper(t *testing.T) {
	helper := scriptedSelection(777, fake.HelperScript{Stdout: []byte("hello")})
	got, err := macAccessibleSelection(context.Background(), helper, testHelperBinary)
	if err != nil || got != "hello" {
		t.Fatalf("macAccessibleSelection = %q, %v", got, err)
	}
	calls := helper.Calls()
	last := calls[len(calls)-1]
	if last.Binary != testHelperBinary || len(last.Args) != 2 || last.Args[0] != "read" || last.Args[1] != "777" {
		t.Fatalf("helper call = %+v, want `read 777`", last)
	}
}

func TestAccessibleSelectionIsEmptyWhenTheAppExposesNone(t *testing.T) {
	helper := scriptedSelection(777, fake.HelperScript{ExitCode: helperExitNoSelection})
	if got, err := macAccessibleSelection(context.Background(), helper, testHelperBinary); err != nil || got != "" {
		t.Fatalf("macAccessibleSelection = %q, %v; want empty and no error", got, err)
	}
}

// TestCopySelectionAsksTheHelperNotSystemEvents: the helper posts Command+C with exactly the Command flag after the
// shortcut's modifiers are up; an osascript keystroke would inherit the held Control and Alt.
func TestCopySelectionAsksTheHelperNotSystemEvents(t *testing.T) {
	helper := scriptedSelection(777, fake.HelperScript{})
	if err := macCopySelection(context.Background(), helper, testHelperBinary); err != nil {
		t.Fatalf("macCopySelection: %v", err)
	}
	for _, call := range helper.Calls() {
		if call.Binary == "osascript" {
			t.Fatalf("copy used osascript: %+v", call)
		}
	}
	calls := helper.Calls()
	last := calls[len(calls)-1]
	if last.Binary != testHelperBinary || last.Args[0] != "copy" || last.Args[1] != "777" {
		t.Fatalf("helper call = %+v, want `copy 777`", last)
	}
}

func TestSelectionHelperExitCodesBecomePortErrors(t *testing.T) {
	tests := map[int]error{
		helperExitDenied:        platform.ErrDenied,
		helperExitNotFrontmost:  platform.ErrConflict,
		helperExitModifiersHeld: platform.ErrConflict,
	}
	for code, want := range tests {
		helper := scriptedSelection(777, fake.HelperScript{ExitCode: code})
		if err := macCopySelection(context.Background(), helper, testHelperBinary); !errors.Is(err, want) {
			t.Errorf("exit %d: error = %v, want %v", code, err, want)
		}
	}
	helper := scriptedSelection(777, fake.HelperScript{ExitCode: 1})
	if err := macCopySelection(context.Background(), helper, testHelperBinary); err == nil {
		t.Error("exit 1: want an error")
	}
}

func TestSelectionNeverTargetsNevermindItself(t *testing.T) {
	helper := scriptedSelection(os.Getpid(), fake.HelperScript{})
	if err := macCopySelection(context.Background(), helper, testHelperBinary); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("error = %v, want ErrConflict", err)
	}
	for _, call := range helper.Calls() {
		if call.Binary == testHelperBinary {
			t.Fatalf("helper ran against Nevermind: %+v", call)
		}
	}
}
