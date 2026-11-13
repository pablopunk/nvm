//go:build linux

package real

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestX11PasteFallsBackToFocusedClassWhenActiveWindowIsMissing(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	helper.Script(xdotoolBinary,
		fake.HelperScript{ExitCode: 1},
		fake.HelperScript{Stdout: []byte("4194305\n")},
		fake.HelperScript{},
	)
	helper.Script("xprop", fake.HelperScript{Stdout: []byte(`WM_CLASS(STRING) = "editor", "Editor"` + "\n")})

	if err := pasteGesture(context.Background(), helper, "Editor"); err != nil {
		t.Fatalf("pasteGesture = %v", err)
	}
	calls := helper.Calls()
	want := [][]string{
		{"getactivewindow"},
		{"getwindowfocus"},
		{"key", "--clearmodifiers", "ctrl+v"},
	}
	if len(calls) != len(want)+1 || calls[2].Binary != "xprop" || !reflect.DeepEqual(calls[2].Args, []string{"-id", "4194305", "WM_CLASS"}) {
		t.Fatalf("xprop lookup call = %+v", calls)
	}
	for index, args := range want {
		callIndex := index
		if index >= 2 {
			callIndex++
		}
		if !reflect.DeepEqual(calls[callIndex].Args, args) {
			t.Fatalf("call %d = %+v, want args %v", callIndex, calls[callIndex], args)
		}
	}
}

func TestX11PasteRejectsMismatchingFocusedClassBeforeGesture(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	helper.Script(xdotoolBinary, fake.HelperScript{Stdout: []byte("4194305\n")})
	helper.Script("xprop", fake.HelperScript{Stdout: []byte(`WM_CLASS(STRING) = "terminal", "Terminal"` + "\n")})

	if err := pasteGesture(context.Background(), helper, "Editor"); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("pasteGesture = %v, want ErrConflict", err)
	}
	calls := helper.Calls()
	if len(calls) != 2 || calls[0].Binary != xdotoolBinary || calls[1].Binary != "xprop" {
		t.Fatalf("mismatching class triggered unexpected calls: %+v", calls)
	}
	if !reflect.DeepEqual(calls[0].Args, []string{"getactivewindow"}) || !reflect.DeepEqual(calls[1].Args, []string{"-id", "4194305", "WM_CLASS"}) {
		t.Fatalf("lookup calls = %+v", calls)
	}
}
