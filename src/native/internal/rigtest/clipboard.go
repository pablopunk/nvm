//go:build rig

package rigtest

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	clipboardSetTimeout = 5 * time.Second
	// xclip serves the selection from a forked child; -loops bounds how long that child lives.
	clipboardServeLoops = "200"
)

// ClipboardSet puts text on the CLIPBOARD selection with xclip. xclip forks a child to keep serving
// the selection, so the call returns once the parent exits. The child is detached (its own session,
// no inherited pipes) so this never holds the test's terminal or output.
func (a *App) ClipboardSet(text string) error {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-loops", clipboardServeLoops, "-i")
	cmd.Env = withDisplay(a.display)
	cmd.Stdin = strings.NewReader(text)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("xclip: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("xclip -i: %w", err)
		}
		return nil
	case <-time.After(clipboardSetTimeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("xclip -i did not return within %s", clipboardSetTimeout)
	}
}

// ClipboardGet returns the CLIPBOARD selection as text.
func (a *App) ClipboardGet() (string, error) {
	out, err := a.output("xclip", "-selection", "clipboard", "-o")
	if err != nil {
		return "", err
	}
	return out, nil
}
