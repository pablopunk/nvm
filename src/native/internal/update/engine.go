package update

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Offer is a newer version the engine found.
type Offer = platform.Release

// Engine is the port to the signed updater that finds, verifies and installs releases. It is the platform's
// SelfUpdater: MyGo's signed updater in the app (platform/real), a scripted fake in tests (update/fake).
type Engine = platform.SelfUpdater

// ErrUpdateReady refuses a channel switch while a downloaded update waits for its restart. The engine has already
// replaced the app's files, so the switch waits until the new version runs.
var ErrUpdateReady = errors.New("restart to finish the update before switching channels")

// ErrChannelUnavailable refuses a channel switch in a build that cannot update itself, and says why.
var ErrChannelUnavailable = errors.New("update channels are not available in this build yet")

// ErrNotReady refuses a restart when no update is downloaded.
var ErrNotReady = errors.New("no update is ready to install")
