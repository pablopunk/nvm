package compositor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
)

// State is what Nevermind keeps about the compositor between runs, in a machine-local file.
type State struct {
	// OwnsShortcuts records that the compositor's module runs the shortcuts, so the portal does not bind them too.
	OwnsShortcuts bool `json:"ownsShortcuts"`
	// Disabled records that the user removed the config; Sync leaves Hyprland alone until Enable.
	Disabled bool `json:"disabled"`
}

// Report is the outcome of the last lifecycle step, for the palette to show.
type Report struct {
	At     time.Time
	Step   string
	Result hyprland.Result
	Err    error
}

// Service is the Hyprland adapter with Nevermind's state. Its lifecycle methods serialize and run off the UI thread;
// its readers return cached values at once.
type Service struct {
	env       hyprland.Env
	statePath string
	now       func() time.Time
	mu        sync.Mutex
	syncMu    sync.Mutex
	session   atomic.Pointer[hyprland.Session]
	state     atomic.Pointer[State]
	last      atomic.Pointer[Report]
}

// New returns a service that keeps its state in statePath and its undo record in env.UndoFile.
func New(env hyprland.Env, statePath string, now func() time.Time) *Service {
	s := &Service{env: env, statePath: statePath, now: now}
	s.state.Store(readState(statePath))
	return s
}

// Present reports, from the environment alone, whether a Hyprland session is running, so the palette can offer the
// page without probing.
func (s *Service) Present() bool { return s.env.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" }

// Session is the last detection, and false before the first one finished.
func (s *Service) Session() (hyprland.Session, bool) {
	if p := s.session.Load(); p != nil {
		return *p, true
	}
	return hyprland.Session{}, false
}

// Detect probes the compositor again and caches the result.
func (s *Service) Detect(ctx context.Context) hyprland.Session {
	session := hyprland.Detect(ctx, s.env)
	s.session.Store(&session)
	return session
}

// State is the kept state.
func (s *Service) State() State { return *s.state.Load() }

// Last is the outcome of the last lifecycle step, or nil.
func (s *Service) Last() *Report { return s.last.Load() }

// Enable lets Sync manage the Hyprland config again after Remove.
func (s *Service) Enable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.State()
	next.Disabled = false
	return s.saveState(next)
}

// Preview prepares a change for the cached session, detecting first when nothing is cached. Shortcuts the user's own
// binds already use are left out and listed as skipped.
func (s *Service) Preview(ctx context.Context, d hyprland.Desired) (hyprland.Plan, error) {
	session, ok := s.Session()
	if !ok {
		session = s.Detect(ctx)
	}
	return hyprland.PrepareAgainstBinds(ctx, s.env, session, d)
}

// SyncOutcome is what Sync did.
type SyncOutcome struct {
	// Wrote says the module was rewritten and Hyprland reloaded.
	Wrote bool
	// OwnershipChanged says the compositor started or stopped running the shortcuts, so the portal's must be rebound.
	OwnershipChanged bool
	Result           hyprland.Result
	Skipped          []hyprland.Skipped
}

// Sync keeps the module in line with d without being asked: it adds the include to the entry file the first time,
// writes the module and reloads Hyprland. It writes nothing when the module already holds d. It does nothing after
// Remove, or where the config cannot be written (a session that is not ready). A module that is in place and current
// still marks the compositor as the owner of the shortcuts. Syncs run one at a time, each against the module the previous one wrote.
func (s *Service) Sync(ctx context.Context, d hyprland.Desired) (SyncOutcome, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if s.State().Disabled {
		return SyncOutcome{}, nil
	}
	plan, err := s.Preview(ctx, d)
	if session, ok := s.Session(); ok && !session.Ready() {
		return SyncOutcome{}, nil
	}
	if err != nil {
		return SyncOutcome{}, err
	}
	owned := s.State().OwnsShortcuts
	if plan.ModuleUnchanged() && plan.IncludePresent {
		return SyncOutcome{Skipped: plan.Skipped, OwnershipChanged: s.adopt() && !owned}, nil
	}
	result, err := s.Apply(ctx, plan, true)
	if err != nil {
		return SyncOutcome{Skipped: plan.Skipped}, err
	}
	return SyncOutcome{Wrote: result.Changed, Result: result, Skipped: plan.Skipped, OwnershipChanged: !owned && s.State().OwnsShortcuts}, nil
}

// adopt records that the compositor runs the shortcuts, for a module that is already in place. It reports success.
func (s *Service) adopt() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.State()
	next.OwnsShortcuts = true
	return s.saveState(next) == nil
}

// Apply writes a previewed plan and records whether the compositor now owns the shortcuts.
func (s *Service) Apply(ctx context.Context, plan hyprland.Plan, addInclude bool) (hyprland.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := hyprland.Apply(ctx, s.env, plan, addInclude)
	if err == nil {
		loaded := plan.IncludePresent || addInclude
		next := s.State()
		// The compositor's own bind list is a check that can lag behind a reload, so a "missing" answer does not take
		// the shortcuts away from a module that is in place; the caller reports it.
		next.OwnsShortcuts = loaded
		err = s.saveState(next)
	}
	s.record("apply", result, err)
	return result, err
}

// Undo restores the module as it was before the last apply. The compositor stops owning the shortcuts, since the
// restored module may not hold them; the next apply decides again.
func (s *Service) Undo(ctx context.Context) (hyprland.Result, error) {
	return s.withdraw(ctx, "undo", hyprland.Undo)
}

// Remove takes the module and the include away.
func (s *Service) Remove(ctx context.Context) (hyprland.Result, error) {
	return s.withdraw(ctx, "remove", hyprland.Remove)
}

func (s *Service) withdraw(ctx context.Context, step string, fn func(context.Context, hyprland.Env, hyprland.Session) (hyprland.Result, error)) (hyprland.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.Session()
	if !ok {
		session = s.Detect(ctx)
	}
	result, err := fn(ctx, s.env, session)
	if err == nil || result.Changed {
		next := s.State()
		next.OwnsShortcuts = false
		next.Disabled = next.Disabled || step == "remove"
		err = errors.Join(err, s.saveState(next))
	}
	s.record(step, result, err)
	return result, err
}

func (s *Service) record(step string, result hyprland.Result, err error) {
	s.last.Store(&Report{At: s.now(), Step: step, Result: result, Err: err})
}

func readState(path string) *State {
	state := &State{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, state)
	}
	return state
}

func (s *Service) saveState(next State) error {
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0o700); err != nil {
		return err
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.statePath); err != nil {
		return err
	}
	s.state.Store(&next)
	return nil
}
