package fixtures

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// chatRequestBody is a valid chat completion request with a stream flag, as the client sends it.
const chatRequestBody = `{"model":"fixture-fast","stream":true,"messages":[{"role":"user","content":"hi"}]}`

// doThroughPort sends a request through the fake HTTP port, so the tests exercise the same path the app uses.
func doThroughPort(t *testing.T, set *fake.Set, ctx context.Context, method, path, body string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, method, "https://backend.test"+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := set.Ports().HTTP.Do(req)
	if err != nil {
		t.Fatalf("Do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header, string(data)
}

func TestFailureModesReturnExactStatusAndBody(t *testing.T) {
	tests := []struct {
		name       string
		mode       FailureMode
		wantStatus int
		wantBody   string
		wantHeader map[string]string
	}{
		{
			name:       "401 unauthorized",
			mode:       ModeUnauthorized,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Unauthorized",
		},
		{
			name:       "402 insufficient credits",
			mode:       ModeInsufficientCredits,
			wantStatus: http.StatusPaymentRequired,
			wantBody:   `{"error":{"type":"insufficient_credits","message":"Request cost would exceed remaining balance","estimated_credits":95,"balance":81}}`,
		},
		{
			name:       "429 rate limited",
			mode:       ModeRateLimited,
			wantStatus: http.StatusTooManyRequests,
			wantBody:   `{"error":{"type":"rate_limited","message":"Rate limit exceeded (chat)","retry_after":30}}`,
			wantHeader: map[string]string{"Retry-After": "30"},
		},
		{
			name:       "500 server error",
			mode:       ModeServerError,
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":{"type":"internal_error","message":"Internal server error"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := NewBackend()
			backend.SetMode(tt.mode)
			set := fake.NewSet()
			set.HTTP.Handle(backend)

			for _, route := range []struct{ method, path string }{
				{http.MethodPost, ChatCompletionsPath},
				{http.MethodGet, ModelsPath},
				{http.MethodGet, MePath},
			} {
				status, header, body := doThroughPort(t, set, context.Background(), route.method, route.path, chatRequestBody)
				if status != tt.wantStatus || body != tt.wantBody {
					t.Fatalf("%s %s: status %d body %q; want %d %q", route.method, route.path, status, body, tt.wantStatus, tt.wantBody)
				}
				for key, want := range tt.wantHeader {
					if got := header.Get(key); got != want {
						t.Fatalf("%s %s header %s = %q, want %q", route.method, route.path, key, got, want)
					}
				}
			}
		})
	}
}

func TestErrorBodiesAreValidJSONWhereTheClientsParseThem(t *testing.T) {
	for name, body := range map[string]string{
		"insufficient credits": InsufficientCreditsBody,
		"rate limited":         RateLimitedBody,
		"server error":         ServerErrorBody,
	} {
		var parsed struct {
			Error struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil || parsed.Error.Type == "" {
			t.Fatalf("%s body is not an error object: %v", name, err)
		}
	}
}

func TestHealthyBackendStreamsTheReplyAndTerminates(t *testing.T) {
	backend := NewBackend()
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	status, header, body := doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	if status != http.StatusOK || header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d content type %q", status, header.Get("Content-Type"))
	}
	if !strings.HasSuffix(body, DoneFrame) {
		t.Fatalf("stream does not end with [DONE]: %q", body[max(0, len(body)-40):])
	}
	if got := assembleStream(t, body); got != FixtureReply {
		t.Fatalf("assembled reply %q, want %q", got, FixtureReply)
	}
}

func TestMalformedStreamIsBrokenAndUnterminated(t *testing.T) {
	backend := NewBackend()
	backend.SetMode(ModeMalformedSSE)
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	status, _, body := doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200 with a broken stream", status)
	}
	if strings.Contains(body, "[DONE]") {
		t.Fatal("malformed stream must not terminate cleanly")
	}
	frames := strings.Split(strings.TrimSpace(body), "\n\n")
	last := strings.TrimPrefix(frames[len(frames)-1], "data: ")
	if json.Valid([]byte(last)) {
		t.Fatalf("last frame %q is valid JSON; it must be malformed", last)
	}
}

func TestMidStreamErrorEmitsErrorEventWithoutTermination(t *testing.T) {
	backend := NewBackend()
	backend.SetMode(ModeMidStreamError)
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	_, _, body := doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	if !strings.Contains(body, `"type":"upstream_error"`) || strings.Contains(body, "[DONE]") {
		t.Fatalf("mid-stream error body = %q", body)
	}
}

func TestStreamModesDoNotAffectOtherRoutes(t *testing.T) {
	backend := NewBackend()
	backend.SetMode(ModeMalformedSSE)
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	status, _, body := doThroughPort(t, set, context.Background(), http.MethodGet, MePath, "")
	if status != http.StatusOK || body != MeBody {
		t.Fatalf("GET /api/me under a stream mode = %d %q", status, body)
	}
}

func TestTimeoutModeHoldsTheRequestUntilTheClientLeaves(t *testing.T) {
	backend := NewBackend()
	backend.SetMode(ModeTimeout)
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	ctx, cancel := context.WithCancel(context.Background())
	outcome := make(chan error, 1)
	go func() {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://backend.test"+MePath, nil)
		_, err := set.Ports().HTTP.Do(req)
		outcome <- err
	}()
	<-backend.Hanging()
	cancel()
	if err := <-outcome; !errors.Is(err, context.Canceled) {
		t.Fatalf("timed-out request returned %v, want context.Canceled", err)
	}
}

func TestHealthyRoutesReturnFixtureJSON(t *testing.T) {
	backend := NewBackend()
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	if _, _, body := doThroughPort(t, set, context.Background(), http.MethodGet, ModelsPath, ""); body != ModelsBody {
		t.Fatalf("models body = %q", body)
	}
	if _, _, body := doThroughPort(t, set, context.Background(), http.MethodGet, MePath, ""); body != MeBody {
		t.Fatalf("me body = %q", body)
	}
	if status, _, _ := doThroughPort(t, set, context.Background(), http.MethodGet, "/api/unknown", ""); status != http.StatusNotFound {
		t.Fatalf("unknown route status %d, want 404", status)
	}
}

func TestBackendRecordsRequestsInOrder(t *testing.T) {
	backend := NewBackend()
	set := fake.NewSet()
	set.HTTP.Handle(backend)
	doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	doThroughPort(t, set, context.Background(), http.MethodGet, MePath, "")
	recorded := backend.Requests()
	if len(recorded) != 2 || recorded[0].Path != ChatCompletionsPath || recorded[0].Body != chatRequestBody || recorded[1].Path != MePath {
		t.Fatalf("recorded requests = %+v", recorded)
	}
}

// assembleStream concatenates the content deltas of an SSE body, failing on any frame that is not valid JSON
// except the terminator.
func assembleStream(t *testing.T, body string) string {
	t.Helper()
	var reply strings.Builder
	for _, frame := range strings.Split(strings.TrimSpace(body), "\n\n") {
		payload := strings.TrimPrefix(frame, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("frame %q is not a chunk: %v", payload, err)
		}
		for _, choice := range chunk.Choices {
			reply.WriteString(choice.Delta.Content)
		}
	}
	return reply.String()
}
