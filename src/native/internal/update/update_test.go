package update

import (
	"context"
	"testing"
)

func TestNoUpdatesReportsUnsupportedWithoutContactingAnything(t *testing.T) {
	u := NoUpdates{Version: "2.0.0-go"}
	for name, state := range map[string]State{"state": u.State(), "check": u.Check(context.Background())} {
		if state.Status != StatusUnsupported || state.CurrentVersion != "2.0.0-go" {
			t.Errorf("%s = %+v, want unsupported at the running version", name, state)
		}
	}
}

func TestMessageNamesTheVersionOrTheReason(t *testing.T) {
	cases := []struct {
		name  string
		state State
		want  string
	}{
		{"up to date", State{Status: StatusIdle}, "Nevermind is up to date"},
		{"available", State{Status: StatusAvailable, LatestVersion: "2.1.0"}, "Nevermind 2.1.0 is available"},
		{"failed", State{Status: StatusFailed, Message: "offline"}, "Update check failed: offline"},
		{"unsupported", State{Status: StatusUnsupported}, "Updates are not available in this build yet"},
	}
	for _, c := range cases {
		if got := Message(c.state); got != c.want {
			t.Errorf("%s: Message = %q, want %q", c.name, got, c.want)
		}
	}
}
