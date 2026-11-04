// Package mygoengine is the update.Engine of the app: MyGo's signed updater. MyGo reads the feed and the public key
// that `mygo build` stamped into the app (mygo.json, updates), verifies the Ed25519 signature and the size of every
// download, and only then replaces the app. This package adds nothing to that check and never skips it.
//
// Layer: service tier, a subpackage of update. It imports update and MyGo.
package mygoengine

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/update"
)

// Engine finds and installs releases through MyGo. It keeps the update Check found, because MyGo installs that one.
type Engine struct {
	mu   sync.Mutex
	last *mygo.Update
}

var _ update.Engine = (*Engine)(nil)

// New returns the engine of this process.
func New() *Engine {
	return &Engine{}
}

// Enabled reports whether this build was stamped with a feed and a public key, and can write where it is installed.
func (e *Engine) Enabled() bool {
	return mygo.Updater.Enabled()
}

// SetPrereleases makes the checks offer prereleases (the nightly channel) or not.
func (e *Engine) SetPrereleases(on bool) {
	mygo.Updater.SetPrereleases(on)
}

// Check asks the feed. A newer version is remembered for Install; an up-to-date app forgets the last one.
func (e *Engine) Check(ctx context.Context) (update.Offer, bool, error) {
	found, err := mygo.Updater.Check(ctx)
	if err != nil {
		e.remember(nil)
		return update.Offer{}, false, errors.New(friendly(err))
	}
	e.remember(found)
	if found == nil {
		return update.Offer{}, false, nil
	}
	return update.Offer{Version: found.Version, Notes: found.Notes, Date: found.Date}, true, nil
}

// Install downloads, verifies and stages the update that Check found. MyGo refuses an archive or delta that is not
// signed with the app's key; the error it returns is what the person reads.
func (e *Engine) Install(ctx context.Context, progress func(done, total int64)) error {
	e.mu.Lock()
	last := e.last
	e.mu.Unlock()
	if last == nil {
		return errors.New("no update was checked")
	}
	if err := last.Install(ctx, progress); err != nil {
		return errors.New(friendly(err))
	}
	return nil
}

// Relaunch quits the app and starts it again, which runs the installed version.
func (e *Engine) Relaunch() {
	mygo.App.Relaunch()
}

func (e *Engine) remember(found *mygo.Update) {
	e.mu.Lock()
	e.last = found
	e.mu.Unlock()
}

// friendly drops the package prefix MyGo puts on its errors, so the message fits the palette's copy.
func friendly(err error) string {
	return strings.TrimPrefix(err.Error(), "mygo: ")
}
