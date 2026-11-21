package safego

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// recordingReporter keeps every failure it receives and hands back a fixed support reference.
type recordingReporter struct {
	mu       sync.Mutex
	ref      string
	failures []Failure
	signal   chan Failure
}

func newRecordingReporter(ref string) *recordingReporter {
	return &recordingReporter{ref: ref, signal: make(chan Failure, 100)}
}

func (r *recordingReporter) Report(f Failure) string {
	r.mu.Lock()
	r.failures = append(r.failures, f)
	r.mu.Unlock()
	r.signal <- f
	return r.ref
}

func (r *recordingReporter) snapshot() []Failure {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Failure(nil), r.failures...)
}

// groupWatchdog bounds how long a test waits for Group.Wait. Without the cancellation a sibling never
// returns, so the watchdog is what turns that hang into a clear failure.
const groupWatchdog = 2 * time.Second

// waitWithWatchdog returns the error of g.Wait, or fails the test when Wait does not return in time.
func waitWithWatchdog(t *testing.T, g *Group) error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- g.Wait() }()
	select {
	case err := <-result:
		return err
	case <-time.After(groupWatchdog):
		t.Fatal("Group.Wait did not return: the first failure did not cancel the other tasks")
		return nil
	}
}

// waitForGoroutines polls until the goroutine count returns to baseline. Goroutine exit cannot be
// observed through a channel, so this is the one place the tests poll real time; the bound keeps a real
// leak from hanging the suite.
func waitForGoroutines(t *testing.T, baseline int) {
	t.Helper()
	for range 2000 {
		if runtime.NumGoroutine() <= baseline {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("goroutines still running: %d, baseline %d", runtime.NumGoroutine(), baseline)
}

// waitForLive polls the process-wide watch until name has the wanted number of live goroutines.
func waitForLive(t *testing.T, name string, want int) {
	t.Helper()
	for range 2000 {
		if Watching().Live(name) == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("live goroutines for %q: %d, want %d", name, Watching().Live(name), want)
}
