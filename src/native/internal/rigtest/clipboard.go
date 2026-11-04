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

// concealedTarget is the X11 target a password manager offers with a copy (the KDE hint, which KeePassXC and
// similar tools advertise). The app treats a clipboard that offers it as concealed and never reads its text.
const concealedTarget = "x-kde-passwordManagerHint"

// ClipboardSet puts text on the CLIPBOARD selection with xclip. xclip forks a child to keep serving
// the selection, so the call returns once the parent exits. The child is detached (its own session,
// no inherited pipes) so this never holds the test's terminal or output.
func (a *App) ClipboardSet(text string) error {
	return a.clipboardServe(text, "-i")
}

// ClipboardSetConcealed puts text on the CLIPBOARD selection as a password manager copy: the only target the
// selection offers is the concealed hint, so the text is not plain text to any reader that ignores the hint.
func (a *App) ClipboardSetConcealed(text string) error {
	return a.clipboardServe(text, "-t", concealedTarget, "-i")
}

// clipboardServe runs xclip with args on the CLIPBOARD selection, feeding text on stdin.
func (a *App) clipboardServe(text string, args ...string) error {
	cmd := exec.Command("xclip", append([]string{"-selection", "clipboard", "-loops", clipboardServeLoops}, args...)...)
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
