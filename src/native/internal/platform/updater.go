package platform

import (
	"context"
	"time"
)

// Release is a newer version of the app that a SelfUpdater found: its version, its release notes in Markdown and when
// it was built.
type Release struct {
	Version string
	Notes   string
	Date    time.Time
}

// SelfUpdater is the intent "find, verify and install a signed release of this app, then restart into it". The app's
// framework implements it with its signed updater, which refuses an archive or delta that is not signed with the
// app's key and checks its size before anything is replaced. A build that cannot update reports Enabled false and
// contacts nothing. Check remembers the release it found, and Install installs that one.
type SelfUpdater interface {
	// Enabled reports whether this build can update itself at all.
	Enabled() bool
	// SetPrereleases chooses whether Check offers prerelease versions.
	SetPrereleases(on bool)
	// Check asks the feed for a version newer than the running one. found is false when the app is up to date.
	Check(ctx context.Context) (release Release, found bool, err error)
	// Install downloads and verifies the release of the last Check, then replaces the app with it. progress receives
	// the bytes downloaded so far and the total.
	Install(ctx context.Context, progress func(done, total int64)) error
	// Relaunch restarts the app, which runs the installed version.
	Relaunch()
}
