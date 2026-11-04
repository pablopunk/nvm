package backend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// jsonBody is a fake response body with the JSON content type.
func jsonBody(status int, body string) fake.Response {
	return fake.Response{Status: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(body)}
}

// TestNewRejectsMissingDependencies checks the constructor refuses a missing clock, port, logger or token source,
// and a malformed environment, instead of failing later on a request.
func TestNewRejectsMissingDependencies(t *testing.T) {
	env, err := NewEnvironment(EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	clock := fake.NewClock()
	logger, err := logx.New(nil, clock, "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	withTokens := staticTokens{token: canaryToken}
	tests := []struct {
		name       string
		withPort   bool
		withLogger bool
		withTokens bool
		env        Environment
		opts       []Option
	}{
		{name: "no clock", withPort: true, withLogger: true, withTokens: true, env: env},
		{name: "no logger", withPort: true, withTokens: true, env: env, opts: []Option{WithClock(clock)}},
		{name: "no tokens", withPort: true, withLogger: true, env: env, opts: []Option{WithClock(clock)}},
		{name: "no port", withLogger: true, withTokens: true, env: env, opts: []Option{WithClock(clock)}},
		{name: "unsafe environment", withPort: true, withLogger: true, withTokens: true, env: Environment{Kind: EnvironmentCustom, BaseURL: "http://x.example"}, opts: []Option{WithClock(clock)}},
		{name: "zero retry attempts", withPort: true, withLogger: true, withTokens: true, env: env, opts: []Option{WithClock(clock), WithRetryPolicy(0, 0, 0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var port platform.HTTP
			if tt.withPort {
				port = fake.NewHTTP()
			}
			var logs *logx.Logger
			if tt.withLogger {
				logs = logger
			}
			var tokens TokenSource
			if tt.withTokens {
				tokens = withTokens
			}
			if _, err := New(port, logs, tt.env, tokens, tt.opts...); err == nil {
				t.Fatal("New succeeded, want an error")
			}
		})
	}
}

// TestEveryRequestCarriesCompatibilityHeaders checks the five compatibility headers and the bearer token are on
// an authenticated request, the request ID is fresh, and the URL comes from the environment.
func TestEveryRequestCarriesCompatibilityHeaders(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, string(testManifest(nil))))
	if _, err := h.client.Compatibility(context.Background()); err != nil {
		t.Fatalf("Compatibility: %v", err)
	}
	requests := h.http.Requests()
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	request := requests[0]
	assertCompatibilityHeaders(t, request.Header)
	if got := request.Header.Get("Authorization"); got != "Bearer "+canaryToken {
		t.Fatalf("Authorization = %q, want the bearer token", got)
	}
	if request.URL != "https://api.nvm.fyi/api/compatibility" {
		t.Fatalf("URL = %q, want the compatibility route on the production origin", request.URL)
	}
}

// assertCompatibilityHeaders checks every compatibility header has the expected value.
func assertCompatibilityHeaders(t *testing.T, header http.Header) {
	t.Helper()
	want := map[string]string{
		"X-Nevermind-Client":         "desktop",
		"X-Nevermind-Client-Version": "1.2.3",
		"X-Nevermind-API-Version":    "1",
		"X-Nevermind-Platform":       nodePlatformName(runtime.GOOS),
		"X-Nevermind-Arch":           nodeArchName(runtime.GOARCH),
	}
	for name, value := range want {
		if got := header.Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if header.Get("X-Request-ID") == "" {
		t.Error("X-Request-ID is missing")
	}
}

// TestRequestIDsAreFreshPerRequest checks two requests never share a request ID, so support can tell them apart.
func TestRequestIDsAreFreshPerRequest(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, string(testManifest(nil))), jsonBody(http.StatusOK, string(testManifest(nil))))
	for range 2 {
		if _, err := h.client.Compatibility(context.Background()); err != nil {
			t.Fatalf("Compatibility: %v", err)
		}
	}
	requests := h.http.Requests()
	if requests[0].Header.Get("X-Request-ID") == requests[1].Header.Get("X-Request-ID") {
		t.Fatal("two requests share an X-Request-ID")
	}
}

// TestDeviceRequestsSendNoToken checks the sign-in calls run before a token exists, so they must not send one.
func TestDeviceRequestsSendNoToken(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")))
	if _, err := h.client.StartDeviceSignIn(context.Background()); err != nil {
		t.Fatalf("StartDeviceSignIn: %v", err)
	}
	request := h.http.Requests()[0]
	if got := request.Header.Get("Authorization"); got != "" {
		t.Fatalf("device request sent Authorization %q, want none", got)
	}
	if !strings.Contains(string(request.Body), `"label":"Nevermind native"`) {
		t.Fatalf("device request body %s lacks the client label", request.Body)
	}
}

// TestAuthenticatedCallFailsWithoutSending checks a call that needs a token fails as Unauthorized, and makes
// no request, when the token source has nothing.
func TestAuthenticatedCallFailsWithoutSending(t *testing.T) {
	h := newHarness(t)
	client := clientWithTokens(t, h, staticTokens{err: errors.New("keychain is locked")})
	_, err := client.ActiveModel(context.Background(), ActiveModelQuery{})
	if !IsKind(err, KindUnauthorized) {
		t.Fatalf("ActiveModel error = %v, want KindUnauthorized", err)
	}
	if n := len(h.http.Requests()); n != 0 {
		t.Fatalf("made %d requests without a token, want none", n)
	}
}

// TestOptionalAuthSendsWithoutTokenWhenNoneExists checks the compatibility manifest is fetched anonymously when
// there is no token, rather than failing the startup check.
func TestOptionalAuthSendsWithoutTokenWhenNoneExists(t *testing.T) {
	h := newHarness(t)
	client := clientWithTokens(t, h, staticTokens{err: errors.New("no token yet")})
	h.http.Script(jsonBody(http.StatusOK, string(testManifest(nil))))
	if _, err := client.Compatibility(context.Background()); err != nil {
		t.Fatalf("Compatibility without a token: %v", err)
	}
	if got := h.http.Requests()[0].Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want none", got)
	}
}

// clientWithTokens builds a second client on the harness's fakes with a different token source.
func clientWithTokens(t *testing.T, h harness, tokens TokenSource) *Client {
	t.Helper()
	env, err := NewEnvironment(EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	client, err := New(h.http, h.logger, env, tokens, WithClock(h.clock), WithJitter(noJitter), WithClientVersion("1.2.3"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// TestMeReadsTheFixtureAccount checks the account call decodes identity and plan. The balance is not asserted:
// the fixture's account shape differs from the route, which is noted in the package report.
func TestMeReadsTheFixtureAccount(t *testing.T) {
	h := newHarness(t)
	h.withBackend(t)
	account, err := h.client.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if account.Email != "fixture@example.test" || account.Plan != "free" {
		t.Fatalf("Me = %+v, want the fixture account", account)
	}
}

// TestActiveModelSendsTheQueryAndDecodesTheDescriptor checks the query parameters the route reads and the
// descriptor fields the desktop app uses.
func TestActiveModelSendsTheQueryAndDecodesTheDescriptor(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, descriptorJSON("openai-completions")))
	descriptor, err := h.client.ActiveModel(context.Background(), ActiveModelQuery{Role: "smart", Chars: 40, ForceFree: true})
	if err != nil {
		t.Fatalf("ActiveModel: %v", err)
	}
	if want := "https://api.nvm.fyi/api/v1/active-model?chars=40&model=smart&tier=free"; h.http.Requests()[0].URL != want {
		t.Fatalf("URL = %q, want %q", h.http.Requests()[0].URL, want)
	}
	if descriptor.ID != "fixture-route" || descriptor.Credits == nil || descriptor.Credits.Total != 81 {
		t.Fatalf("descriptor = %+v, want the decoded route and credits", descriptor)
	}
	if !descriptor.SpeaksChatCompletions() {
		t.Fatal("openai-completions route should speak chat completions")
	}
}

// TestSpeaksChatCompletionsOnlyForOpenAIFormat checks other wire formats are reported as unsupported by ChatStream.
func TestSpeaksChatCompletionsOnlyForOpenAIFormat(t *testing.T) {
	for api, want := range map[string]bool{
		"openai-completions": true,
		"anthropic-messages": false,
		"openai-responses":   false,
		"":                   false,
	} {
		if got := (ModelDescriptor{API: api}).SpeaksChatCompletions(); got != want {
			t.Errorf("SpeaksChatCompletions(%q) = %v, want %v", api, got, want)
		}
	}
}

// TestUnknownRouteOnFixtureIsRejected checks a route the fixture does not serve is a Rejected 404 with a message,
// not a decoding error.
func TestUnknownRouteOnFixtureIsRejected(t *testing.T) {
	h := newHarness(t)
	h.withBackend(t)
	var out struct{}
	err := h.client.callJSON(context.Background(), requestSpec{method: "GET", path: "/api/v1/not-served", auth: authRequired}, &out)
	if !IsKind(err, KindRejected) || statusOf(err) != statusNotFound {
		t.Fatalf("a call to an unserved route = %v, want KindRejected with status 404", err)
	}
}

// TestOnlineStartsTrue checks a new client reports online before any request.
func TestOnlineStartsTrue(t *testing.T) {
	if !newHarness(t).client.Online() {
		t.Fatal("a new client should report online")
	}
}

// TestEnvironmentIsTheValidatedOrigin checks the client exposes the origin it was built with.
func TestEnvironmentIsTheValidatedOrigin(t *testing.T) {
	h := newHarness(t)
	if got := h.client.Environment(); got.Kind != EnvironmentProduction || got.BaseURL != "https://api.nvm.fyi" {
		t.Fatalf("Environment() = %+v", got)
	}
}

// fiveMinutes is the device code lifetime the backend issues.
const fiveMinutes = 5 * time.Minute

// descriptorJSON returns an active-model descriptor for the given wire format.
func descriptorJSON(api string) string {
	body := map[string]any{
		"id":            "fixture-route",
		"name":          "Fixture Route",
		"contextWindow": 128000,
		"maxTokens":     4096,
		"reasoning":     false,
		"input":         []string{"text"},
		"api":           api,
		"provider":      "fixture",
		"baseUrl":       "https://api.nvm.fyi/api/v1",
		"modelTier":     "free",
		"creditKind":    "free",
		"credits":       map[string]int{"paid": 0, "free": 81, "total": 81},
		"notice":        "ok",
	}
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(data)
}
