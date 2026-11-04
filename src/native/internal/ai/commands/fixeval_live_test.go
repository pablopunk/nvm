//go:build liveeval

package commands

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// liveEvalTimeout bounds the whole live run: every case makes at least one model call.
const liveEvalTimeout = 15 * time.Minute

// liveNearRatio is how close a live correction must be to the expected text to count as the expected correction. A
// model varies its wording; the guard already bounds how much it may change, so the live score allows this share of
// the characters to differ.
const liveNearRatio = 0.1

// liveStdHTTP is the production HTTP port for the live eval.
type liveStdHTTP struct{ client *http.Client }

// Do sends the request with the standard client.
func (s liveStdHTTP) Do(req *platform.Request) (*platform.Response, error) {
	return s.client.Do(req)
}

// liveTokens hands the eval the token from NVM_TOKEN.
type liveTokens string

// Get returns the bearer token.
func (t liveTokens) Get() (string, error) { return string(t), nil }

// TestLiveFixEval runs the case set against the real backend and prints a score table. It is opt-in: build with
// -tags liveeval and set NVM_TOKEN and NVM_BASE_URL. Without them it skips. The model answers each case once per
// attempt; the table shows whether each case behaved as expected and how many characters the correction changed.
func TestLiveFixEval(t *testing.T) {
	token, base := os.Getenv("NVM_TOKEN"), os.Getenv("NVM_BASE_URL")
	if token == "" || base == "" {
		t.Skip("set NVM_TOKEN and NVM_BASE_URL to run the live correction eval")
	}
	env, err := backend.NewEnvironment(backend.EnvironmentCustom, base, false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	ports := real.NewPorts()
	logger, err := logx.New(nil, ports.Clock, "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	defer func() { _ = logger.Close() }()
	client, err := backend.New(liveStdHTTP{client: &http.Client{}}, logger, env, liveTokens(token), backend.WithClock(ports.Clock))
	if err != nil {
		t.Fatalf("backend.New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), liveEvalTimeout)
	defer cancel()
	source := liveSource{client: client}
	ask := func(system, user string) (string, error) {
		return chat.Complete(ctx, source, system, user)
	}
	passed := 0
	cases := loadFixEval(t)
	for _, c := range cases {
		got, err := CorrectText(c.Input, ask, nil)
		ok, reason := scoreLiveCase(c, got, err)
		if ok {
			passed++
		}
		t.Logf("%-16s %-10s %-5t changed=%-4d %s", c.ID, c.Expect, ok, editCount(c.Input, got), reason)
	}
	t.Logf("live fixeval: %d/%d cases pass", passed, len(cases))
}

// liveSource adapts the backend client to the one-shot completion port of the chat package.
type liveSource struct{ client *backend.Client }

// ActiveModel asks the backend which route serves the query.
func (s liveSource) ActiveModel(ctx context.Context, query backend.ActiveModelQuery) (backend.ModelDescriptor, error) {
	return s.client.ActiveModel(ctx, query)
}

// ChatStream opens the streamed reply of one chat request.
func (s liveSource) ChatStream(ctx context.Context, req backend.ChatRequest) (chat.EventStream, error) {
	stream, err := s.client.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// scoreLiveCase judges a live correction. A fixed case passes when the text is within liveNearRatio of the expected
// one and keeps its protected tokens; every other expectation is scored as the L1 eval scores it.
func scoreLiveCase(c evalCase, got string, err error) (bool, string) {
	if c.Expect == expectFixed && err == nil && charEditRatio(got, c.Output) <= liveNearRatio {
		c.Output = got
	}
	return scoreCase(c, got, err)
}

// editCount is the number of characters that differ between input and output, for the table.
func editCount(input, output string) int {
	return editDistance([]rune(input), []rune(output))
}
