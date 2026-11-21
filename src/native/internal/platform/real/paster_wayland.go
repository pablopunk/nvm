package real

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// waylandFocusSettle is how long wtype waits before it presses the keys, so the compositor has handed the keyboard
// back to the app that had it before the palette hid. wtype sleeps inside its own command list (-s), so the delay
// is not a second process.
const waylandFocusSettle = 50 * time.Millisecond

// waylandKeyGesture sends chord ("ctrl+v") to the focused window with wtype. A Wayland client cannot read other
// windows, so the focused-app check uses the compositor's own query (Hyprland) and refuses to type when it cannot
// name the window.
func waylandKeyGesture(ctx context.Context, helper platform.Helper, expectedAppID, chord string) error {
	if expectedAppID != "" {
		focused, err := hyprlandActiveClass(ctx, helper)
		if err != nil {
			return err
		}
		if !strings.EqualFold(focused, expectedAppID) {
			return platform.ErrConflict
		}
	}
	args := wtypeChordArgs(chord, waylandFocusSettle)
	result, err := helper.Run(ctx, wtypeBinary, args, nil)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return platform.ErrUnsupported
		}
		return fmt.Errorf("wtype: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("wtype exited with %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

// wtypeChordArgs builds the wtype arguments for a chord such as "ctrl+v": sleep, hold each modifier, type the key,
// release the modifiers in reverse order. `-M ctrl v -m ctrl` is the form wtype's manual gives for a shortcut.
func wtypeChordArgs(chord string, settle time.Duration) []string {
	parts := strings.Split(chord, "+")
	key, modifiers := parts[len(parts)-1], parts[:len(parts)-1]
	args := []string{"-s", strconv.Itoa(int(settle.Milliseconds()))}
	for _, modifier := range modifiers {
		args = append(args, "-M", modifier)
	}
	args = append(args, key)
	for index := len(modifiers) - 1; index >= 0; index-- {
		args = append(args, "-m", modifiers[index])
	}
	return args
}

// hyprlandActiveClass names the focused window's class with hyprctl. Any other compositor, or no hyprctl, is
// platform.ErrUnsupported because Wayland offers clients no portable way to ask.
func hyprlandActiveClass(ctx context.Context, helper platform.Helper) (string, error) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" || !programInstalled("hyprctl") {
		return "", platform.ErrUnsupported
	}
	result, err := helper.Run(ctx, "hyprctl", []string{"activewindow", "-j"}, nil)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("hyprctl activewindow exited with %d", result.ExitCode)
	}
	var window struct {
		Class string `json:"class"`
	}
	if err := json.Unmarshal(result.Stdout, &window); err != nil || strings.TrimSpace(window.Class) == "" {
		return "", platform.ErrNotFound
	}
	return window.Class, nil
}
