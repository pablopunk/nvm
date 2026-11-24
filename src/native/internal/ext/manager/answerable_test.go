package manager

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

func TestAnExtensionRestartingAfterAFailureStillAnswersSearch(t *testing.T) {
	cases := []struct {
		status supervisor.Status
		want   bool
	}{
		{supervisor.Status{State: supervisor.StateIdle}, false},
		{supervisor.Status{State: supervisor.StateStarting}, false},
		{supervisor.Status{State: supervisor.StateStarting, Reason: "exited: panic"}, true},
		{supervisor.Status{State: supervisor.StateFailed, Reason: "exited: panic"}, true},
		{supervisor.Status{State: supervisor.StateRunning}, true},
	}
	for _, c := range cases {
		if got := isAnswerable(c.status); got != c.want {
			t.Errorf("isAnswerable(%+v) = %v, want %v", c.status, got, c.want)
		}
	}
}
