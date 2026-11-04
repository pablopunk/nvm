package quota

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestOneOwnerSharesItsCountAcrossRegistryLookups(t *testing.T) {
	registry := NewRegistry(fake.NewClock(), 2)
	if !registry.For("emoji-symbols").Allow() || !registry.For("emoji-symbols").Allow() {
		t.Fatal("the first two calls of an owner fit its minute")
	}
	if registry.For("emoji-symbols").Allow() {
		t.Fatal("a third call through another lookup of the same owner must be refused")
	}
	if !registry.For("other").Allow() {
		t.Fatal("another owner has its own count")
	}
}

func TestWindowAllowsAgainAfterAMinute(t *testing.T) {
	clock := fake.NewClock()
	window := NewWindow(clock, 1)
	if !window.Allow() || window.Allow() {
		t.Fatal("one call fits, the next does not")
	}
	clock.Advance(time.Minute)
	if !window.Allow() {
		t.Fatal("the call fits again once the minute has passed")
	}
}
