package app

import (
	"context"
	"net/http"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/update"
	updatefake "github.com/pablopunk/nvm/native/internal/update/fake"
)

// connectedBase is the signed-in backend the compatibility tests use.
const connectedBase = "http://localhost:4321"

// compatibilityManifest is the manifest /api/compatibility answers with, saying whether it accepts this client.
func compatibilityManifest(compatible bool) fake.Response {
	body := `{"backend":{"version":"0.9.0","environment":"development"},` +
		`"api":{"currentVersion":1,"supportedVersions":[1]},` +
		`"desktop":{"minimumSupportedVersion":"0.1.0","latestVersion":null,"updateUrl":"https://github.com/pablopunk/nvm/releases/latest","supportPolicy":"latest_two_minor_versions_or_90_days"},` +
		`"client":{"name":"desktop","version":"2.0.0-go","apiVersion":1,"platform":"darwin","arch":"arm64","compatible":` +
		map[bool]string{true: "true", false: "false"}[compatible] + `,"unsupportedReason":null},` +
		`"features":{},"notices":[]}`
	return fake.Response{Status: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: []byte(body)}
}

// noToken is the token source of a signed-out client; the compatibility request is anonymous.
type noToken struct{}

func (noToken) Get() (string, error) { return "", nil }

// menuClient is the backend client over the fake HTTP port, as the session builds it for the signed-in backend.
func menuClient(t *testing.T, m *menuUpdates) *backend.Client {
	t.Helper()
	env, err := backend.NewEnvironment(backend.EnvironmentDevelopment, connectedBase, false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	client, err := backend.New(m.set.HTTP, logx.Default(), env, noToken{}, backend.WithClock(m.set.Clock))
	if err != nil {
		t.Fatalf("backend.New: %v", err)
	}
	return client
}

func TestCompatibilityRowTakesTheUpdateRowsPlace(t *testing.T) {
	cases := []struct {
		name     string
		engine   func() *updatefake.Engine
		drive    func(t *testing.T, m *menuUpdates)
		disabled bool
		click    func(t *testing.T, m *menuUpdates)
	}{
		{
			name:   "idle opens the Updates page and checks",
			engine: func() *updatefake.Engine { return updatefake.NewEngine(nil) },
			click: func(t *testing.T, m *menuUpdates) {
				m.click(t, "update")
				if title, shown := m.topPage(); title != "Updates" || !shown {
					t.Fatalf("top page = %q shown %v, want the Updates page", title, shown)
				}
				eventually(t, "the check to run", func() bool { return m.engine.Checks() == 1 })
			},
		},
		{
			name:   "downloading opens the Updates page",
			engine: func() *updatefake.Engine { return newOfferEngine(updatefake.Install{Block: true}) },
			drive: func(t *testing.T, m *menuUpdates) {
				checkOnce(t, m)
				m.waitForStatus(t, update.StatusDownloading)
			},
			click: func(t *testing.T, m *menuUpdates) {
				m.click(t, "update")
				if title, _ := m.topPage(); title != "Updates" {
					t.Fatalf("top page = %q, want Updates", title)
				}
			},
		},
		{
			name:   "ready restarts into the update",
			engine: func() *updatefake.Engine { return newOfferEngine(updatefake.Install{}) },
			drive: func(t *testing.T, m *menuUpdates) {
				checkOnce(t, m)
				m.waitForStatus(t, update.StatusDownloaded)
			},
			click: func(t *testing.T, m *menuUpdates) {
				m.click(t, "update")
				if got := m.engine.Relaunches(); got != 1 {
					t.Fatalf("relaunches = %d, want 1", got)
				}
			},
		},
		{
			name:     "unsupported build has no action, not the manifest's update URL",
			engine:   updatefake.Disabled,
			disabled: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMenuUpdates(t, tc.engine())
			m.app.compat.setActive(connectedBase)
			m.app.compat.record(connectedBase, false)
			m.onUIThread(m.app.refreshMenubarMenu)
			if tc.drive != nil {
				tc.drive(t, m)
			}
			row := menuRowOf(t, m.tray.Items(), "update")
			if row.Label != "Update Nevermind" || row.Disabled != tc.disabled {
				t.Fatalf("row = %q disabled %v, want Update Nevermind disabled %v", row.Label, row.Disabled, tc.disabled)
			}
			if row.Icon == nil || !row.Icon.Template {
				t.Fatal("the compatibility row has no download glyph on macOS")
			}
			if tc.click != nil {
				tc.click(t, m)
			}
		})
	}
}

func TestCompatibilityRowLeavesTheMenuWhenAcceptedOrSignedOut(t *testing.T) {
	m := newMenuUpdates(t, updatefake.NewEngine(nil))
	m.app.compat.setActive(connectedBase)
	m.app.compat.record(connectedBase, true)
	m.onUIThread(m.app.refreshMenubarMenu)
	if label := menuRowOf(t, m.tray.Items(), "check-updates").Label; label != "Check for Updates…" {
		t.Fatalf("accepted backend shows %q, want Check for Updates…", label)
	}
	m.app.compat.record(connectedBase, false)
	m.app.compat.setActive("")
	m.onUIThread(m.app.refreshMenubarMenu)
	if label := menuRowOf(t, m.tray.Items(), "check-updates").Label; label != "Check for Updates…" {
		t.Fatalf("signed-out app shows %q, want Check for Updates…", label)
	}
}

func TestCompatibilityFetchStoresTheManifestForTheMenu(t *testing.T) {
	m := newMenuUpdates(t, updatefake.NewEngine(nil))
	m.app.compat.setActive(connectedBase)
	client := menuClient(t, m)
	m.set.HTTP.Script(compatibilityManifest(false))
	if !m.app.fetchCompatibility(context.Background(), client, connectedBase) {
		t.Fatal("a manifest that refuses the client did not change the answer")
	}
	if !m.app.compat.rejectsClient() {
		t.Fatal("the fetched manifest does not refuse the client")
	}
	m.set.HTTP.Script(fake.Response{Status: http.StatusNotFound})
	if m.app.fetchCompatibility(context.Background(), client, connectedBase) {
		t.Fatal("a backend without a manifest changed the answer")
	}
	if !m.app.compat.rejectsClient() {
		t.Fatal("a backend without a manifest dropped the answer it had; 1.x keeps its cache")
	}
	m.set.HTTP.Script(compatibilityManifest(true))
	m.app.fetchCompatibility(context.Background(), client, connectedBase)
	if m.app.compat.rejectsClient() {
		t.Fatal("an accepting manifest still refuses the client")
	}
}
