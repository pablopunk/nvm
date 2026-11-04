package apps

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The quit logic has no 1.x unit tests (the 1.x quit was an AppleScript and a shell loop). These tests pin the
// rules the plan sets: TERM then KILL after the grace period on the Clock, KILL at once for Force Quit, never
// signalling this process, process 0 or init, and signalling single PIDs only.

// stubbornProcesses is a process table where some processes ignore SIGTERM. It records every signal in order.
type stubbornProcesses struct {
	mu      sync.Mutex
	table   map[int]platform.Process
	ignore  map[int]bool
	signals []fake.SignalCall
}

func newStubbornProcesses(list ...platform.Process) *stubbornProcesses {
	s := &stubbornProcesses{table: map[int]platform.Process{}, ignore: map[int]bool{}}
	for _, p := range list {
		s.table[p.PID] = p
	}
	return s
}

func (s *stubbornProcesses) List() ([]platform.Process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []platform.Process
	for _, p := range s.table {
		out = append(out, p)
	}
	return out, nil
}

func (s *stubbornProcesses) Signal(pid int, sig platform.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals = append(s.signals, fake.SignalCall{PID: pid, Signal: sig})
	if _, ok := s.table[pid]; !ok {
		return platform.ErrNotFound
	}
	if sig == platform.SignalKill || (sig == platform.SignalTerminate && !s.ignore[pid]) {
		delete(s.table, pid)
	}
	return nil
}

func (s *stubbornProcesses) recorded() []fake.SignalCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]fake.SignalCall(nil), s.signals...)
}

// Pure selection: process 0, process 1 and this process are never picked, and names match case-insensitively, also on
// the kernel's 15-byte name.
func TestSignalablePIDsExcludesSystemAndSelf(t *testing.T) {
	list := []platform.Process{
		{PID: 0, Name: "sched"},
		{PID: 1, Name: "example"},
		{PID: 42, Name: "Example"},
		{PID: 43, Name: "example-helper"},
		{PID: 99, Name: "nevermind"},
		{PID: 100, Name: "my-long-applica"},
	}
	if got, want := SignalablePIDs(list, []string{"example"}, 99), []int{42}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pids = %v, want %v", got, want)
	}
	if got, want := SignalablePIDs(list, []string{"my-long-application-name"}, 99), []int{100}; !reflect.DeepEqual(got, want) {
		t.Fatalf("truncated name pids = %v, want %v", got, want)
	}
	if got := SignalablePIDs(list, []string{"nevermind"}, 99); len(got) != 0 {
		t.Fatalf("own process selected: %v", got)
	}
	if got := SignalablePIDs(list, []string{"", "  "}, 99); len(got) != 0 {
		t.Fatalf("blank names selected: %v", got)
	}
}

// Force Quit sends SIGKILL to the matching processes at once and to nothing else.
func TestForceQuitKillsOnlyTheAppsProcesses(t *testing.T) {
	procs := fake.NewProcesses(
		platform.Process{PID: 10, Name: "example"},
		platform.Process{PID: 11, Name: "editor"},
	)
	q := &Quitter{Processes: procs, Clock: fake.NewClock(), Self: 999}
	out, err := q.ForceQuit([]string{"example"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Found != 1 || out.Killed != 1 {
		t.Fatalf("outcome = %+v", out)
	}
	if want := []fake.SignalCall{{PID: 10, Signal: platform.SignalKill}}; !reflect.DeepEqual(procs.Signals(), want) {
		t.Fatalf("signals = %+v, want %+v", procs.Signals(), want)
	}
}

// Quit sends SIGTERM and returns once the process has exited, without SIGKILL.
func TestQuitTerminatesAProcessThatExits(t *testing.T) {
	procs := fake.NewProcesses(platform.Process{PID: 10, Name: "example"})
	q := &Quitter{Processes: procs, Clock: fake.NewClock(), Self: 999, Grace: time.Second, Poll: 100 * time.Millisecond}
	out, err := q.Quit([]string{"example"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Terminated != 1 || out.Killed != 0 {
		t.Fatalf("outcome = %+v", out)
	}
	if want := []fake.SignalCall{{PID: 10, Signal: platform.SignalTerminate}}; !reflect.DeepEqual(procs.Signals(), want) {
		t.Fatalf("signals = %+v, want %+v", procs.Signals(), want)
	}
}

// Quit escalates: a process that ignores SIGTERM is sent SIGKILL after the grace period, measured on the fake clock.
func TestQuitEscalatesToKillAfterTheGracePeriod(t *testing.T) {
	procs := newStubbornProcesses(platform.Process{PID: 10, Name: "example"})
	procs.ignore[10] = true
	clock := fake.NewClock()
	q := &Quitter{Processes: procs, Clock: clock, Self: 999, Grace: time.Second, Poll: 250 * time.Millisecond}
	type result struct {
		out QuitOutcome
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := q.Quit([]string{"example"})
		done <- result{out, err}
	}()
	for i := 0; i < 4; i++ {
		clock.AwaitTimers(1)
		clock.Advance(250 * time.Millisecond)
	}
	r := <-done
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.out.Killed != 1 {
		t.Fatalf("outcome = %+v, want one kill", r.out)
	}
	want := []fake.SignalCall{{PID: 10, Signal: platform.SignalTerminate}, {PID: 10, Signal: platform.SignalKill}}
	if got := procs.recorded(); !reflect.DeepEqual(got, want) {
		t.Fatalf("signals = %+v, want %+v", got, want)
	}
}

// A process that disappears between the list and the signal is not a failure.
func TestQuitIgnoresProcessesThatAlreadyExited(t *testing.T) {
	procs := fake.NewProcesses(platform.Process{PID: 10, Name: "example"})
	procs.FailNext(fake.OpProcessesSignal, platform.ErrNotFound)
	q := &Quitter{Processes: procs, Clock: fake.NewClock(), Self: 999}
	out, err := q.ForceQuit([]string{"example"})
	if err != nil || out.Failed != 0 {
		t.Fatalf("outcome = %+v, err = %v", out, err)
	}
}

// An OS refusal to signal is counted as a failure, so the result can say so.
func TestQuitCountsRefusedSignals(t *testing.T) {
	procs := fake.NewProcesses(platform.Process{PID: 10, Name: "example"})
	procs.FailNext(fake.OpProcessesSignal, errors.New("operation not permitted"))
	q := &Quitter{Processes: procs, Clock: fake.NewClock(), Self: 999}
	out, _ := q.ForceQuit([]string{"example"})
	if out.Failed != 1 {
		t.Fatalf("outcome = %+v, want one failure", out)
	}
}

// Without the Processes port (macOS today) quitting is unsupported and nothing is signalled.
func TestQuitWithoutProcessPortIsUnsupported(t *testing.T) {
	q := &Quitter{Clock: fake.NewClock(), Self: 999}
	if _, err := q.ForceQuit([]string{"example"}); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("err = %v, want unsupported", err)
	}
}

// Quit All leaves Nevermind, the desktop shell and the file manager's desktop alone.
func TestQuitAllExceptionsAreNotQuit(t *testing.T) {
	for _, name := range []string{"gnome-shell", "Finder", "xfdesktop"} {
		if !IsExcludedFromQuitAll(name) {
			t.Errorf("%s is not excepted", name)
		}
	}
	if IsExcludedFromQuitAll("Notes") {
		t.Error("Notes is excepted")
	}
}
