package backend

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestCanaryTokenNeverReachesLogsOrErrors drives authenticated calls through success, every failure family,
// a stream, a sign-in start and a network failure, then searches everything the logger keeps: the in-memory ring,
// the log files on disk, and every error text. The canary token must appear in none of them.
func TestCanaryTokenNeverReachesLogsOrErrors(t *testing.T) {
	h := newHarness(t)
	backend := h.withBackend(t)
	ctx := context.Background()
	var errs []error

	_, err := h.client.Me(ctx)
	errs = append(errs, err)
	backend.SetMode(fixtures.ModeUnauthorized)
	_, err = h.client.Me(ctx)
	errs = append(errs, err)
	backend.SetMode(fixtures.ModeInsufficientCredits)
	_, err = h.client.ActiveModel(ctx, ActiveModelQuery{})
	errs = append(errs, err)
	backend.SetMode(fixtures.ModeMidStreamError)
	stream, err := h.client.ChatStream(ctx, ChatRequest{Messages: userMessages("hi")})
	errs = append(errs, err)
	if stream != nil {
		for range 100 {
			event, nextErr := stream.Next()
			if nextErr != nil {
				break
			}
			if event.Err != nil {
				errs = append(errs, event.Err)
			}
		}
		_ = stream.Close()
	}
	h.http.Script(fake.Response{Err: errNetworkDown}, fake.Response{Err: errNetworkDown}, fake.Response{Err: errNetworkDown})
	for range 3 {
		_, err = h.client.StartDeviceSignIn(ctx)
		errs = append(errs, err)
	}
	if err := h.logger.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	assertNoCanary(t, h)
	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), canaryToken) {
			t.Fatalf("error text carries the token: %v", err)
		}
		if err != nil && strings.Contains(UserMessage(err), canaryToken) {
			t.Fatalf("palette copy carries the token: %q", UserMessage(err))
		}
	}
	if n := len(h.logger.Recent(0)); n == 0 {
		t.Fatal("the logger kept no entries, so the canary check proved nothing")
	}
}

// assertNoCanary searches the logger's ring and every file in its directory for the canary token. The logs must
// also contain the request lines, so the search is over real output.
func assertNoCanary(t *testing.T, h harness) {
	t.Helper()
	for _, entry := range h.logger.Recent(0) {
		encoded, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("encode entry: %v", err)
		}
		if strings.Contains(string(encoded), canaryToken) {
			t.Fatalf("ring entry carries the token: %s", encoded)
		}
	}
	sawRequest := false
	for _, name := range h.files.Paths() {
		info, err := h.files.Stat(name)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.IsDir() {
			continue
		}
		file, err := h.files.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(file), canaryToken) {
			t.Fatalf("log file %s carries the token", name)
		}
		if strings.Contains(string(file), "backend request") {
			sawRequest = true
		}
	}
	if !sawRequest {
		t.Fatal("no request lines reached the log files, so the canary check proved nothing")
	}
}
