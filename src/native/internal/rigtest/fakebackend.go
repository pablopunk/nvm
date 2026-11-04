//go:build rig

package rigtest

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// FakeBackend is the recorded fake backend (internal/fixtures) served on a loopback listener for one test. Launch
// points the app at URL through Options.Backend. Its methods (SetMode, SetReply, SetChunkDelay, SetActiveModel,
// Requests, ApproveDeviceCode, LatestDeviceCode) script the replies and approve sign-ins.
type FakeBackend struct {
	*fixtures.Backend
	// URL is the origin of the listener, http://127.0.0.1:<port>.
	URL string
}

// StartFakeBackend serves a healthy fake backend on 127.0.0.1 until the test ends.
func StartFakeBackend(t *testing.T) *FakeBackend {
	t.Helper()
	// Sign-in codes expire on the clock the app uses, so the fake reads the real one.
	backend := fixtures.NewBackendWithClock(real.NewPorts().Clock)
	server := httptest.NewServer(backend)
	t.Cleanup(server.Close)
	return &FakeBackend{Backend: backend, URL: server.URL}
}

// WaitForDeviceCode waits until the app starts a sign-in and returns the code it shows.
func (f *FakeBackend) WaitForDeviceCode(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		if code := f.LatestDeviceCode(); code != "" {
			return code, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no device sign-in started within %s", timeout)
		}
		time.Sleep(pollInterval)
	}
}
