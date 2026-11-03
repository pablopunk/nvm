package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by Shell.FailNext.
const (
	OpShellOpenPath  = "OpenPath"
	OpShellOpenURL   = "OpenURL"
	OpShellReveal    = "Reveal"
	OpShellTrash     = "Trash"
	OpShellApps      = "InstalledApps"
	OpShellAppIcon   = "AppIcon"
	OpShellLaunchApp = "LaunchApp"
)

// ShellCall is one recorded call on the shell. Arg is the path, URL or app path the call named; Beep records
// an empty Arg.
type ShellCall struct {
	Op  string
	Arg string
}

// Shell is a platform.Shell that records what it was asked to do and answers app queries from fixtures.
// Icons are keyed by path; AppIcon returns the stored icon regardless of the requested size and returns
// ErrNotFound for unknown paths.
//
//	shell := fake.NewShell()
//	shell.SetApps(fixtures.Apps(3))
//	shell.Calls() // every OpenPath, Reveal, Trash... in order
type Shell struct {
	mu     sync.Mutex
	apps   []platform.App
	icons  map[string][]byte
	calls  []ShellCall
	faults faultSet
}

var _ platform.Shell = (*Shell)(nil)

// NewShell returns a shell with no apps and no icons.
func NewShell() *Shell {
	return &Shell{icons: map[string][]byte{}}
}

// FailNext makes the next call of op return err. Use the Op constants of this file.
func (s *Shell) FailNext(op string, err error) {
	s.faults.failNext(op, err)
}

// SetApps replaces the installed apps and the icons of files and apps by path.
func (s *Shell) SetApps(apps []platform.App, icons map[string][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apps = slices.Clone(apps)
	s.icons = map[string][]byte{}
	for path, icon := range icons {
		s.icons[path] = slices.Clone(icon)
	}
}

// Calls returns the recorded calls in order.
func (s *Shell) Calls() []ShellCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// OpenPath records the call.
func (s *Shell) OpenPath(path string) error {
	return s.record(OpShellOpenPath, path)
}

// OpenURL records the call.
func (s *Shell) OpenURL(url string) error {
	return s.record(OpShellOpenURL, url)
}

// Reveal records the call.
func (s *Shell) Reveal(path string) error {
	return s.record(OpShellReveal, path)
}

// Trash records the call; the fake does not remove anything, so tests pair it with FS when they need that.
func (s *Shell) Trash(path string) error {
	return s.record(OpShellTrash, path)
}

// InstalledApps returns a copy of the fixture apps.
func (s *Shell) InstalledApps() ([]platform.App, error) {
	if err := s.faults.take(OpShellApps); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.apps), nil
}

// AppIcon returns the stored icon for path, or ErrNotFound.
func (s *Shell) AppIcon(path string, size int) ([]byte, error) {
	if err := s.faults.take(OpShellAppIcon); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	icon, ok := s.icons[path]
	if !ok {
		return nil, platform.ErrNotFound
	}
	return slices.Clone(icon), nil
}

// LaunchApp records the launch by the app's path.
func (s *Shell) LaunchApp(app platform.App) error {
	return s.record(OpShellLaunchApp, app.Path)
}

// Beep records the call.
func (s *Shell) Beep() {
	_ = s.record("Beep", "")
}

// record appends a call unless a queued fault for op applies, in which case the fault is returned and the
// call is not recorded, as a failed action leaves no trace.
func (s *Shell) record(op, arg string) error {
	if err := s.faults.take(op); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, ShellCall{Op: op, Arg: arg})
	return nil
}
