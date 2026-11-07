package apps

import (
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Quit timing of a run: the app gets QuitTimeout to close after a quit request, polled every QuitPoll.
const (
	defaultQuitTimeout = 10 * time.Second
	defaultQuitPoll    = 250 * time.Millisecond
)

// UninstallDeps are the ports and facts the uninstall flow works with. Nothing here runs an OS command itself: the
// composition root supplies the Trash, the process checks and the quit request, so the guards are tested with
// fakes and never touch a real application.
type UninstallDeps struct {
	// Platform is the GOOS the rules follow: "darwin" and "linux" have uninstall; others refuse.
	Platform string
	// Home is the user's home folder, the root of every per-user location.
	Home string
	// UID is the current user. Associated data owned by another user is never offered.
	UID             int
	Files           Files
	DirectoryAccess platform.DirectoryAccess
	Clock           platform.Clock
	// Trash moves one item to the Trash (the Shell port). It never deletes permanently.
	Trash func(path string) error
	// ReadBundleID returns an app's bundle identifier (macOS plist) or desktop identifier (Linux file name).
	ReadBundleID func(appPath string) (string, error)
	// FindApps lists the app bundles with a bundle identifier, from Spotlight. Nil scans the application folders.
	FindApps func(bundleID string) ([]string, error)
	// IsRunning reports whether the app has a live process. An error counts as running.
	IsRunning func(appPath string) (bool, error)
	// RequestQuit asks the app to quit. It does not wait.
	RequestQuit func(appPath string) error
	// NevermindAppPath is this copy of the app, which can never be uninstalled from itself.
	NevermindAppPath  string
	NevermindBundleID string
	QuitTimeout       time.Duration
	QuitPoll          time.Duration
}

// Uninstaller discovers an app's related items and moves the reviewed selection to the Trash.
type Uninstaller struct {
	deps UninstallDeps
	seq  atomic.Uint64
}

// NewUninstaller returns an uninstaller with the default quit timing.
func NewUninstaller(deps UninstallDeps) *Uninstaller {
	if deps.Platform == "" {
		deps.Platform = runtime.GOOS
	}
	if deps.QuitTimeout <= 0 {
		deps.QuitTimeout = defaultQuitTimeout
	}
	if deps.QuitPoll <= 0 {
		deps.QuitPoll = defaultQuitPoll
	}
	if deps.NevermindBundleID == "" {
		deps.NevermindBundleID = NeverMindBundleID
	}
	return &Uninstaller{deps: deps}
}

// Discover checks the app and lists its items for review. It does no work on platforms without uninstall.
func (u *Uninstaller) Discover(appPath string) Discovery {
	switch u.deps.Platform {
	case "darwin":
		return u.discoverDarwin(appPath)
	case "linux":
		return u.discoverLinux(appPath)
	default:
		return Discovery{ReasonCode: "unsupported-platform", Message: "Uninstall is not available on this system yet"}
	}
}

// Selected returns the candidates of the snapshot whose IDs are set to true in values. Values from outside the
// snapshot are ignored: only IDs the review produced can select anything.
func Selected(snapshot UninstallSnapshot, values map[string]bool) []Candidate {
	var out []Candidate
	for _, candidate := range snapshot.Candidates {
		if values[candidate.ID] {
			out = append(out, candidate)
		}
	}
	return out
}

// nextID names a candidate within this process. IDs only need to be unique among the candidates of one review.
func (u *Uninstaller) nextID() string {
	return "candidate-" + strconv.FormatUint(u.seq.Add(1), 10)
}
