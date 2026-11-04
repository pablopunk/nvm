package mygoengine

import (
	"context"
	"testing"
)

// A test binary is not stamped with a feed or a public key, so the real engine must say so and contact nothing.
func TestUnstampedBuildIsNotEnabledAndChecksFail(t *testing.T) {
	e := New()
	if e.Enabled() {
		t.Fatal("a build without a feed and key reports updates enabled")
	}
	if _, found, err := e.Check(context.Background()); found || err == nil {
		t.Fatalf("check = found %v, err %v; want a refusal", found, err)
	}
}

func TestInstallWithoutACheckRefuses(t *testing.T) {
	if err := New().Install(context.Background(), nil); err == nil {
		t.Fatal("install without a checked update must refuse")
	}
}
