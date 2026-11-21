package app

import (
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"net/http"
	"testing"
	"time"
)

// fixedToken is the signed-in account's token for the fake backend.
type fixedToken struct{}

func (fixedToken) Get() (string, error) { return "test-token", nil }

// decisionsBackendServing is decisionsBackend with the handler of the decisions route given.
func decisionsBackendServing(t *testing.T, serve http.HandlerFunc) (*backend.Client, *fake.HTTP) {
	t.Helper()
	clock := fake.NewClock()
	logger, err := logx.New(nil, clock, "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	port := fake.NewHTTP()
	port.Handle(serve)
	client, err := backend.New(port, logger, env, fixedToken{}, backend.WithClock(clock), backend.WithJitter(func(d time.Duration) time.Duration { return d }))
	if err != nil {
		t.Fatalf("backend.New: %v", err)
	}
	return client, port
}
