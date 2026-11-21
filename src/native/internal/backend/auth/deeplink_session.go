package auth

import (
	"context"
	"errors"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Deep-link exchange timing. The approval page sends the user back before the code expires, so the exchange polls
// for as long as the device code lives, and backs off by the device flow's step when the backend asks.
const (
	consumeWindow       = 5 * time.Minute
	consumePollEvery    = 2 * time.Second
	consumeSlowDownStep = 5 * time.Second
)

// Attach claims the sign-in scheme with the OS and routes its URLs to HandleURL. The returned function stops the
// routing.
func (s *Session) Attach(links platform.DeepLinks) (detach func(), err error) {
	if err := links.Register(Scheme); err != nil {
		return nil, err
	}
	return links.OnOpen(s.HandleURL), nil
}

// HandleURL handles a URL the OS delivered. Anything that is not a sign-in link is ignored.
func (s *Session) HandleURL(rawURL string) {
	s.mu.Lock()
	active := s.env.BaseURL
	s.mu.Unlock()
	link, err := ParseDeepLink(rawURL, active)
	if errors.Is(err, ErrNotAuthLink) {
		return
	}
	if err != nil {
		s.deps.Logger.Warn("auth.deeplink_rejected", logx.F{"reason": err.Error()})
		return
	}
	s.consumeLink(link)
}

// consumeLink exchanges the code of a sign-in link. It does nothing while a sign-in runs or when the active backend
// is already connected, and switches to the link's origin first when that origin differs.
func (s *Session) consumeLink(link DeepLink) {
	s.mu.Lock()
	running := s.sign.Phase == PhaseStarting || s.sign.Phase == PhasePending
	active := s.env.BaseURL
	s.mu.Unlock()
	if running {
		s.deps.Logger.Info("auth.deeplink_signin_running", nil)
		return
	}
	if link.BaseURL != active {
		if err := s.SwitchEnvironment(environmentKindFor(link.BaseURL), link.BaseURL); err != nil {
			s.deps.Logger.Warn("auth.deeplink_environment_rejected", logx.F{"error": err.Error()})
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.connected {
		s.mu.Unlock()
		cancel()
		s.deps.Logger.Info("auth.deeplink_already_connected", nil)
		return
	}
	if !s.beginFlowLocked(cancel) {
		s.mu.Unlock()
		cancel()
		return
	}
	s.sign.Phase = PhasePending
	client := s.client
	s.mu.Unlock()
	s.notifyChange()
	safego.Go("auth.deeplink", func(context.Context) { s.runConsume(ctx, client, link.Code) })
}

// runConsume exchanges the code and records how it ended, like runSignIn does for the device flow.
func (s *Session) runConsume(ctx context.Context, client *backend.Client, code string) {
	approval, err := consumeDeviceCode(ctx, client, s.deps.Clock, code)
	if err != nil {
		s.endFlowFailed(ctx, client, err)
		return
	}
	s.completeSignIn(client, approval)
}

// consumeDeviceCode polls the code until it is approved, denied, expired or ctx ends. Failed polls are retried
// until the window closes, since the approval page may still be finishing; the window is the code's lifetime.
func consumeDeviceCode(ctx context.Context, client *backend.Client, clock platform.Clock, code string) (backend.Approval, error) {
	deadline := clock.Now().Add(consumeWindow)
	interval := consumePollEvery
	for clock.Now().Before(deadline) {
		if err := waitFor(ctx, clock, interval); err != nil {
			return backend.Approval{}, err
		}
		poll, err := client.PollDeviceSignIn(ctx, code)
		if err != nil {
			if ctx.Err() != nil {
				return backend.Approval{}, ctx.Err()
			}
			continue
		}
		switch poll.Phase {
		case backend.DeviceApproved:
			return backend.Approval{Token: poll.Token, User: poll.User}, nil
		case backend.DeviceDenied:
			return backend.Approval{}, backend.ErrSignInDenied
		case backend.DeviceExpired:
			return backend.Approval{}, backend.ErrSignInExpired
		case backend.DeviceSlowDown:
			interval += consumeSlowDownStep
		}
	}
	return backend.Approval{}, backend.ErrSignInExpired
}

// waitFor waits d on the clock, or returns the context's error when ctx ends first.
func waitFor(ctx context.Context, clock platform.Clock, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-clock.After(d):
		return nil
	}
}
