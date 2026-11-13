package apps

import (
	"strings"
	"testing"
)

// Ported from app-uninstall-service.test.ts: "rejects unsafe bundle IDs before any associated path is constructed".
func TestValidateBundleIDRejectsUnsafeValues(t *testing.T) {
	long := strings.Repeat("a", MaxBundleIDBytes+1)
	for _, value := range []string{"", "com..example", "com.example/escape", "com.example. ", "com.example\napp", "com.example..", long} {
		if got, ok := ValidateBundleID(value); ok {
			t.Errorf("ValidateBundleID(%q) = %q, want refusal", value, got)
		}
	}
}

// Ported from the same case: the valid identifier passes unchanged.
func TestValidateBundleIDAcceptsSafeValue(t *testing.T) {
	if got, ok := ValidateBundleID("com.example.App"); !ok || got != "com.example.App" {
		t.Fatalf("ValidateBundleID(com.example.App) = %q, %v", got, ok)
	}
}

// Hostile identifiers beyond the 1.x cases: NUL, tab, backslash and a soft hyphen are refused.
func TestValidateBundleIDRefusesControlAndSeparatorCharacters(t *testing.T) {
	for _, value := range []string{"com.example\x00x", "com.example\tx", `com.example\x`, "com.exa\u00admple", "com.example.Ap p"} {
		if _, ok := ValidateBundleID(value); ok {
			t.Errorf("ValidateBundleID(%q) accepted", value)
		}
	}
}

// BundleIDsOverlap is the 1.x bundleIdsOverlap, exercised by the "demotes exact data" cases: equal and
// dot-prefix identifiers overlap; a longer word that only starts with the same letters does not.
func TestBundleIDsOverlap(t *testing.T) {
	cases := []struct {
		left, right string
		want        bool
	}{
		{"com.example.App", "com.example.App", true},
		{"com.example.App", "com.example.App.Helper", true},
		{"com.example.App.Helper", "com.example.App", true},
		{"com.example.App", "com.example.Appetite", false},
		{"com.example.App", "com.other.Tool", false},
		{"COM.EXAMPLE.APP", "com.example.app", true},
	}
	for _, c := range cases {
		if got := BundleIDsOverlap(c.left, c.right); got != c.want {
			t.Errorf("BundleIDsOverlap(%q, %q) = %v, want %v", c.left, c.right, got, c.want)
		}
	}
}
