package apps

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Ported from running-app-status.test.ts (4 cases). The 1.x detection was injected as detectRunningAppPaths;
// the Go service keeps that seam as Detect, so each case drives the same sequence.

func notesCandidates() []App {
	return []App{{Name: "Notes", Path: "/Applications/Notes.app"}}
}

// Ported: "returns cached running paths immediately while refreshing stale snapshots".
func TestRunningReturnsCachedPathsWhileRefreshingStaleSnapshot(t *testing.T) {
	clock := fake.NewClock()
	var detections atomic.Int32
	var changes atomic.Int32
	var mu sync.Mutex
	answers := []map[string]bool{
		{"/applications/notes.app": true},
		{"/applications/notes.app": true, "/applications/mail.app": true},
	}
	running := NewRunning(RunningOptions{
		Candidates: notesCandidates,
		Clock:      clock,
		Platform:   "darwin",
		Detect: func([]App) (map[string]bool, error) {
			mu.Lock()
			defer mu.Unlock()
			i := int(detections.Add(1)) - 1
			return answers[i], nil
		},
		Changed: func() { changes.Add(1) },
	})

	if _, err := running.Refresh("initial"); err != nil {
		t.Fatal(err)
	}
	clock.Advance(RunningTTL + 1)
	got := running.Snapshot([]string{"/Applications/Notes.app"})
	if !reflect.DeepEqual(got, []string{"/Applications/Notes.app"}) {
		t.Fatalf("Snapshot = %v, want the cached Notes", got)
	}
	running.WaitIdle()
	if detections.Load() != 2 {
		t.Fatalf("detections = %d, want 2", detections.Load())
	}
	if changes.Load() != 2 {
		t.Fatalf("changes = %d, want 2 (initial, then Mail started)", changes.Load())
	}
}

// Ported: "does not block renderer requests when no snapshot exists". The detection is gated, so the answer can be
// shown to come back before any process table was read.
func TestRunningDoesNotBlockRendererWithoutSnapshot(t *testing.T) {
	gate := make(chan struct{})
	started := make(chan struct{}, 1)
	running := NewRunning(RunningOptions{
		Candidates: notesCandidates,
		Clock:      fake.NewClock(),
		Platform:   "darwin",
		Detect: func([]App) (map[string]bool, error) {
			started <- struct{}{}
			<-gate
			return map[string]bool{"/applications/notes.app": true}, nil
		},
		Changed: func() {},
	})

	if got := running.Snapshot([]string{"/Applications/Notes.app"}); len(got) != 0 {
		t.Fatalf("Snapshot = %v, want nothing before the first detection", got)
	}
	<-started

	close(gate)
	running.WaitIdle()
	if got := running.Snapshot([]string{"/Applications/Notes.app"}); !reflect.DeepEqual(got, []string{"/Applications/Notes.app"}) {
		t.Fatalf("after detection Snapshot = %v, want Notes", got)
	}
}

// Ported: "normalizes detected running paths before storing snapshots". On macOS the comparison ignores case, and the
// caller gets back the spelling it asked for.
func TestRunningNormalizesDetectedPaths(t *testing.T) {
	running := NewRunning(RunningOptions{
		Candidates: notesCandidates,
		Clock:      fake.NewClock(),
		Platform:   "darwin",
		Detect: func([]App) (map[string]bool, error) {
			return map[string]bool{"/Applications/NOTES.app": true}, nil
		},
		Changed: func() {},
	})
	if _, err := running.Refresh("initial"); err != nil {
		t.Fatal(err)
	}
	if got := running.Snapshot([]string{"/Applications/Notes.app"}); !reflect.DeepEqual(got, []string{"/Applications/Notes.app"}) {
		t.Fatalf("Snapshot = %v, want the requested spelling", got)
	}
}

// Ported: "dedupes in-flight refreshes and caps renderer requested paths".
func TestRunningDedupesInFlightRefreshesAndCapsRequests(t *testing.T) {
	var detections atomic.Int32
	running := NewRunning(RunningOptions{
		Candidates: func() []App { return []App{{Name: "app", Path: "/usr/bin/app"}} },
		Clock:      fake.NewClock(),
		Platform:   "linux",
		Detect: func([]App) (map[string]bool, error) {
			detections.Add(1)
			return map[string]bool{"/usr/bin/app": true}, nil
		},
		Changed: func() {},
	})

	running.ScheduleRefresh("one")
	running.ScheduleRefresh("two")
	if _, err := running.Refresh("three"); err != nil {
		t.Fatal(err)
	}
	running.WaitIdle()
	if detections.Load() != 1 {
		t.Fatalf("detections = %d, want 1", detections.Load())
	}
	requested := make([]string, 35)
	for i := range requested {
		requested[i] = "/tmp/" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	requested[0] = "/usr/bin/app"
	if got := running.Snapshot(requested); !reflect.DeepEqual(got, []string{"/usr/bin/app"}) {
		t.Fatalf("Snapshot = %v, want [/usr/bin/app]", got)
	}
}

// Hostile input beyond 1.x: a request past the cap is not answered even when its app is running. Only the first 30
// distinct requests count.
func TestRunningIgnoresRequestsPastTheCap(t *testing.T) {
	running := NewRunning(RunningOptions{
		Candidates: func() []App { return []App{{Name: "late", Path: "/opt/late.app"}} },
		Clock:      fake.NewClock(),
		Platform:   "darwin",
		Detect: func([]App) (map[string]bool, error) {
			return map[string]bool{"/opt/late.app": true}, nil
		},
		Changed: func() {},
	})
	if _, err := running.Refresh("initial"); err != nil {
		t.Fatal(err)
	}
	requested := make([]string, 0, 31)
	for i := 0; i < maxRequestedRunning; i++ {
		requested = append(requested, "/nothing/"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	requested = append(requested, "/opt/late.app")
	if got := running.Snapshot(requested); len(got) != 0 {
		t.Fatalf("Snapshot answered past the cap: %v", got)
	}
}

// Hostile input: without the process port (macOS today) the service reports ErrUnsupported and shows nothing running.
func TestRunningWithoutProcessPortReportsUnsupported(t *testing.T) {
	running := NewRunning(RunningOptions{Candidates: notesCandidates, Clock: fake.NewClock(), Platform: "darwin"})
	if _, err := running.Refresh("initial"); err == nil {
		t.Fatal("Refresh without a process port returned nil")
	}
	if got := running.Snapshot([]string{"/Applications/Notes.app"}); len(got) != 0 {
		t.Fatalf("Snapshot = %v, want nothing", got)
	}
}

// Process matching on Linux: a desktop entry's program is matched against the process table's names, and a
// .desktop file that changed is parsed again.
func TestRunningMatchesDesktopEntriesAgainstProcessNames(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	const desktop = "/home/u/.local/share/applications/example.desktop"
	files.Seed(desktop, []byte("[Desktop Entry]\nName=Example\nExec=/opt/example/bin/example-app %U\n"))
	stopped := NewRunning(RunningOptions{
		Candidates: func() []App { return []App{{Name: "Example", Path: desktop}} },
		Clock:      clock,
		Processes:  fake.NewProcesses(platform.Process{PID: 7, Name: "bash"}),
		Files:      files,
		Platform:   "linux",
	})
	if _, err := stopped.Refresh("initial"); err != nil {
		t.Fatal(err)
	}
	if got := stopped.Snapshot([]string{desktop}); len(got) != 0 {
		t.Fatalf("stopped app reported running: %v", got)
	}

	running := NewRunning(RunningOptions{
		Candidates: func() []App { return []App{{Name: "Example", Path: desktop}} },
		Clock:      clock,
		Processes:  fake.NewProcesses(platform.Process{PID: 42, Name: "example-app"}),
		Files:      files,
		Platform:   "linux",
	})
	if _, err := running.Refresh("initial"); err != nil {
		t.Fatal(err)
	}
	if got := running.Snapshot([]string{desktop}); !reflect.DeepEqual(got, []string{desktop}) {
		t.Fatalf("Snapshot = %v, want the example app running", got)
	}
}
