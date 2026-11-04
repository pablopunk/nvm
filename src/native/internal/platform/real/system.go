package real

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// systemSettingsURLScheme opens the macOS settings app at a pane.
const systemSettingsURLScheme = "x-apple.systempreferences:"

// settingsPaneName accepts the bundle-style pane identifiers the macOS settings URL takes, and nothing that
// could change the command line or the URL's meaning.
var settingsPaneName = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)

// settingsAnchorName accepts the section names macOS settings anchors use.
var settingsAnchorName = regexp.MustCompile(`^[A-Za-z]+$`)

// system is the production platform.System. Power and session commands run through the Helper port, so a
// test with a fake helper sees the exact program and arguments of each OS. Settings that open a window on
// Linux start detached through the launch function, because they stay open until the user closes them.
type system struct {
	helper platform.Helper
	shell  platform.Shell
	goos   string
	home   string
	user   string
	chains map[systemAction][]systemCommand
	launch func(name string, args []string) error
}

var _ platform.System = (*system)(nil)

// newSystem builds the system port for one OS. home and user come from the process environment; an empty
// home disables Trash emptying, so no relative path is ever removed.
func newSystem(helper platform.Helper, shell platform.Shell, goos, home, user string) *system {
	return &system{
		helper: helper, shell: shell, goos: goos, home: home, user: user,
		chains: systemChains(goos, user), launch: startDetached,
	}
}

// LockScreen locks the session with the first lock program the OS has.
func (s *system) LockScreen() error {
	return s.runChain(s.chains[actionLock])
}

// Sleep puts the computer to sleep.
func (s *system) Sleep() error {
	return s.runChain(s.chains[actionSleep])
}

// Restart restarts the computer.
func (s *system) Restart() error {
	return s.runChain(s.chains[actionRestart])
}

// ShutDown powers the computer off.
func (s *system) ShutDown() error {
	return s.runChain(s.chains[actionShutDown])
}

// LogOut ends the user's session.
func (s *system) LogOut() error {
	return s.runChain(s.chains[actionLogOut])
}

// EmptyTrash deletes the Trash contents. Linux removes the freedesktop Trash folders itself; the other systems
// run their own empty-Trash command.
func (s *system) EmptyTrash() error {
	if s.goos == "linux" {
		return emptyFreedesktopTrash(s.home)
	}
	return s.runChain(s.chains[actionEmptyTrash])
}

// OpenSettings opens the system settings, at a pane where the OS can.
func (s *system) OpenSettings(pane, anchor string) error {
	switch s.goos {
	case "darwin":
		return s.shell.OpenURL(macSettingsURL(pane, anchor))
	case "windows":
		return s.shell.OpenURL("ms-settings:")
	case "linux":
		return s.startChain(linuxSettingsChain)
	}
	return platform.ErrUnsupported
}

// macSettingsURL builds the settings URL for a pane and anchor. An invalid pane opens the top level; an invalid
// anchor is dropped.
func macSettingsURL(pane, anchor string) string {
	if !settingsPaneName.MatchString(pane) {
		return systemSettingsURLScheme
	}
	if anchor != "" && settingsAnchorName.MatchString(anchor) {
		return systemSettingsURLScheme + pane + "?" + anchor
	}
	return systemSettingsURLScheme + pane
}

// runChain runs the commands in order until one exits with status 0. A command that is not installed is skipped;
// when no command is installed the intent is unsupported on this system.
func (s *system) runChain(chain []systemCommand) error {
	if len(chain) == 0 {
		return platform.ErrUnsupported
	}
	var failures []error
	for _, command := range chain {
		result, err := s.helper.Run(context.Background(), command.name, command.args, nil)
		if err == nil && result.ExitCode == 0 {
			return nil
		}
		if err == nil {
			err = fmt.Errorf("%s exited with status %d", command.name, result.ExitCode)
		}
		failures = append(failures, err)
	}
	if allNotInstalled(failures) {
		return platform.ErrUnsupported
	}
	return errors.Join(failures...)
}

// startChain starts the first program of the chain that the OS has, without waiting for it to exit.
func (s *system) startChain(chain []systemCommand) error {
	var failures []error
	for _, command := range chain {
		err := s.launch(command.name, command.args)
		if err == nil {
			return nil
		}
		failures = append(failures, err)
	}
	if allNotInstalled(failures) {
		return platform.ErrUnsupported
	}
	return errors.Join(failures...)
}

// allNotInstalled reports whether every failure says a program was not found.
func allNotInstalled(failures []error) bool {
	for _, err := range failures {
		if !errors.Is(err, exec.ErrNotFound) {
			return false
		}
	}
	return len(failures) > 0
}

// startDetached starts a program and reaps it in the background, so the app never waits on a window.
func startDetached(name string, args []string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// emptyFreedesktopTrash removes everything in the Trash folders under home. It refuses an empty home, so the
// relative folder is never emptied.
func emptyFreedesktopTrash(home string) error {
	if home == "" {
		return platform.ErrUnsupported
	}
	var failures []error
	for _, sub := range []string{"files", "info"} {
		if err := emptyFolder(filepath.Join(home, ".local", "share", "Trash", sub)); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
