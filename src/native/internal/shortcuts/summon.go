package shortcuts

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// A relaunched app can find its shortcut still held by the previous instance for a moment, so the configured
// accelerator is retried this many times, this far apart, before the status settles.
const (
	summonRetries    = 8
	summonRetryEvery = 400 * time.Millisecond
)

// Status is what the palette shows of its summon shortcut: the configured accelerator, whether it is bound,
// the recovery accelerator bound in its place, and whether the configured one is still being retried.
type Status struct {
	Accelerator string
	Registered  bool
	Recovery    string
	Retrying    bool
}

// SummonOptions are what the registrar needs: the OS shortcuts, the clock that spaces the retries, the
// platform's name (it selects the recovery list), the action the shortcut runs and a hook for status changes.
type SummonOptions struct {
	Hotkeys  platform.Hotkeys
	Clock    platform.Clock
	GOOS     string
	Toggle   func()
	OnChange func(Status)
}

// Unavailable reports whether a configured accelerator is unbound with no retry pending: the user has to act.
func (s Status) Unavailable() bool {
	return s.Accelerator != "" && !s.Registered && !s.Retrying
}

// Summon owns the palette's own summon shortcut. It is the only code that binds it, so the palette's status
// and the bound shortcut cannot disagree.
type Summon struct {
	opts SummonOptions

	mu         sync.Mutex
	status     Status
	generation int
}

// NewSummon returns a registrar that has bound nothing yet; Start binds the configured accelerator.
func NewSummon(opts SummonOptions) *Summon {
	return &Summon{opts: opts}
}

// Status returns the current status.
func (s *Summon) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Start binds accelerator (empty when none is set) and returns the status it reached at once. A conflict binds
// the first free recovery accelerator at once and retries the configured one in the background.
func (s *Summon) Start(accelerator string) Status {
	s.mu.Lock()
	s.generation++
	generation := s.generation
	s.mu.Unlock()
	if accelerator == "" {
		s.publish(Status{})
		return s.Status()
	}
	err := s.bind(accelerator)
	if err == nil {
		s.publish(Status{Accelerator: accelerator, Registered: true})
		return s.Status()
	}
	recovery := s.bindRecovery(accelerator)
	s.publish(Status{Accelerator: accelerator, Recovery: recovery, Retrying: retryable(err)})
	if retryable(err) {
		go s.retry(generation, accelerator)
	}
	return s.Status()
}

// Suspend releases the shortcut and stops its retries while a recorder takes the keys. The status is kept, so the
// next Start binds the configured accelerator again.
func (s *Summon) Suspend() {
	s.mu.Lock()
	s.generation++
	status := s.status
	s.mu.Unlock()
	s.release(status)
}

// Change moves the shortcut to accelerator (empty to set none). It is all or nothing: on failure the previous
// accelerator is bound again and the status does not change, and the error wraps platform.ErrConflict.
func (s *Summon) Change(accelerator string) error {
	s.mu.Lock()
	s.generation++
	previous := s.status
	s.mu.Unlock()
	s.release(previous)
	if accelerator == "" {
		s.publish(Status{})
		return nil
	}
	if err := s.bind(accelerator); err != nil {
		s.restore(previous)
		return fmt.Errorf("%w: %s", platform.ErrConflict, accelerator)
	}
	s.publish(Status{Accelerator: accelerator, Registered: true})
	return nil
}

// retry binds the configured accelerator as the clock passes each interval, until it binds or the attempts
// run out. A Start or Change that began after it makes it stop.
func (s *Summon) retry(generation int, accelerator string) {
	for range summonRetries {
		<-s.opts.Clock.After(summonRetryEvery)
		if !s.isCurrent(generation) {
			return
		}
		if s.bind(accelerator) == nil {
			s.promote(generation, accelerator)
			return
		}
	}
	s.settleRetries(generation)
}

// promote replaces the recovery accelerator with the configured one once the retry binds it.
func (s *Summon) promote(generation int, accelerator string) {
	s.mu.Lock()
	if generation != s.generation {
		s.mu.Unlock()
		s.opts.Hotkeys.Unregister(accelerator)
		return
	}
	recovery := s.status.Recovery
	s.mu.Unlock()
	if recovery != "" {
		s.opts.Hotkeys.Unregister(recovery)
	}
	s.publishIfCurrent(generation, Status{Accelerator: accelerator, Registered: true})
}

// settleRetries ends the retrying state after the last attempt failed.
func (s *Summon) settleRetries(generation int) {
	s.mu.Lock()
	status := s.status
	s.mu.Unlock()
	status.Retrying = false
	s.publishIfCurrent(generation, status)
}

func (s *Summon) isCurrent(generation int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return generation == s.generation
}

// bind registers accelerator with the toggle. It is a single place so the platform error is classified once.
func (s *Summon) bind(accelerator string) error {
	return s.opts.Hotkeys.Register(accelerator, s.opts.Toggle)
}

// bindRecovery binds the first recovery accelerator that is free and is not the configured one, and returns it.
func (s *Summon) bindRecovery(configured string) string {
	for _, candidate := range recoveryAccelerators(s.opts.GOOS) {
		if candidate == configured {
			continue
		}
		if s.bind(candidate) == nil {
			return candidate
		}
	}
	return ""
}

// release unbinds what a status holds: the configured accelerator and the recovery one.
func (s *Summon) release(status Status) {
	if status.Registered {
		s.opts.Hotkeys.Unregister(status.Accelerator)
	}
	if status.Recovery != "" {
		s.opts.Hotkeys.Unregister(status.Recovery)
	}
}

// restore binds what a status held again after a Change failed. A retry that was running has been stopped by
// the Change, so the restored status is not retrying.
func (s *Summon) restore(previous Status) {
	if previous.Accelerator == "" {
		return
	}
	if s.bind(previous.Accelerator) == nil {
		s.publish(Status{Accelerator: previous.Accelerator, Registered: true})
		return
	}
	s.publish(Status{Accelerator: previous.Accelerator, Recovery: s.bindRecovery(previous.Accelerator)})
}

func (s *Summon) publish(status Status) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
	s.notify(status)
}

func (s *Summon) publishIfCurrent(generation int, status Status) {
	s.mu.Lock()
	if generation != s.generation {
		s.mu.Unlock()
		return
	}
	s.status = status
	s.mu.Unlock()
	s.notify(status)
}

func (s *Summon) notify(status Status) {
	if s.opts.OnChange != nil {
		s.opts.OnChange(status)
	}
}

// retryable reports whether a failed bind can succeed later. An unsupported session cannot.
func retryable(err error) bool {
	return err != nil && !errors.Is(err, platform.ErrUnsupported)
}
