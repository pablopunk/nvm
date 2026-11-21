package real

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/x11"
)

// pasteGesture sends Ctrl+V to the focused window: with xdotool in an X11 session, with wtype in a Wayland one.
// Ctrl+V is the 1.x gesture; terminals that paste with Ctrl+Shift+V are not covered.
func pasteGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	return keyGesture(ctx, helper, expectedAppID, "ctrl+v")
}

// undoGesture sends Ctrl+Z to the focused window the same way.
func undoGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	return keyGesture(ctx, helper, expectedAppID, "ctrl+z")
}

// keyGesture sends one key chord to the focused window with the session's tool, after checking the focused window's
// class when expectedAppID is set.
func keyGesture(ctx context.Context, helper platform.Helper, expectedAppID, chord string) error {
	switch linuxPasteTool("linux", os.Getenv, programInstalled) {
	case wtypeBinary:
		return waylandKeyGesture(ctx, helper, expectedAppID, chord)
	case xdotoolBinary:
		return x11KeyGesture(ctx, helper, expectedAppID, chord)
	}
	return platform.ErrUnsupported
}

// x11KeyGesture sends the chord with xdotool.
func x11KeyGesture(ctx context.Context, helper platform.Helper, expectedAppID, chord string) error {
	if expectedAppID != "" {
		focused, err := focusedWindowClass(ctx, helper)
		if err != nil {
			return err
		}
		if !strings.EqualFold(focused, expectedAppID) {
			return platform.ErrConflict
		}
	}
	_, err := runXdotool(ctx, helper, "key", "--clearmodifiers", chord)
	return err
}

// runXdotool runs xdotool with args and returns its trimmed standard output. A missing binary is
// platform.ErrUnsupported; a non-zero exit is an error that names the failing command.
func runXdotool(ctx context.Context, helper platform.Helper, args ...string) (string, error) {
	result, err := helper.Run(ctx, xdotoolBinary, args, nil)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", platform.ErrUnsupported
		}
		return "", fmt.Errorf("xdotool %s: %w", args[0], err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("xdotool %s exited with %d", args[0], result.ExitCode)
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// frontmostAppID names the focused window's class, the identity keyGesture checks before it types: the X11 window
// class, or the Hyprland window class in a Wayland session that runs Hyprland.
func frontmostAppID(ctx context.Context, helper platform.Helper, _ int) (string, error) {
	switch linuxPasteTool("linux", os.Getenv, programInstalled) {
	case wtypeBinary:
		return hyprlandActiveClass(ctx, helper)
	case xdotoolBinary:
		return focusedWindowClass(ctx, helper)
	}
	return "", platform.ErrUnsupported
}

// focusedWindowClass resolves the EWMH active window first and falls back to the X server's input focus when
// the window manager has no _NET_ACTIVE_WINDOW property. The class is always read for the resolved window ID.
func focusedWindowClass(ctx context.Context, helper platform.Helper) (string, error) {
	id, err := runXdotool(ctx, helper, "getactivewindow")
	if err != nil || !isUsablePasteWindowID(id) {
		id, err = runXdotool(ctx, helper, "getwindowfocus")
		if err != nil || !isUsablePasteWindowID(id) {
			return "", platform.ErrNotFound
		}
	}
	properties, err := helper.Run(ctx, "xprop", []string{"-id", id, "WM_CLASS"}, nil)
	if err != nil {
		return "", err
	}
	if properties.ExitCode != 0 {
		return "", fmt.Errorf("xprop WM_CLASS exited with %d", properties.ExitCode)
	}
	class := x11.ParseProperties(string(properties.Stdout)).WMClass()
	if strings.TrimSpace(class) == "" || strings.EqualFold(class, "root") {
		return "", platform.ErrNotFound
	}
	return class, nil
}

// isUsablePasteWindowID excludes X11's root window, which xdotool reports as ID zero.
func isUsablePasteWindowID(id string) bool {
	return id != "0" && isWindowID(id)
}

// typeGesture reports that typing is unavailable on Linux: keyboard.type-text is macOS only, as in 1.x.
func typeGesture(context.Context, platform.Helper, string, string, time.Duration) error {
	return platform.ErrUnsupported
}
