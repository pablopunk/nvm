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
	// Undecorated asks for the app's windows without compositor decorations.
	Undecorated bool `json:"undecorated"`
	// OwnsShortcuts records that the compositor's module runs the shortcuts, so the portal does not bind them too.
	OwnsShortcuts bool `json:"ownsShortcuts"`
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

// SetUndecorated keeps the appearance choice. It takes effect at the next apply.
func (s *Service) SetUndecorated(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.State()
	next.Undecorated = on
	return s.saveState(next)
}

// Preview prepares a change for the cached session, detecting first when nothing is cached.
func (s *Service) Preview(ctx context.Context, d hyprland.Desired) (hyprland.Plan, error) {
	session, ok := s.Session()
	if !ok {
		session = s.Detect(ctx)
	}
	return hyprland.Prepare(session, d)
}

// Apply writes a previewed plan and records whether the compositor now owns the shortcuts.
func (s *Service) Apply(ctx context.Context, plan hyprland.Plan, addInclude bool) (hyprland.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := hyprland.Apply(ctx, s.env, plan, addInclude)
	if err == nil {
		loaded := plan.IncludePresent || addInclude
		next := s.State()
		next.OwnsShortcuts = loaded && result.Loaded != hyprland.LoadedMissing
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
