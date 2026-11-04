package real

import (
	"context"
	"testing"
)

// A test binary is not stamped with a feed or a public key, so the updater must say so and contact nothing.
func TestUnstampedBuildIsNotEnabledAndChecksFail(t *testing.T) {
	u := NewSelfUpdater()
	if u.Enabled() {
		t.Fatal("a build without a feed and key reports updates enabled")
	}
	if _, found, err := u.Check(context.Background()); found || err == nil {
		t.Fatalf("check = found %v, err %v; want a refusal", found, err)
	}
}

func TestInstallWithoutACheckRefuses(t *testing.T) {
	if err := NewSelfUpdater().Install(context.Background(), nil); err == nil {
		t.Fatal("install without a checked release must refuse")
	}
}
