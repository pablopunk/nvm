package supervisor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/child"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// TestMain makes the test binary serve as the extension child when the supervisor starts it.
func TestMain(m *testing.M) {
	if code, handled := child.Main(os.Args); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// waitTimeout bounds every wait in these tests. A wait that reaches it is a failure, not a slow run.
const waitTimeout = 20 * time.Second

// callBudget is generous enough for a child under -race, and short enough that the hanging fixtures finish fast.
var callBudget = Budget{Soft: time.Second, Hard: 500 * time.Millisecond}

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "testdata", "extensions", name, "main.go"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return path
}

func newRuntime(t *testing.T, name string, tweak func(*Config)) *Runtime {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	cfg := Config{
		Worker:       real.NewWorker(),
		Clock:        real.NewPorts().Clock,
		Executable:   exe,
		StartTimeout: 10 * time.Second,
		Budgets:      map[string]Budget{protocol.MethodSearch: callBudget, protocol.MethodRoot: callBudget},
	}
	if tweak != nil {
		tweak(&cfg)
	}
	id := api.Identity{ID: "acme." + name, Title: name}
	return New(cfg, id, fixture(t, name))
}

func requireUnixProcesses(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("child process limits and process sizes are verified on Linux only")
	}
}

// waitState blocks until the runtime reaches a state, or fails the test.
func waitState(t *testing.T, r *Runtime, want State) Status {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	var last Status
	err := r.WaitFor(ctx, func(s Status) bool {
		last = s
		return s.State == want
	})
	if err != nil {
		t.Fatalf("waiting for state %s: %v (last %+v)", want, err, last)
	}
	return last
}

func callSearch(t *testing.T, r *Runtime, query string) (protocol.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	return r.Call(ctx, protocol.MethodSearch, query, nil)
}

// startAndWait starts the runtime and waits until its child answers.
func startAndWait(t *testing.T, r *Runtime) {
	t.Helper()
	r.Start()
	waitState(t, r, StateRunning)
}

func callError(t *testing.T, err error) *CallError {
	t.Helper()
	var callErr *CallError
	if !errors.As(err, &callErr) {
		t.Fatalf("error = %v (%T); want a *CallError", err, err)
	}
	return callErr
}

func TestGoodExtensionRunsAndAnswersCalls(t *testing.T) {
	r := newRuntime(t, "acme.good", nil)
	defer r.Stop()
	startAndWait(t, r)
	response, err := callSearch(t, r, "a")
	if err != nil || response.Failure != nil || !strings.Contains(string(response.Data), `"alpha"`) {
		t.Fatalf("search = %+v, %v; want the alpha item", response, err)
	}
}

func TestHangingCallIsKilledAtItsHardBudgetAndTheExtensionRecovers(t *testing.T) {
	r := newRuntime(t, "acme.spin", nil)
	defer r.Stop()
	startAndWait(t, r)
	_, err := callSearch(t, r, "x")
	if callErr := callError(t, err); callErr.Failure.Code != api.CodeTimeout {
		t.Fatalf("failure = %+v; want timeout", callErr.Failure)
	}
	status := waitState(t, r, StateFailed)
	if !strings.Contains(status.Reason, "did not finish") {
		t.Fatalf("reason = %q; want the timeout", status.Reason)
	}
	recoverAfterFailure(t, r)
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	response, err := r.Call(ctx, protocol.MethodRoot, "", nil)
	if err != nil || !strings.Contains(string(response.Data), "still works") {
		t.Fatalf("root after restart = %+v, %v; want the extension back", response, err)
	}
}

// recoverAfterFailure waits out the backoff of the last failure, then starts the extension again, as the next
// call would do.
func recoverAfterFailure(t *testing.T, r *Runtime) {
	t.Helper()
	backoff := backoffFor(r.Status().Failures)
	<-real.NewPorts().Clock.After(backoff + 50*time.Millisecond)
	r.Start()
	waitState(t, r, StateRunning)
}

func TestSlowCallRecordsADiagnosticAndTimesOut(t *testing.T) {
	r := newRuntime(t, "acme.slow", func(c *Config) {
		c.Budgets = map[string]Budget{protocol.MethodSearch: {Soft: 10 * time.Millisecond, Hard: 150 * time.Millisecond}}
	})
	defer r.Stop()
	startAndWait(t, r)
	_, err := callSearch(t, r, "x")
	if callErr := callError(t, err); callErr.Failure.Code != api.CodeTimeout {
		t.Fatalf("failure = %+v; want timeout", callErr.Failure)
	}
	status := waitState(t, r, StateFailed)
	if len(status.Diagnostics) == 0 || !strings.Contains(strings.Join(status.Diagnostics, "\n"), "took longer than") {
		t.Fatalf("diagnostics = %v; want the soft budget overrun", status.Diagnostics)
	}
}

func TestCrashingExtensionIsContainedWithItsExitReason(t *testing.T) {
	requireUnixProcesses(t)
	r := newRuntime(t, "acme.crash", nil)
	defer r.Stop()
	startAndWait(t, r)
	_, err := callSearch(t, r, "x")
	if callErr := callError(t, err); callErr.Failure.Code != api.CodeInternal {
		t.Fatalf("failure = %+v; want internal", callErr.Failure)
	}
	status := waitState(t, r, StateFailed)
	if !strings.Contains(status.Reason, "exited with code") || !strings.Contains(status.Reason, "goroutine panic") {
		t.Fatalf("reason = %q; want the exit code and the panic text", status.Reason)
	}
}

func TestRecoveredPanicLeavesTheChildRunning(t *testing.T) {
	r := newRuntime(t, "acme.panics", nil)
	defer r.Stop()
	startAndWait(t, r)
	response, err := callSearch(t, r, "boom")
	if err != nil || response.Failure == nil || !strings.HasPrefix(response.Failure.Reference, "ext-panic-") {
		t.Fatalf("boom = %+v, %v; want a child-side panic failure", response, err)
	}
	if status := r.Status(); status.State != StateRunning || status.Failures != 0 {
		t.Fatalf("status after a recovered panic = %+v; want running with no failures", status)
	}
}

func TestOversizedResultIsAFailureNotAPartialRead(t *testing.T) {
	r := newRuntime(t, "acme.huge", nil)
	defer r.Stop()
	startAndWait(t, r)
	_, err := callSearch(t, r, "x")
	callErr := callError(t, err)
	if !strings.Contains(callErr.Failure.Message, "line limit") {
		t.Fatalf("failure = %+v; want the line limit", callErr.Failure)
	}
	waitState(t, r, StateFailed)
}

func TestSlowInitDoesNotBlockTheHostPastTheStartTimeout(t *testing.T) {
	r := newRuntime(t, "acme.initspin", func(c *Config) { c.StartTimeout = 300 * time.Millisecond })
	defer r.Stop()
	r.Start()
	status := waitState(t, r, StateFailed)
	if !strings.Contains(status.Reason, "did not start within") {
		t.Fatalf("reason = %q; want the start timeout", status.Reason)
	}
}

func TestSourceThatDoesNotLoadIsBrokenAndNotRetried(t *testing.T) {
	r := newRuntime(t, "acme.syntax", nil)
	defer r.Stop()
	r.Start()
	status := waitState(t, r, StateBroken)
	if !strings.Contains(status.Reason, "compile") || status.Failures != 0 {
		t.Fatalf("status = %+v; want a compile reason and no failure count", status)
	}
	if _, err := callSearch(t, r, "x"); err == nil {
		t.Fatal("a broken extension answered a call")
	}
}

func TestRepeatedFailuresDisableTheExtensionUntilReset(t *testing.T) {
	requireUnixProcesses(t)
	r := newRuntime(t, "acme.crash", func(c *Config) { c.MaxFailures = 2 })
	defer r.Stop()
	startAndWait(t, r)
	_, _ = callSearch(t, r, "x")
	waitState(t, r, StateFailed)
	recoverAfterFailure(t, r)
	_, _ = callSearch(t, r, "x")
	status := waitState(t, r, StateDisabled)
	if !strings.Contains(status.Reason, "disabled after 2 failures") {
		t.Fatalf("reason = %q; want the disable reason", status.Reason)
	}
	r.Reset()
	if got := r.Status().State; got != StateIdle {
		t.Fatalf("state after Reset = %s; want idle", got)
	}
}

func TestStopDuringACallEndsTheCallAndTheExtensionCanStartAgain(t *testing.T) {
	r := newRuntime(t, "acme.slow", func(c *Config) {
		c.Budgets = map[string]Budget{protocol.MethodSearch: {Hard: 5 * time.Second}}
	})
	defer r.Stop()
	startAndWait(t, r)
	done := make(chan error, 1)
	go func() {
		_, err := callSearch(t, r, "x")
		done <- err
	}()
	r.Stop()
	if err := <-done; err == nil {
		t.Fatal("a call survived Stop")
	}
	if got := r.Status().State; got != StateIdle {
		t.Fatalf("state after Stop = %s; want idle", got)
	}
	startAndWait(t, r)
}

func TestCancelledContextReturnsWithoutFailingTheChild(t *testing.T) {
	r := newRuntime(t, "acme.good", nil)
	defer r.Stop()
	startAndWait(t, r)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Call(ctx, protocol.MethodSearch, "a", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Call with a cancelled context = %v; want context.Canceled", err)
	}
	if _, err := callSearch(t, r, "a"); err != nil {
		t.Fatalf("next call after a cancel = %v; want it to answer", err)
	}
}

// fakeProcesses reports the running child with one fixed resident size.
type fakeProcesses struct {
	runtime    *Runtime
	residentKB int64
}

func (f fakeProcesses) List() ([]platform.Process, error) {
	f.runtime.mu.Lock()
	child := f.runtime.child
	f.runtime.mu.Unlock()
	if child == nil {
		return nil, nil
	}
	return []platform.Process{{PID: child.proc.PID(), Name: "child", ResidentKB: f.residentKB}}, nil
}

func (f fakeProcesses) Signal(int, platform.Signal) error { return nil }

func TestResidentWatchKillsAnOversizedChild(t *testing.T) {
	r := newRuntime(t, "acme.good", func(c *Config) { c.ResidentLimit = 1 << 20 })
	r.cfg.Processes = fakeProcesses{runtime: r, residentKB: 4 << 10}
	defer r.Stop()
	startAndWait(t, r)
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	if err := r.WaitFor(ctx, func(s Status) bool { return s.State == StateFailed }); err != nil {
		t.Fatalf("resident watch did not stop the child: %v", err)
	}
	if reason := r.Status().Reason; !strings.Contains(reason, "memory limit exceeded") {
		t.Fatalf("reason = %q; want the memory limit", reason)
	}
}

func TestMemoryBombHitsTheAddressSpaceCap(t *testing.T) {
	requireUnixProcesses(t)
	r := newRuntime(t, "acme.bomb", func(c *Config) { c.MemoryLimit = 2 << 30 })
	defer r.Stop()
	startAndWait(t, r)
	if _, err := callSearch(t, r, "x"); err == nil {
		t.Fatal("the memory bomb returned a result under a 2 GiB cap")
	}
	status := waitState(t, r, StateFailed)
	if !strings.Contains(status.Reason, "exited with code") {
		t.Fatalf("reason = %q; want the child to die of memory exhaustion", status.Reason)
	}
}

func TestBackoffDoublesAndCaps(t *testing.T) {
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second}
	for i, delay := range want {
		if got := backoffFor(i + 1); got != delay {
			t.Fatalf("backoffFor(%d) = %s; want %s", i+1, got, delay)
		}
	}
	if got := backoffFor(50); got != backoffMax {
		t.Fatalf("backoffFor(50) = %s; want the %s cap", got, backoffMax)
	}
}
