// Package extaitest is the test rig of the extension model calls: a fake port set, the fixture backend behind a real
// backend client, and the call service over both. Tests sign the account out with SignOut.
//
// Layer: ai. It is for tests only; no production package imports it.
package extaitest

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// Rig is one account and one call service. Time is the fake set's clock; the logger uses the real clock, so a test
// that waits on a timer sees only the timers of the code under test.
type Rig struct {
	Set      *fake.Set
	Backend  *fixtures.Backend
	Service  *extai.Service
	signedIn bool
}

// staticToken is the bearer every test client carries.
type staticToken string

func (t staticToken) Get() (string, error) { return string(t), nil }

// New returns a rig signed in to a healthy fixture backend. The home folder is home/ada below the test volume root.
func New(t *testing.T) *Rig {
	t.Helper()
	set := fake.NewSet()
	fixture := fixtures.NewBackendWithClock(set.Clock)
	set.HTTP.Handle(fixture)
	logger, err := logx.New(set.FS, real.NewPorts().Clock, "/logs", logx.Options{FlushInterval: time.Second, Level: logx.Debug})
	if err != nil {
		t.Fatalf("logx: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("environment: %v", err)
	}
	client, err := backend.New(set.HTTP, logger, env, staticToken("test-token"),
		backend.WithClock(set.Clock),
		backend.WithJitter(func(d time.Duration) time.Duration { return d }),
		backend.WithRetryPolicy(1, 0, 0))
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	rig := &Rig{Set: set, Backend: fixture, signedIn: true}
	rig.Service = extai.New(extai.Deps{
		Client:    func() *backend.Client { return rig.client(client) },
		Connected: func() bool { return rig.signedIn },
		Clock:     set.Clock,
		Clipboard: set.Clipboard,
		Files:     set.FS,
		Home:      testpath.Abs("home", "ada"),
	})
	return rig
}

// client returns the backend client while the account is signed in, and nil after SignOut.
func (r *Rig) client(client *backend.Client) *backend.Client {
	if !r.signedIn {
		return nil
	}
	return client
}

// SignOut signs the account out: calls are refused as not signed in.
func (r *Rig) SignOut() {
	r.signedIn = false
}
