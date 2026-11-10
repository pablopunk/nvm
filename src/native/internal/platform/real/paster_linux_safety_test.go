//go:build linux

package real

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestX11PasteFallsBackWhenActiveWindowIDIsInvalid(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	helper.Script(xdotoolBinary,
		fake.HelperScript{Stdout: []byte("None\n")},
		fake.HelperScript{Stdout: []byte("4194305\n")},
		fake.HelperScript{},
	)
	helper.Script("xprop", fake.HelperScript{Stdout: []byte(`WM_CLASS(STRING) = "editor", "Editor"` + "\n")})

	if err := pasteGesture(context.Background(), helper, "Editor"); err != nil {
		t.Fatalf("pasteGesture = %v", err)
	}
	calls := helper.Calls()
	if len(calls) != 4 || calls[0].Args[0] != "getactivewindow" || calls[1].Args[0] != "getwindowfocus" || calls[2].Binary != "xprop" || calls[3].Args[0] != "key" {
		t.Fatalf("calls = %+v, want active lookup, focus fallback, WM_CLASS lookup, and paste gesture", calls)
	}
}

func TestX11PasteStopsForInvalidWindowIDs(t *testing.T) {
	for _, invalidID := range []string{"", "None", "0", "not-a-window", "-1", "0x123"} {
		t.Run(strings.ReplaceAll(invalidID, "", "empty"), func(t *testing.T) {
			installFakeXdotool(t)
			helper := fake.NewHelper()
			helper.Script(xdotoolBinary,
				fake.HelperScript{Stdout: []byte(invalidID)},
				fake.HelperScript{Stdout: []byte(invalidID)},
			)
			if err := pasteGesture(context.Background(), helper, "Editor"); !errors.Is(err, platform.ErrNotFound) {
				t.Fatalf("pasteGesture = %v, want ErrNotFound", err)
			}
			calls := helper.Calls()
			if len(calls) != 2 || calls[0].Args[0] != "getactivewindow" || calls[1].Args[0] != "getwindowfocus" {
				t.Fatalf("calls = %+v, want only active and focused ID lookups", calls)
			}
		})
	}
}

func TestX11PasteRejectsUnusableWMClassBeforeGesture(t *testing.T) {
	tests := []struct {
		name   string
		script fake.HelperScript
	}{
		{name: "missing class", script: fake.HelperScript{Stdout: []byte("WM_NAME(STRING) = \"Editor\"\n")}},
		{name: "root class", script: fake.HelperScript{Stdout: []byte(`WM_CLASS(STRING) = "root", "ROOT"`)}},
		{name: "xprop error", script: fake.HelperScript{Err: errors.New("xprop unavailable")}},
		{name: "xprop nonzero", script: fake.HelperScript{ExitCode: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installFakeXdotool(t)
			helper := fake.NewHelper()
			helper.Script(xdotoolBinary, fake.HelperScript{Stdout: []byte("4194305")})
			helper.Script("xprop", test.script)
			if err := pasteGesture(context.Background(), helper, "Editor"); err == nil {
				t.Fatal("pasteGesture succeeded without a usable WM_CLASS")
			}
			calls := helper.Calls()
			if len(calls) != 2 || calls[1].Binary != "xprop" {
				t.Fatalf("calls = %+v, want lookup only and no key gesture", calls)
			}
		})
	}
}

func TestX11PasteAndUndoRequireMatchingFocusedClass(t *testing.T) {
	gestures := []struct {
		name string
		call func(context.Context, platform.Helper, string) error
		key  string
	}{
		{name: "paste", call: pasteGesture, key: "ctrl+v"},
		{name: "undo", call: undoGesture, key: "ctrl+z"},
	}
	for _, gesture := range gestures {
		for _, classCase := range []struct {
			name      string
			class     string
			wantError bool
		}{
			{name: "matching", class: "Editor"},
			{name: "mismatching", class: "Terminal", wantError: true},
		} {
			t.Run(gesture.name+"/"+classCase.name, func(t *testing.T) {
				installFakeXdotool(t)
				helper := fake.NewHelper()
				helper.Script(xdotoolBinary,
					fake.HelperScript{Stdout: []byte("4194305")},
					fake.HelperScript{},
				)
				helper.Script("xprop", fake.HelperScript{Stdout: []byte(`WM_CLASS(STRING) = "editor", "` + classCase.class + `"` + "\n")})

				err := gesture.call(context.Background(), helper, "Editor")
				if classCase.wantError {
					if !errors.Is(err, platform.ErrConflict) {
						t.Fatalf("gesture error = %v, want ErrConflict", err)
					}
				} else if err != nil {
					t.Fatalf("gesture error = %v", err)
				}
				calls := helper.Calls()
				wantCalls := 2
				if !classCase.wantError {
					wantCalls = 3
				}
				if len(calls) != wantCalls {
					t.Fatalf("calls = %+v, want %d calls", calls, wantCalls)
				}
				if !classCase.wantError && calls[2].Args[2] != gesture.key {
					t.Fatalf("gesture args = %v, want key %q", calls[2].Args, gesture.key)
				}
			})
		}
	}
}
