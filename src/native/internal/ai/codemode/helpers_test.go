package codemode

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ext/child"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/store"
)

// TestMain makes the test binary serve as the program child when the supervisor starts it.
func TestMain(m *testing.M) {
	if code, handled := child.Main(os.Args); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fixture is a runner over a real notes store, with a real child process per run.
type fixture struct {
	t      *testing.T
	notes  *notes.Store
	runner *Runner
	steps  *fakeSteps
	stage  string
}

type fixtureOption func(*Options)

func newFixture(t *testing.T, tweak ...fixtureOption) *fixture {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ports := real.NewPorts()
	st, err := store.Open(ports.FS, ports.Clock, t.TempDir(), store.WithDebounce(notes.WriteDelay))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	store := notes.New(st, ports.Clock, false)
	opts := Options{
		Supervisor: supervisor.Config{
			Worker: real.NewWorker(), Clock: ports.Clock, Processes: ports.Processes, Executable: exe,
			StartTimeout: 15 * time.Second, MemoryLimit: testMemoryLimit,
		},
		Files: ports.FS, StageRoot: t.TempDir(), Notes: func() *notes.Store { return store },
	}
	for _, change := range tweak {
		change(&opts)
	}
	return &fixture{t: t, notes: store, runner: NewRunner(opts), steps: &fakeSteps{}, stage: opts.StageRoot}
}

// run runs a program with the fixture's fake chat.
func (f *fixture) run(body string) agent.Result {
	f.t.Helper()
	return f.runIn(context.Background(), body)
}

func (f *fixture) runIn(ctx context.Context, body string) agent.Result {
	f.t.Helper()
	result, err := f.runner.Run(agent.WithSubCalls(ctx, f.steps), body)
	if err != nil {
		f.t.Fatalf("Run: %v", err)
	}
	return result
}

func (f *fixture) note(markdown string) notes.Note {
	f.t.Helper()
	note, err := f.notes.CreateWith(time.Now(), markdown)
	if err != nil {
		f.t.Fatal(err)
	}
	return note
}

func (f *fixture) noteIDs() []string {
	var ids []string
	for _, note := range f.notes.List() {
		ids = append(ids, note.ID)
	}
	return ids
}

func contains(t *testing.T, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

// fakeSteps stands in for the agent's nested calls: it records every row and answers each question with decide.
type fakeSteps struct {
	mu     sync.Mutex
	rows   []*fakeRow
	decide func(row *fakeRow, ctx context.Context) (bool, error)
}

type fakeRow struct {
	label, detail string
	asked         bool
	status        agent.CallStatus
	outcome       string
	ended         bool
}

func (s *fakeSteps) Begin(label, detail string) agent.SubCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := &fakeRow{label: label, detail: detail}
	s.rows = append(s.rows, row)
	return &fakeCall{steps: s, row: row}
}

func (s *fakeSteps) snapshot() []fakeRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]fakeRow, len(s.rows))
	for i, row := range s.rows {
		out[i] = *row
	}
	return out
}

type fakeCall struct {
	steps *fakeSteps
	row   *fakeRow
}

func (c *fakeCall) Ask(ctx context.Context) (bool, error) {
	c.steps.mu.Lock()
	c.row.asked = true
	decide := c.steps.decide
	c.steps.mu.Unlock()
	if decide == nil {
		return false, nil
	}
	return decide(c.row, ctx)
}

func (c *fakeCall) End(status agent.CallStatus, outcome, _ string) {
	c.steps.mu.Lock()
	defer c.steps.mu.Unlock()
	c.row.status, c.row.outcome, c.row.ended = status, outcome, true
}

func allow(*fakeRow, context.Context) (bool, error)   { return true, nil }
func decline(*fakeRow, context.Context) (bool, error) { return false, nil }

// childProcesses counts the live children of the test process. Only Linux can say, so other systems skip the check.
func childProcesses(t *testing.T) int {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("child processes are counted on Linux only")
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Skip("no /proc")
	}
	count := 0
	for _, entry := range entries {
		raw, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if err != nil {
			continue
		}
		// pid (comm) state ppid ...: comm may hold spaces, so read after the last parenthesis.
		text := string(raw)
		fields := strings.Fields(text[strings.LastIndex(text, ")")+1:])
		if len(fields) > 1 && fields[1] == strconv.Itoa(os.Getpid()) && fields[0] != "Z" {
			count++
		}
	}
	return count
}

// noChildren waits for the children of the test process to be reaped, which follows a stop within moments.
func noChildren(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for childProcesses(t) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d child processes are still running", childProcesses(t))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// noStagedPrograms fails when a run left its folder behind.
func (f *fixture) noStagedPrograms() {
	f.t.Helper()
	entries, _ := os.ReadDir(f.stage)
	if len(entries) != 0 {
		f.t.Fatalf("a run left %d staged folders behind", len(entries))
	}
}
