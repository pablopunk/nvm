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

// installFakeXdotool puts an executable named xdotool on a private PATH and points DISPLAY at it, so pasteCapable
// sees an X11 session with the tool installed. The tool itself is never run: the helper is scripted.
func installFakeXdotool(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, xdotoolBinary), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("DISPLAY", ":99")
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("WAYLAND_DISPLAY", "")
}

type blockingPasteHelper struct{ entered chan struct{} }

func (h blockingPasteHelper) Run(ctx context.Context, _ string, _ []string, _ []byte) (platform.HelperResult, error) {
	close(h.entered)
	<-ctx.Done()
	return platform.HelperResult{}, ctx.Err()
}

func TestContextPasteCancellationPreventsWaitAndGesture(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	called := false
	p := NewPaster(helper, func(context.Context) error { called = true; return nil }).(platform.ContextPaster)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.PasteIntoFrontmostContext(ctx, "Editor"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if called || len(helper.Calls()) != 0 {
		t.Fatalf("wait called %v, helper calls %d", called, len(helper.Calls()))
	}
}

func TestContextPasteCancellationDuringWaitPreventsGesture(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	entered := make(chan struct{})
	p := NewPaster(helper, func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() }).(platform.ContextPaster)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.PasteIntoFrontmostContext(ctx, "Editor") }()
	<-entered
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if len(helper.Calls()) != 0 {
		t.Fatalf("helper calls = %d", len(helper.Calls()))
	}
}

func TestContextPasteCancellationInterruptsHelper(t *testing.T) {
	installFakeXdotool(t)
	entered := make(chan struct{})
	p := NewPaster(blockingPasteHelper{entered: entered}, nil).(platform.ContextPaster)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.PasteIntoFrontmostContext(ctx, "") }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("helper was not invoked")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

// TestX11PasteSendsCtrlVToTheFocusedWindow: with xdotool present, the gesture is one key event with Ctrl+V and
// --clearmodifiers.
func TestX11PasteSendsCtrlVToTheFocusedWindow(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	helper.Script(xdotoolBinary, fake.HelperScript{})
	if err := pasteGesture(context.Background(), helper, ""); err != nil {
		t.Fatalf("pasteGesture = %v", err)
	}
	calls := helper.Calls()
	if len(calls) != 1 || !slices.Equal(calls[0].Args, []string{"key", "--clearmodifiers", "ctrl+v"}) {
		t.Fatalf("calls = %+v", calls)
	}
}

// TestX11PasteChecksTheFocusedWindowWhenAnAppIsNamed: when the caller names the app that must have focus, a
// different focused window is a conflict and no key is sent.
func TestX11PasteChecksTheFocusedWindowWhenAnAppIsNamed(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	helper.Script(xdotoolBinary, fake.HelperScript{Stdout: []byte("4194305\n")})
	helper.Script("xprop", fake.HelperScript{Stdout: []byte(`WM_CLASS(STRING) = "terminal", "Terminal"` + "\n")})
	if err := pasteGesture(context.Background(), helper, "Firefox"); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("pasteGesture = %v, want ErrConflict", err)
	}
	if calls := helper.Calls(); len(calls) != 2 || calls[1].Binary != "xprop" {
		t.Fatalf("a conflicting paste sent keys or skipped class lookup: %+v", calls)
	}
}

// TestX11PasteIsUnsupportedWithoutXdotoolOrOnWayland: a missing tool, no display, or a Wayland session reports
// ErrUnsupported and sends nothing.
func TestX11PasteIsUnsupportedWithoutXdotoolOrOnWayland(t *testing.T) {
	installFakeXdotool(t)
	t.Setenv("PATH", t.TempDir())
	if err := pasteGesture(context.Background(), fake.NewHelper(), ""); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("without xdotool: %v", err)
	}

	installFakeXdotool(t)
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	if pasteCapable("linux", os.Getenv, programInstalled) {
		t.Fatal("a Wayland session reports paste as supported")
	}

	installFakeXdotool(t)
	t.Setenv("DISPLAY", "")
	if pasteCapable("linux", os.Getenv, programInstalled) {
		t.Fatal("a session without DISPLAY reports paste as supported")
	}
}

// TestX11PasteMapsAMissingBinaryToUnsupported: the helper reports a missing binary as exec.ErrNotFound; the paste
// reports ErrUnsupported rather than a raw exec error.
func TestX11PasteMapsAMissingBinaryToUnsupported(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	helper.Script(xdotoolBinary, fake.HelperScript{Err: exec.ErrNotFound})
	if err := pasteGesture(context.Background(), helper, ""); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("pasteGesture = %v, want ErrUnsupported", err)
	}
}

// TestPasteWaitsForThePaletteBeforeTheGesture: the port calls the hide wait first, and sends nothing when the wait
// fails.
func TestPasteWaitsForThePaletteBeforeTheGesture(t *testing.T) {
	installFakeXdotool(t)
	helper := fake.NewHelper()
	waited := false
	wait := func(context.Context) error {
		waited = true
		return platform.ErrDenied
	}
	err := NewPaster(helper, wait).PasteIntoFrontmost("")
	if !waited || !errors.Is(err, platform.ErrDenied) {
		t.Fatalf("waited %v, err %v", waited, err)
	}
	if len(helper.Calls()) != 0 {
		t.Fatal("keys were sent before the palette hid")
	}
}
