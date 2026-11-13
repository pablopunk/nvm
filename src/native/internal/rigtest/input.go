//go:build rig

package rigtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const commandTimeout = 10 * time.Second

// xdo runs xdotool on the app's display and returns its combined output on failure.
func (a *App) xdo(args ...string) error {
	return a.run("xdotool", args...)
}

// run executes a helper tool on the app's display, bounded by commandTimeout.
func (a *App) run(name string, args ...string) error {
	_, err := a.output(name, args...)
	return err
}

// output is run that also returns the tool's stdout.
func (a *App) output(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = withDisplay(a.display)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// withDisplay is the current environment with DISPLAY set to display; exec keeps the last value.
func withDisplay(display string) []string {
	return append(os.Environ(), "DISPLAY="+display)
}

// PressHotkey sends one press of an accelerator such as "Alt+Shift+Space" through the X server,
// where the app's global grab sees it.
func (a *App) PressHotkey(accelerator string) error {
	keys, err := XdotoolKeys(accelerator)
	if err != nil {
		return err
	}
	return a.xdo("key", "--clearmodifiers", keys)
}

// Type types text into the focused window, one character at a time.
func (a *App) Type(text string) error {
	return a.xdo("type", "--clearmodifiers", "--delay", "40", "--", text)
}

// Key presses one named key such as "Escape", "Return" or "Down".
func (a *App) Key(name string) error {
	return a.xdo("key", "--clearmodifiers", name)
}

// Click moves the pointer to (x, y) on the display and clicks the left button there. It does not wait for the pointer
// to move (--sync): a pointer already at (x, y), as after an earlier click there, sends no motion, so the wait never ends.
func (a *App) Click(x, y int) error {
	return a.xdo("mousemove", fmt.Sprint(x), fmt.Sprint(y), "click", "1")
}

// Screenshot saves the whole display as a PNG in the test's artifacts and returns its path.
func (a *App) Screenshot(name string) (string, error) {
	path := a.artifactPath(name + "-desktop.png")
	return path, a.run("import", "-window", "root", path)
}

func (a *App) artifactPath(file string) string {
	return filepath.Join(a.artifacts, sanitizeName(file))
}
