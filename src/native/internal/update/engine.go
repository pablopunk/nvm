package update

import (
	"context"
	"errors"
	"time"
)

// Offer is a newer version the engine found: its version, its release notes in Markdown and when it was built.
type Offer struct {
	Version string
	Notes   string
	Date    time.Time
}

// ErrUpdateReady refuses a channel switch while a downloaded update waits for its restart. The engine has already
// replaced the app's files, so the switch waits until the new version runs.
var ErrUpdateReady = errors.New("restart to finish the update before switching channels")

// ErrNotReady refuses a restart when no update is downloaded.
var ErrNotReady = errors.New("no update is ready to install")

// Engine is the port to the signed updater that finds, verifies and installs releases. The real engine is MyGo's
// (package update/mygoengine); tests use update/fake. Check remembers the offer it found, and Install installs that
// offer. Install must verify the signature and the size of what it downloads before it replaces anything.
type Engine interface {
	// Enabled reports whether this build can update itself at all.
	Enabled() bool
	// SetPrereleases chooses whether Check offers prerelease versions.
	SetPrereleases(on bool)
	// Check asks the feed for a version newer than the running one. found is false when the app is up to date.
	Check(ctx context.Context) (offer Offer, found bool, err error)
	// Install downloads and verifies the offer of the last Check, then replaces the app with it. progress receives
	// the bytes downloaded so far and the total.
	Install(ctx context.Context, progress func(done, total int64)) error
	// Relaunch restarts the app, which runs the installed version.
	Relaunch()
}
