package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

func TestAnalyticsEndpointFollowsPackagedIdentity(t *testing.T) {
	previous := releaseBuild
	releaseBuild = "false"
	t.Cleanup(func() { releaseBuild = previous })
	if got := analyticsEndpoint(false); got != "" {
		t.Fatalf("development endpoint = %q, want disabled", got)
	}
	if got := analyticsEndpoint(true); got != analytics.IngestEndpoint {
		t.Fatalf("packaged endpoint = %q, want ingest endpoint", got)
	}
}

func TestAnalyticsSettingsOptInObservesSignedInOwner(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: 200})
	logger, err := logx.New(nil, set.Clock, "", logx.Options{})
	if err != nil { t.Fatal(err) }
	defer logger.Close()
	st, err := store.Open(set.FS, set.Clock, "/config")
	if err != nil { t.Fatal(err) }
	defer st.Close()
	profile := auth.ProfileDocument(st)
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil { t.Fatal(err) }
	const rawID = "owner-private-canary-91"
	if err := profile.Update(func(p *auth.Profile) { p.Connections[env.BaseURL] = auth.Connection{UserID: rawID, Email: "owner-private@example.test"} }); err != nil { t.Fatal(err) }
	if err := auth.NewVault(set.Keychain).SaveToken(env.BaseURL, backend.Token("token-canary")); err != nil { t.Fatal(err) }
	app := &App{S: &Settings{}}
	app.analytics = analytics.New(analytics.Config{Endpoint: "https://collector.test", APIKey: analytics.IngestKey, Transport: set.HTTP, FS: set.FS, Clock: set.Clock, Dir: "/analytics", Consent: false})
	app.account, err = auth.New(auth.Deps{HTTP: set.HTTP, Clock: set.Clock, Keychain: set.Keychain, Logger: logger, Profile: profile})
	if err != nil { t.Fatal(err) }
	if err := (&settingsStore{p: app}).saveDiagnostics(json.RawMessage("true")); err != nil { t.Fatal(err) }
	if err := app.analytics.Flush(context.Background()); err != nil { t.Fatal(err) }
	requests := set.HTTP.Requests()
	if len(requests) != 1 { t.Fatalf("collector requests = %d, want 1", len(requests)) }
	var body map[string]any
	if err := json.Unmarshal(requests[0].Body, &body); err != nil { t.Fatal(err) }
	encoded, _ := json.Marshal(body)
	want := sha256.Sum256([]byte(rawID))
	if !strings.Contains(string(encoded), hex.EncodeToString(want[:])) { t.Fatalf("hashed owner missing from payload: %s", encoded) }
	if strings.Contains(string(encoded), rawID) || strings.Contains(string(encoded), "owner-private@example.test") { t.Fatalf("private account identity leaked: %s", encoded) }
}
