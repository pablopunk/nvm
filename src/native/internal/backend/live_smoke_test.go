//go:build livesmoke

package backend

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// liveSmokeTimeout bounds the whole smoke run.
const liveSmokeTimeout = 2 * time.Minute

// stdHTTP is the production HTTP port built on net/http. Only this opt-in smoke uses it.
type stdHTTP struct {
	client *http.Client
}

// Do sends the request with the standard client.
func (s stdHTTP) Do(req *platform.Request) (*platform.Response, error) {
	return s.client.Do(req)
}

// TestLiveSmoke calls the deployed backend with a real token: the compatibility manifest, the account, the model
// list, and a 20-token streamed completion. It is opt-in: build with -tags livesmoke and set NVM_TOKEN and
// NVM_BASE_URL. Without them it skips. It logs no token and no account email.
func TestLiveSmoke(t *testing.T) {
	token := os.Getenv("NVM_TOKEN")
	base := os.Getenv("NVM_BASE_URL")
	if token == "" || base == "" {
		t.Skip("set NVM_TOKEN and NVM_BASE_URL to run the live smoke")
	}
	kind := EnvironmentCustom
	if base == productionBaseURL {
		kind = EnvironmentProduction
	}
	env, err := NewEnvironment(kind, base, false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	ports := real.NewPorts()
	logger, err := logx.New(nil, ports.Clock, "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	defer func() { _ = logger.Close() }()
	client, err := New(stdHTTP{client: &http.Client{}}, logger, env, staticTokens{token: token}, WithClock(ports.Clock))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), liveSmokeTimeout)
	defer cancel()

	report, err := client.Compatibility(ctx)
	if err != nil {
		t.Fatalf("Compatibility: %v", err)
	}
	t.Logf("compatibility: state %s, backend %s (%s)", report.State, report.Manifest.Backend.Version, report.Manifest.Backend.Environment)

	account, err := client.Me(ctx)
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	t.Logf("account: plan %s, balance %d", account.Plan, account.Balance)

	route, err := client.ActiveModel(ctx, ActiveModelQuery{})
	if err != nil {
		t.Fatalf("ActiveModel: %v", err)
	}
	t.Logf("active model: %s (%s)", route.ID, route.API)

	stream, err := client.ChatStream(ctx, ChatRequest{Messages: userMessages("Reply with one word."), MaxTokens: 20})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	var text []byte
	for {
		event, nextErr := stream.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		switch event.Type {
		case EventContent:
			text = append(text, event.Text...)
		case EventError:
			t.Fatalf("stream failed: %v", event.Err)
		case EventDone:
			if len(text) == 0 {
				t.Fatal("stream ended with no text")
			}
			t.Logf("stream: %d bytes of text, finish reason %q", len(text), event.FinishReason)
		}
	}
}
