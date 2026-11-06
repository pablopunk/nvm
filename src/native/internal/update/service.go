package update

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ChannelStore keeps the chosen channel across launches. A nil store keeps stable for the process.
type ChannelStore interface {
	Channel() Channel
	SetChannel(Channel) error
}

// Options wire a Service. OnChange is called, off the caller's goroutine, after every change of State.
type Options struct {
	Engine   Engine
	Clock    platform.Clock
	Channels ChannelStore
	Current  string
	OnChange func(State)
}

// Service checks for updates, downloads them in the background and keeps the state the menu and palette show. It
// implements Updater. It never downgrades: an offer that is not newer than the running version is dropped.
type Service struct {
	engine   Engine
	clock    platform.Clock
	channels ChannelStore
	onChange func(State)

	mu       sync.Mutex
	state    State
	channel  Channel
	checking bool
	epoch    int
	cancel   context.CancelFunc
}

// NewService returns a service at the running version, on the channel the store keeps.
func NewService(o Options) *Service {
	channel := ChannelStable
	if o.Channels != nil {
		channel = o.Channels.Channel()
	}
	o.Engine.SetPrereleases(channel.Prereleases())
	return &Service{
		engine:   o.Engine,
		clock:    o.Clock,
		channels: o.Channels,
		onChange: o.OnChange,
		channel:  channel,
		state:    initialState(o.Engine, o.Current),
	}
}

// initialState is unsupported in a build that cannot update, so the page never waits for a check to say so.
func initialState(engine Engine, current string) State {
	if !engine.Enabled() {
		return unsupportedState(current)
	}
	return State{Status: StatusIdle, CurrentVersion: current}
}

// State returns the current state without contacting anything.
func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Channel returns the channel the checks follow.
func (s *Service) Channel() Channel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channel
}

// Check looks for a newer version. A newer one starts its download in the background and is reported as available
// at once; Check returns without waiting for the download. A check that cannot run because another check or a
// download is in progress returns the current state.
func (s *Service) Check(ctx context.Context) State {
	state, _ := s.CheckOwned(ctx)
	return state
}

// CheckOwned reports whether this caller started and completed the feed check. The state is still
// returned for callers that need the legacy snapshot behavior of Check.
func (s *Service) CheckOwned(ctx context.Context) (State, bool) {
	if !s.engine.Enabled() {
		return s.publish(unsupportedState(s.currentVersion())), true
	}
	epoch, started := s.beginCheck()
	if !started {
		return s.State(), false
	}
	offer, found, err := s.engine.Check(ctx)
	state, completed := s.finishCheck(ctx, epoch, offer, found, err)
	return state, completed
}

// RestartToUpdate restarts the app into the downloaded update. It refuses when no update is downloaded.
func (s *Service) RestartToUpdate() error {
	s.mu.Lock()
	if s.state.Status != StatusDownloaded {
		s.mu.Unlock()
		return ErrNotReady
	}
	s.state.Status = StatusInstalling
	state := s.state
	s.mu.Unlock()
	s.notify(state)
	s.engine.Relaunch()
	return nil
}

// SetChannel switches the channel that checks follow and persists it. It drops an update that is downloading, and
// refuses while a downloaded update waits for its restart. The caller checks again on the new channel.
func (s *Service) SetChannel(channel Channel) error {
	if !s.engine.Enabled() {
		return ErrChannelUnavailable
	}
	s.mu.Lock()
	if channel == s.channel {
		s.mu.Unlock()
		return nil
	}
	if s.state.Status == StatusDownloaded || s.state.Status == StatusInstalling {
		s.mu.Unlock()
		return ErrUpdateReady
	}
	s.channel = channel
	s.epoch++
	s.cancelDownloadLocked()
	s.state = State{Status: StatusIdle, CurrentVersion: s.state.CurrentVersion}
	state := s.state
	s.mu.Unlock()
	s.engine.SetPrereleases(channel.Prereleases())
	s.notify(state)
	if s.channels != nil {
		return s.channels.SetChannel(channel)
	}
	return nil
}

func (s *Service) currentVersion() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.CurrentVersion
}

// publish replaces the state and notifies the change.
func (s *Service) publish(state State) State {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
	s.notify(state)
	return state
}

func (s *Service) notify(state State) {
	if s.onChange != nil {
		s.onChange(state)
	}
}

// beginCheck marks a check as running. It refuses while a check, a download or a restart is in progress.
func (s *Service) beginCheck() (int, bool) {
	s.mu.Lock()
	if s.checking || s.busyLocked() {
		s.mu.Unlock()
		return 0, false
	}
	s.checking = true
	s.state.Status = StatusChecking
	state := s.state
	epoch := s.epoch
	s.mu.Unlock()
	s.notify(state)
	return epoch, true
}

func (s *Service) busyLocked() bool {
	switch s.state.Status {
	case StatusDownloading, StatusDownloaded, StatusInstalling:
		return true
	}
	return false
}

// finishCheck turns the engine's answer into the state. An answer that arrives after a channel switch is dropped.
func (s *Service) finishCheck(ctx context.Context, epoch int, offer Offer, found bool, err error) (State, bool) {
	s.mu.Lock()
	s.checking = false
	if epoch != s.epoch {
		s.mu.Unlock()
		return s.State(), false
	}
	state, download := s.answerLocked(offer, found, err)
	if download {
		state = s.startDownloadLocked(ctx, epoch, offer)
	}
	s.state = state
	s.mu.Unlock()
	s.notify(state)
	return state, true
}

// answerLocked is the state a check answer leads to: a failed check, up to date, or an available newer version. A
// version that is not newer than the running one is never offered.
func (s *Service) answerLocked(offer Offer, found bool, err error) (State, bool) {
	current := s.state.CurrentVersion
	switch {
	case err != nil:
		return State{Status: StatusFailed, CurrentVersion: current, Stage: StageCheck, Message: err.Error()}, false
	case !found || !IsNewer(offer.Version, current):
		return State{Status: StatusCurrent, CurrentVersion: current}, false
	}
	return State{Status: StatusAvailable, CurrentVersion: current, LatestVersion: offer.Version}, true
}
