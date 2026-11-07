package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestRefreshGetsBoundedTimeWithoutExtendingForegroundActions(t *testing.T) {
	if got := runTimeoutOf(api.Invocation{}); got != extensionRunTimeout {
		t.Fatalf("ordinary action timeout %v", got)
	}
	if got := runTimeoutOf(api.Invocation{Launch: api.LaunchContext{Refresh: true}}); got != extensionRefreshTimeout || got <= extensionRunTimeout {
		t.Fatalf("refresh timeout %v", got)
	}
}
