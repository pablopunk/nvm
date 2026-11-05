package analytics

import (
	"context"
	"net/http"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPlatformValuesMatchOneXCollectorTokens(t *testing.T) {
	tests := []struct {
		goOS, goArch, wantOS, wantArch string
	}{
		{"darwin", "amd64", "Darwin", "x64"},
		{"darwin", "arm64", "Darwin", "arm64"},
		{"linux", "386", "Linux", "ia32"},
		{"linux", "arm", "Linux", "arm"},
		{"windows", "amd64", "Windows_NT", "x64"},
		{"windows", "s390x", "Windows_NT", "s390x"},
		{"FreeBSD", "x64", "FreeBSD", "x64"},
	}
	for _, tc := range tests {
		t.Run(tc.goOS+"/"+tc.goArch, func(t *testing.T) {
			gotOS, gotArch := platformValues(tc.goOS, tc.goArch)
			if gotOS != tc.wantOS || gotArch != tc.wantArch {
				t.Fatalf("platformValues(%q, %q) = (%q, %q), want (%q, %q)", tc.goOS, tc.goArch, gotOS, gotArch, tc.wantOS, tc.wantArch)
			}
		})
	}
}

func TestSerializedCommonPropertiesNormalizePlatformAndKeepConsentPrivacy(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := New(Config{Endpoint: testEndpoint, APIKey: IngestKey, Transport: set.HTTP, FS: set.FS, Clock: set.Clock, Dir: "/config", Version: "1.8.4", OS: "windows", Arch: "amd64", Consent: true})
	c.AppLaunched()
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	event := sentBatches(t, set)[0].Batch[0]
	if event.Properties["os_type"] != "Windows_NT" || event.Properties["arch"] != "x64" {
		t.Fatalf("serialized platform metadata = (%v, %v), want (Windows_NT, x64)", event.Properties["os_type"], event.Properties["arch"])
	}
	if event.Properties["app_version"] != "1.8.4" || event.Properties["channel"] != "stable" {
		t.Fatalf("release metadata changed: app_version=%v channel=%v", event.Properties["app_version"], event.Properties["channel"])
	}
	for _, private := range []string{"email", "distinct_id", "user_id", "prompt", "clipboard"} {
		if _, exists := event.Properties[private]; exists {
			t.Fatalf("serialized metadata included private field %q", private)
		}
	}
	if !c.Consent() {
		t.Fatal("successful analytics send changed consent")
	}
}
