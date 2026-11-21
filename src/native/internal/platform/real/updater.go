package real

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// selfUpdater is MyGo's signed updater. MyGo reads the feed and the public key that `mygo build` stamped into the app
// (mygo.json, updates), verifies the Ed25519 signature and the size of every download, and only then replaces the app.
// This adapter adds nothing to that check and never skips it.
type selfUpdater struct {
	mu   sync.Mutex
	last *mygo.Update
}

// NewSelfUpdater returns the app's updater over MyGo.
func NewSelfUpdater() platform.SelfUpdater {
	return &selfUpdater{}
}

func (u *selfUpdater) Enabled() bool {
	return mygo.Updater.Enabled()
}

// ManagedByPackageManager: a Linux release build that MyGo cannot update was installed where only the package
// manager writes, as the .deb installs it.
func (u *selfUpdater) ManagedByPackageManager() bool {
	return runtime.GOOS == "linux" && updateRepository != "" && !mygo.Updater.Enabled()
}

// SetChannel points the checks at the channel's releases. Stable reads MyGo's feed, the repository's latest release;
// Nightly and Dev read the release pointers (channelSource), so a nightly never offers a dev build.
func (u *selfUpdater) SetChannel(channel string) {
	source := newChannelSource(channel)
	if source == nil {
		mygo.Updater.SetSource(nil)
		mygo.Updater.SetPrereleases(channel == updateChannelNightly || channel == updateChannelDev)
		return
	}
	mygo.Updater.SetPrereleases(false)
	mygo.Updater.SetSource(source)
}

func (u *selfUpdater) Check(ctx context.Context) (platform.Release, bool, error) {
	found, err := mygo.Updater.Check(ctx)
	if err != nil {
		u.remember(nil)
		return platform.Release{}, false, errors.New(friendlyMyGoError(err))
	}
	u.remember(found)
	if found == nil {
		return platform.Release{}, false, nil
	}
	return platform.Release{Version: found.Version, Notes: found.Notes, Date: found.Date}, true, nil
}

func (u *selfUpdater) Install(ctx context.Context, progress func(done, total int64)) error {
	u.mu.Lock()
	last := u.last
	u.mu.Unlock()
	if last == nil {
		return errors.New("no update was checked")
	}
	if err := last.Install(ctx, progress); err != nil {
		return errors.New(friendlyMyGoError(err))
	}
	return nil
}

func (u *selfUpdater) Relaunch() {
	mygo.App.Relaunch()
}

func (u *selfUpdater) remember(found *mygo.Update) {
	u.mu.Lock()
	u.last = found
	u.mu.Unlock()
}

// friendlyMyGoError drops the package prefix MyGo puts on its errors, so the message fits the palette's copy.
func friendlyMyGoError(err error) string {
	return strings.TrimPrefix(err.Error(), "mygo: ")
}
