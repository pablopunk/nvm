package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// TokenSource supplies the bearer token for authenticated calls. Get returns an error when the user is not
// signed in or the credential store cannot be read; the client then fails the call as Unauthorized without
// sending it.
type TokenSource interface {
	// Get returns the current token.
	Get() (string, error)
}

// Client talks to one backend origin. It is safe for concurrent use.
type Client struct {
	http        platform.HTTP
	logger      *logx.Logger
	env         Environment
	tokens      TokenSource
	clock       platform.Clock
	identity    identity
	retry       retryPolicy
	jitter      func(time.Duration) time.Duration
	deviceLabel string
	offline     offlineTracker
	// pause waits d between attempts and polls. It is the one wait the client makes; tests replace it to move the
	// fake clock inline instead of from another goroutine.
	pause func(ctx context.Context, d time.Duration) error
}

// New returns a client for env. It validates the environment against the packaged policy and checks that the
// clock, HTTP port, logger and token source are present. The logger must not be nil; use a memory-only logx
// logger in tests.
func New(httpPort platform.HTTP, logger *logx.Logger, env Environment, tokens TokenSource, opts ...Option) (*Client, error) {
	cfg := settings{
		identity:    defaultIdentity(),
		retry:       defaultRetryPolicy(),
		jitter:      defaultJitter,
		deviceLabel: defaultDeviceLabel,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := validateSettings(cfg); err != nil {
		return nil, err
	}
	if httpPort == nil || logger == nil || tokens == nil {
		return nil, fmt.Errorf("backend: New needs an HTTP port, a logger and a token source")
	}
	validated, err := NewEnvironment(env.Kind, env.BaseURL, cfg.packaged)
	if err != nil {
		return nil, err
	}
	client := &Client{
		http:        httpPort,
		logger:      logger,
		env:         validated,
		tokens:      tokens,
		clock:       cfg.clock,
		identity:    cfg.identity,
		retry:       cfg.retry,
		jitter:      cfg.jitter,
		deviceLabel: cfg.deviceLabel,
	}
	client.pause = client.clockPause
	return client, nil
}

// Environment returns the validated origin this client talks to.
func (c *Client) Environment() Environment {
	return c.env
}

// Online reports whether the last few requests reached the backend. It turns false after consecutive
// network failures and true again on the next response. Requests are still attempted while offline, so
// recovery is noticed without a separate probe.
func (c *Client) Online() bool {
	return !c.offline.isOffline()
}
