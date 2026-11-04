// Package fake is a scripted updater for tests. Each Check answers with the next scripted state; once the script is
// used up, the last state repeats.
package fake

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/update"
)

// Updater answers checks from a script and counts them. It is safe for concurrent use.
//
//	u := fake.New("2.0.0", update.State{Status: update.StatusAvailable, LatestVersion: "2.1.0"})
type Updater struct {
	mu     sync.Mutex
	script []update.State
	last   update.State
	checks int
}

var _ update.Updater = (*Updater)(nil)

// New returns an updater at version that answers its checks with results in order.
func New(version string, results ...update.State) *Updater {
	return &Updater{script: results, last: update.State{Status: update.StatusIdle, CurrentVersion: version}}
}

// State returns the state of the last check, or idle before any check.
func (u *Updater) State() update.State {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.last
}

// Check counts the check and returns the next scripted state.
func (u *Updater) Check(context.Context) update.State {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.checks++
	if len(u.script) > 0 {
		u.last = u.script[0]
		u.script = u.script[1:]
	}
	return u.last
}

// Checks reports how many checks ran.
func (u *Updater) Checks() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.checks
}
