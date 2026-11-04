package fake

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/update"
)

// Check is one scripted answer of Engine.Check.
type Check struct {
	Offer update.Offer
	Found bool
	Err   error
}

// Install is one scripted Engine.Install. It reports each step of Progress as (done, Total). Block makes the install
// wait for its context, as a download that a channel switch cancels does. Err is returned after the steps.
type Install struct {
	Progress []int64
	Total    int64
	Block    bool
	Err      error
}

// Engine is a scripted update.Engine. Checks answer in order and the last one repeats; installs run in order, and
// once they are used up an install succeeds. It records what the service asked of it.
type Engine struct {
	mu          sync.Mutex
	enabled     bool
	checks      []Check
	installs    []Install
	prereleases bool
	checkCount  int
	installCnt  int
	relaunches  int
}

var _ update.Engine = (*Engine)(nil)

// NewEngine returns an enabled engine with the scripted answers.
func NewEngine(checks []Check, installs ...Install) *Engine {
	return &Engine{enabled: true, checks: checks, installs: installs}
}

// Disabled returns an engine of a build that cannot update.
func Disabled() *Engine {
	return &Engine{}
}

// Enabled reports the build's capability.
func (e *Engine) Enabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enabled
}

// SetPrereleases records the channel's prerelease choice.
func (e *Engine) SetPrereleases(on bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prereleases = on
}

// Check counts the check and returns the next scripted answer.
func (e *Engine) Check(context.Context) (update.Offer, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checkCount++
	if len(e.checks) == 0 {
		return update.Offer{}, false, nil
	}
	answer := e.checks[0]
	if len(e.checks) > 1 {
		e.checks = e.checks[1:]
	}
	return answer.Offer, answer.Found, answer.Err
}

// Install runs the next scripted install.
func (e *Engine) Install(ctx context.Context, progress func(done, total int64)) error {
	e.mu.Lock()
	e.installCnt++
	script := Install{}
	if len(e.installs) > 0 {
		script = e.installs[0]
		e.installs = e.installs[1:]
	}
	e.mu.Unlock()
	for _, done := range script.Progress {
		progress(done, script.Total)
	}
	if script.Block {
		<-ctx.Done()
		return ctx.Err()
	}
	return script.Err
}

// Relaunch counts the restarts asked for.
func (e *Engine) Relaunch() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.relaunches++
}

// Prereleases reports the last SetPrereleases choice.
func (e *Engine) Prereleases() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.prereleases
}

// Checks reports how many checks ran.
func (e *Engine) Checks() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.checkCount
}

// Installs reports how many installs were started.
func (e *Engine) Installs() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.installCnt
}

// Relaunches reports how many restarts were asked for.
func (e *Engine) Relaunches() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.relaunches
}
