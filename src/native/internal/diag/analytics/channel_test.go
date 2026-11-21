package analytics

import "testing"

func TestChannelClassification(t *testing.T) {
	for _, tc := range []struct{ version, want string }{
		{"0.9.0", "stable"}, {"1.8.4", "stable"}, {"2.0.0", "stable"},
		{"2026.11.5", "prerelease"}, {"2.0.0-next.1", "prerelease"}, {"2.0.0-go", "prerelease"},
		{"unknown", "prerelease"}, {"v2.0.0", "prerelease"},
		{"1.foo", "prerelease"}, {"2.0", "prerelease"}, {"2.0.0.foo", "prerelease"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if got := Channel(tc.version); got != tc.want {
				t.Fatalf("Channel(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

func TestChannelSeparatesStableFromPrereleaseBuilds(t *testing.T) {
	cases := map[string]string{
		"0.9.0":         "stable",
		"2.0.0-go":      "prerelease",
		"2.0.0-next.0":  "prerelease",
		"2026.1030.911": "prerelease",
		"dev":           "prerelease",
		"":              "prerelease",
	}
	for version, want := range cases {
		if got := Channel(version); got != want {
			t.Errorf("Channel(%q) = %q, want %q", version, got, want)
		}
	}
}
