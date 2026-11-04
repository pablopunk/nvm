package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// testManifest returns a compatibility manifest as the backend sends it for a client at version 1.2.3. mutate
// may change or delete fields before it is encoded; a nil mutate keeps the defaults.
func testManifest(mutate func(map[string]any)) []byte {
	manifest := map[string]any{
		"backend": map[string]any{"version": "0.9.0", "environment": "production"},
		"api":     map[string]any{"currentVersion": 1, "supportedVersions": []int{1}},
		"desktop": map[string]any{
			"minimumSupportedVersion": "0.1.0",
			"latestVersion":           nil,
			"updateUrl":               "https://github.com/pablopunk/nvm/releases/latest",
			"supportPolicy":           "latest_two_minor_versions_or_90_days",
		},
		"client": map[string]any{
			"name":              "desktop",
			"version":           "1.2.3",
			"apiVersion":        1,
			"platform":          "linux",
			"arch":              "x64",
			"compatible":        true,
			"unsupportedReason": nil,
		},
		"features": map[string]bool{"proxy_streaming": true},
		"notices":  []any{},
	}
	if mutate != nil {
		mutate(manifest)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		panic(err)
	}
	return data
}

// compatibilityWith scripts one manifest for a client at version 1.2.3 and returns the derived report.
func compatibilityWith(t *testing.T, mutate func(map[string]any)) CompatReport {
	t.Helper()
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, string(testManifest(mutate))))
	report, err := h.client.Compatibility(context.Background())
	if err != nil {
		t.Fatalf("Compatibility: %v", err)
	}
	return report
}

// TestCompatibilityStateFollowsTheManifest checks each state the desktop app acts on, from the manifest fields that
// drive it.
func TestCompatibilityStateFollowsTheManifest(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   CompatState
	}{
		{name: "current client", want: CompatOK},
		{
			name: "backend refuses this version",
			mutate: func(m map[string]any) {
				client := m["client"].(map[string]any)
				client["compatible"] = false
				client["unsupportedReason"] = "unsupported_desktop_version"
			},
			want: CompatUpdateRequired,
		},
		{
			name: "force update notice",
			mutate: func(m map[string]any) {
				m["notices"] = []any{map[string]any{"type": "force_update", "message": "update now"}}
			},
			want: CompatUpdateRequired,
		},
		{
			name: "this API major was retired",
			mutate: func(m map[string]any) {
				m["api"].(map[string]any)["supportedVersions"] = []int{2}
			},
			want: CompatSunset,
		},
		{
			name: "backend reports an unsupported API version",
			mutate: func(m map[string]any) {
				client := m["client"].(map[string]any)
				client["compatible"] = false
				client["unsupportedReason"] = "unsupported_api_version"
			},
			want: CompatSunset,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compatibilityWith(t, tt.mutate).State; got != tt.want {
				t.Fatalf("State = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCompatibilityIgnoresTheManifestsLatestRelease checks that a release the manifest names as latest does not prompt
// this client. The backend's latestVersion is a 1.x date-stamped version, which sorts above every 2.0 build.
func TestCompatibilityIgnoresTheManifestsLatestRelease(t *testing.T) {
	report := compatibilityWith(t, func(m map[string]any) {
		desktop := m["desktop"].(map[string]any)
		desktop["latestVersion"] = "2026.1104.1"
		desktop["updateUrl"] = "https://github.com/pablopunk/nvm/releases/latest"
	})
	if report.State != CompatOK {
		t.Fatalf("State = %q, want %q: the manifest's latest release belongs to the 1.x line", report.State, CompatOK)
	}
}

// TestCompatibilityDecodesTheManifest checks the fields the UI shows are read into the typed manifest.
func TestCompatibilityDecodesTheManifest(t *testing.T) {
	manifest := compatibilityWith(t, nil).Manifest
	if manifest.Backend.Environment != "production" || manifest.API.CurrentVersion != 1 {
		t.Fatalf("manifest backend/api = %+v / %+v", manifest.Backend, manifest.API)
	}
	if manifest.Desktop.MinimumSupportedVersion != "0.1.0" {
		t.Fatalf("manifest desktop = %+v", manifest.Desktop)
	}
	if !manifest.Client.Compatible || !manifest.Features["proxy_streaming"] {
		t.Fatalf("manifest client/features = %+v / %v", manifest.Client, manifest.Features)
	}
}

// TestCompatibilityWithoutRequiredFieldIsBadResponse checks a manifest missing a field the client requires is
// refused rather than read as zero values.
func TestCompatibilityWithoutRequiredFieldIsBadResponse(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, string(testManifest(func(m map[string]any) { delete(m, "api") }))))
	_, err := h.client.Compatibility(context.Background())
	if !IsKind(err, KindBadResponse) {
		t.Fatalf("Compatibility error = %v, want KindBadResponse", err)
	}
}

// TestCompatibilityServerErrorIsReported checks a failed manifest fetch is an error, not a silent OK.
func TestCompatibilityServerErrorIsReported(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Status: http.StatusBadRequest, Body: []byte(`{"error":{"type":"bad","message":"no"}}`)})
	_, err := h.client.Compatibility(context.Background())
	if !IsKind(err, KindRejected) {
		t.Fatalf("Compatibility error = %v, want KindRejected", err)
	}
}
