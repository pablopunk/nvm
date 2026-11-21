package extjobs

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

// FrontmostPollJobID is the host job that watches the app in front. It exists only while an enabled extension command
// declares app.frontmost.changed, so nothing polls the desktop for an extension that does not listen.
const (
	FrontmostPollJobID   = "frontmost.poll"
	frontmostPollEvery   = 5 * time.Second
	frontmostPollTimeout = 3 * time.Second
)

// Frontmost names the app in front, by bundle ID, then name, or "" when no app is. A nil Frontmost in Config means the
// platform cannot tell, and no poll is registered.
type Frontmost func(ctx context.Context) (string, error)

// frontmostPollDefinition is the poll job: every few seconds it reads the front app and emits the change event when the
// app differs from the last one it saw.
func (a *Adapter) frontmostPollDefinition() jobs.Definition {
	return jobs.Definition{
		ID:       FrontmostPollJobID,
		Title:    "Frontmost App Poll",
		Owner:    jobs.OwnerHost,
		Scope:    "apps",
		Triggers: []jobs.Trigger{{Kind: jobs.TriggerInterval, Every: frontmostPollEvery, Delay: frontmostPollEvery}},
		Budget:   jobs.Budget{Timeout: frontmostPollTimeout},
		Run: func(ctx context.Context, _ jobs.Run) error {
			return a.pollFrontmost(ctx)
		},
	}
}

// pollFrontmost emits app.frontmost.changed when the front app differs from the last one seen. A failed read is not a
// failure of the job: the platform may be busy, and the next poll asks again.
func (a *Adapter) pollFrontmost(ctx context.Context) error {
	current, err := a.cfg.Frontmost(ctx)
	if err != nil || current == "" {
		return nil
	}
	a.mu.Lock()
	changed := current != a.frontmostLast
	a.frontmostLast = current
	a.mu.Unlock()
	if changed {
		a.cfg.Registry.Emit(FrontmostChangedEvent, nil)
	}
	return nil
}

// syncFrontmostPoll registers the poll while a subscriber exists and removes it when none does. The adapter's sync job
// is the only caller, so the poll is added and removed in step with the subscribers.
func (a *Adapter) syncFrontmostPoll(wanted bool) {
	if a.cfg.Frontmost == nil {
		return
	}
	a.mu.Lock()
	running := a.frontmostPolling
	a.frontmostPolling = wanted
	if !wanted {
		a.frontmostLast = ""
	}
	a.mu.Unlock()
	switch {
	case wanted && !running:
		if err := a.cfg.Registry.Register(a.frontmostPollDefinition()); err != nil {
			a.logWarn("extension.frontmost.register.failed", FrontmostPollJobID)
		}
	case !wanted && running:
		a.cfg.Registry.Unregister(FrontmostPollJobID)
	}
}

// declaresFrontmost reports whether a command runs on app.frontmost.changed.
func declaresFrontmost(command api.Command) bool {
	for _, trigger := range command.Triggers {
		if trigger.Type == api.TriggerAppFrontmostChange {
			return true
		}
	}
	return false
}
