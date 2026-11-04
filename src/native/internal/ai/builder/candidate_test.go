package builder

import (
	"strings"
	"testing"
)

const clockManifest = `{"apiVersion":"1.0","id":"user.clock","title":"Clock"}`

// TestNewCandidateAcceptsAValidManifestAndSource checks a candidate keeps the manifest text, main.go and its extra files.
func TestNewCandidateAcceptsAValidManifestAndSource(t *testing.T) {
	candidate, err := NewCandidate(clockManifest, "package main", map[string]string{"helpers.go": "package main"})
	if err != nil {
		t.Fatalf("NewCandidate: %v", err)
	}
	if candidate.Manifest.ID != "user.clock" || candidate.ManifestText != clockManifest || len(candidate.Files()) != 3 {
		t.Fatalf("candidate = %+v, want the manifest, main.go and helpers.go", candidate)
	}
}

// TestNewCandidateRefusesWhatTheHostWouldRefuse checks the arguments are checked before the extension is built: a
// manifest that is not JSON or not valid, an empty source, and an extra file that is not a plain .go name.
func TestNewCandidateRefusesWhatTheHostWouldRefuse(t *testing.T) {
	cases := map[string]struct {
		manifest, source string
		extras           map[string]string
		want             string
	}{
		"not json":         {"{", "package main", nil, "not valid JSON"},
		"invalid manifest": {`{"apiVersion":"1.0","id":"nevermind.clock","title":"Clock"}`, "package main", nil, "invalid"},
		"empty source":     {clockManifest, "  ", nil, "source is empty"},
		"bad extra name":   {clockManifest, "package main", map[string]string{"../x.go": "package main"}, "plain .go name"},
		"test file":        {clockManifest, "package main", map[string]string{"a_test.go": "package main"}, "plain .go name"},
	}
	for name, tc := range cases {
		_, err := NewCandidate(tc.manifest, tc.source, tc.extras)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, tc.want)
		}
	}
}
