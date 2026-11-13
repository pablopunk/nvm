//go:build linux

package real

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// installSession puts empty executables with the given names on a private PATH and sets the session variables,
// so the capability check sees those tools. The tools are never run: the helper is scripted.
func installSession(t *testing.T, env map[string]string, tools ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range tools {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	for _, key := range []string{"XDG_SESSION_TYPE", "WAYLAND_DISPLAY", "DISPLAY", "HYPRLAND_INSTANCE_SIGNATURE"} {
		t.Setenv(key, env[key])
	}
}

func hyprlandWaylandEnv() map[string]string {
	env := map[string]string{"HYPRLAND_INSTANCE_SIGNATURE": "abc_1_2"}
	for key, value := range waylandEnv {
		env[key] = value
	}
	return env
}

func TestWaylandPasteHoldsCtrlAroundV(t *testing.T) {
	installSession(t, waylandEnv, wtypeBinary)
	helper := fake.NewHelper()
	helper.Script(wtypeBinary, fake.HelperScript{})
	if err := pasteGesture(context.Background(), helper, ""); err != nil {
		t.Fatalf("pasteGesture = %v", err)
	}
	calls := helper.Calls()
	want := []string{"-s", "50", "-M", "ctrl", "v", "-m", "ctrl"}
	if len(calls) != 1 || calls[0].Binary != wtypeBinary || !slices.Equal(calls[0].Args, want) {
		t.Fatalf("calls = %+v, want one wtype call with %v", calls, want)
	}
}

func TestWaylandUndoSendsCtrlZ(t *testing.T) {
	installSession(t, waylandEnv, wtypeBinary)
	helper := fake.NewHelper()
	helper.Script(wtypeBinary, fake.HelperScript{})
	if err := undoGesture(context.Background(), helper, ""); err != nil {
		t.Fatalf("undoGesture = %v", err)
	}
	if calls := helper.Calls(); len(calls) != 1 || calls[0].Args[len(calls[0].Args)-3] != "z" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestWaylandPasteNeverUsesXdotool(t *testing.T) {
	installSession(t, xwaylandEnv, xdotoolBinary)
	helper := fake.NewHelper()
	if err := pasteGesture(context.Background(), helper, ""); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("pasteGesture = %v, want ErrUnsupported", err)
	}
	if len(helper.Calls()) != 0 {
		t.Fatalf("calls = %+v, want none", helper.Calls())
	}
}

func TestX11PasteKeepsXdotoolWhenWtypeIsAlsoInstalled(t *testing.T) {
	installSession(t, x11Env, xdotoolBinary, wtypeBinary)
	helper := fake.NewHelper()
	helper.Script(xdotoolBinary, fake.HelperScript{})
	if err := pasteGesture(context.Background(), helper, ""); err != nil {
		t.Fatalf("pasteGesture = %v", err)
	}
	calls := helper.Calls()
	if len(calls) != 1 || calls[0].Binary != xdotoolBinary {
		t.Fatalf("calls = %+v, want xdotool", calls)
	}
}

func TestX11PasteIsUnsupportedWithOnlyWtype(t *testing.T) {
	installSession(t, x11Env, wtypeBinary)
	if err := pasteGesture(context.Background(), fake.NewHelper(), ""); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("pasteGesture = %v, want ErrUnsupported", err)
	}
}

func TestWaylandPasteReportsWtypeFailures(t *testing.T) {
	tests := []struct {
		name      string
		script    fake.HelperScript
		want      error
		anyFailed bool
	}{
		{name: "missing binary", script: fake.HelperScript{Err: exec.ErrNotFound}, want: platform.ErrUnsupported},
		{name: "compositor refuses virtual keyboard", script: fake.HelperScript{ExitCode: 1, Stderr: []byte("Compositor does not support the virtual keyboard protocol")}, anyFailed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installSession(t, waylandEnv, wtypeBinary)
			helper := fake.NewHelper()
			helper.Script(wtypeBinary, test.script)
			err := pasteGesture(context.Background(), helper, "")
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("pasteGesture = %v, want %v", err, test.want)
			}
			if test.anyFailed && (err == nil || errors.Is(err, platform.ErrUnsupported)) {
				t.Fatalf("pasteGesture = %v, want a failure naming wtype", err)
			}
		})
	}
}

func TestWaylandPasteChecksTheHyprlandClassWhenAnAppIsNamed(t *testing.T) {
	tests := []struct {
		name      string
		class     string
		wantErr   error
		wantPaste bool
	}{
		{name: "matching class", class: "Editor", wantPaste: true},
		{name: "different class", class: "kitty", wantErr: platform.ErrConflict},
		{name: "no focused window", class: "", wantErr: platform.ErrNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installSession(t, hyprlandWaylandEnv(), wtypeBinary, "hyprctl")
			helper := fake.NewHelper()
			helper.Script("hyprctl", fake.HelperScript{Stdout: []byte(`{"class": "` + test.class + `", "title": "x"}`)})
			helper.Script(wtypeBinary, fake.HelperScript{})
			err := pasteGesture(context.Background(), helper, "editor")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("pasteGesture = %v, want %v", err, test.wantErr)
			}
			calls := helper.Calls()
			pasted := len(calls) == 2 && calls[1].Binary == wtypeBinary
			if pasted != test.wantPaste {
				t.Fatalf("calls = %+v, want paste %v", calls, test.wantPaste)
			}
		})
	}
}

func TestWaylandPasteTypesNothingWhenTheAppCannotBeNamed(t *testing.T) {
	installSession(t, waylandEnv, wtypeBinary)
	helper := fake.NewHelper()
	if err := pasteGesture(context.Background(), helper, "Editor"); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("pasteGesture = %v, want ErrUnsupported without Hyprland", err)
	}
	if len(helper.Calls()) != 0 {
		t.Fatalf("calls = %+v, want none", helper.Calls())
	}
}

func TestWaylandFrontmostAppIDReadsTheHyprlandClass(t *testing.T) {
	installSession(t, hyprlandWaylandEnv(), wtypeBinary, "hyprctl")
	helper := fake.NewHelper()
	helper.Script("hyprctl", fake.HelperScript{Stdout: []byte(`{"class": "firefox"}`)})
	got, err := frontmostAppID(context.Background(), helper, 0)
	if err != nil || got != "firefox" {
		t.Fatalf("frontmostAppID = %q, %v", got, err)
	}
}

func TestWtypeChordArgsHoldModifiersInOrderAndReleaseInReverse(t *testing.T) {
	got := wtypeChordArgs("ctrl+shift+v", 20*time.Millisecond)
	want := []string{"-s", "20", "-M", "ctrl", "-M", "shift", "v", "-m", "shift", "-m", "ctrl"}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}
