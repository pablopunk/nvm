package testpath

import (
	"path/filepath"
	"testing"
)

// TestAbsIsAbsoluteOnTheRunningOS is the reason the package exists: every OS must accept what Abs returns.
func TestAbsIsAbsoluteOnTheRunningOS(t *testing.T) {
	for _, p := range []string{Abs(), Abs("home", "me"), Abs("cfg", "nvm", "settings.json")} {
		if !filepath.IsAbs(p) {
			t.Fatalf("Abs returned %q, which is not absolute on this OS", p)
		}
	}
}

// TestAbsKeepsTheRootAndTheElements checks that the elements land below the root in native separators.
func TestAbsKeepsTheRootAndTheElements(t *testing.T) {
	want := filepath.Join(Root(), "home", "me")
	if got := Abs("home", "me"); got != want {
		t.Fatalf("Abs(home, me) = %q, want %q", got, want)
	}
	if got := filepath.Dir(Abs("home", "me")); got != filepath.Join(Root(), "home") {
		t.Fatalf("parent of Abs(home, me) = %q", got)
	}
}
