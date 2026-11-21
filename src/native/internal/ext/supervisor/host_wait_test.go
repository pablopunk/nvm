package supervisor

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// slowShell answers the shell call after advancing the clock by wait, as a command that runs for that long would.
type slowShell struct {
	clock *fake.Clock
	wait  time.Duration
}

func (s slowShell) Serve(_ context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	if call.Service != protocol.ServiceDesktopShell {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeUnavailable, Message: "only the shell in this test"}}
	}
	s.clock.Advance(s.wait)
	data, _ := json.Marshal(api.ShellResult{Stdout: "ok"})
	return protocol.ServiceReply{Data: data}
}

// TestAHostWaitLongerThanTheRunBudgetDoesNotFailTheRun: the shell call takes twenty seconds of host time, and the root
// budget is 150 ms. The extension computes almost nothing, so the call completes.
func TestAHostWaitLongerThanTheRunBudgetDoesNotFailTheRun(t *testing.T) {
	clock := fake.NewClock()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	cfg := Config{
		Worker:       real.NewWorker(),
		Clock:        clock,
		Executable:   exe,
		StartTimeout: 10 * time.Second,
		Budgets:      map[string]Budget{protocol.MethodRoot: {Hard: 150 * time.Millisecond}},
	}
	r := New(cfg, api.Identity{ID: "acme.blocked", Title: "blocked"}, fixture(t, "acme.blocked"), slowShell{clock: clock, wait: 20 * time.Second})
	defer r.Stop()
	startAndWait(t, r)
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	response, err := r.Call(ctx, protocol.MethodRoot, "", nil)
	if err != nil || !strings.Contains(string(response.Data), "Shell finished: ok") {
		t.Fatalf("root = %+v, %v; want the call to finish after the host shell answered", response, err)
	}
}
