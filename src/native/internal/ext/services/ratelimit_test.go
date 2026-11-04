package services

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestRateLimiterBurstsThenRefillsOverTime(t *testing.T) {
	clock := fake.NewClock()
	limiter := newRateLimiter(clock, 2)
	if !limiter.allow() || !limiter.allow() {
		t.Fatal("a fresh bucket refused its first two calls")
	}
	if limiter.allow() {
		t.Fatal("an empty bucket allowed a call")
	}
	clock.Advance(30 * time.Second)
	if !limiter.allow() {
		t.Fatal("half a minute did not refill one token of two per minute")
	}
	if limiter.allow() {
		t.Fatal("half a minute refilled more than one token")
	}
	clock.Advance(10 * time.Minute)
	if !limiter.allow() || !limiter.allow() || limiter.allow() {
		t.Fatal("refill is not capped at the bucket size")
	}
}
