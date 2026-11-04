package extjobs

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
	"github.com/pablopunk/nvm/native/internal/safego"
)

const (
	extensionID = "acme.tasks"
	watchdog    = 10 * time.Second
)

var (
	watchedRoot = testpath.Abs("watched")
	homeFolder  = testpath.Abs("home", "acme")
)

// fakeHost answers Commands with what the test set, and records every invocation it is asked to run.
type fakeHost struct {
	mu          sync.Mutex
	commands    []manager.CommandResult
	failure     *api.Failure
	invocations chan api.Invocation
}

func (h *fakeHost) Commands(context.Context) []manager.CommandResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]manager.CommandResult(nil), h.commands...)
}

func (h *fakeHost) setCommands(results ...manager.CommandResult) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.commands = results
}

func (h *fakeHost) Run(_ context.Context, _ string, invocation api.Invocation) (api.Result, *api.Failure, error) {
	h.invocations <- invocation
	h.mu.Lock()
	defer h.mu.Unlock()
	return api.Result{}, h.failure, nil
}

// rig is an adapter over a real scheduler on a fake clock, with a fake file system for the folder triggers.
type rig struct {
	t       *testing.T
	clock   *fake.Clock
	files   *fake.FS
	sched   *jobs.Scheduler
	host    *fakeHost
	adapter *Adapter
}

func newRig(t *testing.T) *rig {
	t.Helper()
	clock := fake.NewClock()
	files := fake.NewFS()
	sched, err := jobs.New(jobs.Config{Clock: clock, Reporter: safego.ReporterFunc(func(safego.Failure) string { return "" })})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{invocations: make(chan api.Invocation, 16)}
	adapter := New(context.Background(), Config{
		Host:     func() Host { return host },
		Registry: sched,
		Files:    files,
		Home:     homeFolder,
	})
	t.Cleanup(func() {
		adapter.Close()
		_ = sched.Close()
	})
	return &rig{t: t, clock: clock, files: files, sched: sched, host: host, adapter: adapter}
}

// sync runs one reconcile of the running extensions, as the sync job does.
func (r *rig) sync(commands ...api.Command) {
	r.t.Helper()
	r.host.setCommands(manager.CommandResult{ExtensionID: extensionID, Title: "Tasks", Commands: commands})
	if err := r.adapter.reconcile(context.Background()); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) job(id string) (jobs.Snapshot, bool) {
	for _, snap := range r.sched.Snapshot() {
		if snap.ID == id {
			return snap, true
		}
	}
	return jobs.Snapshot{}, false
}

// nextInvocation waits for the next run the host receives, bounded by the watchdog.
func (r *rig) nextInvocation() api.Invocation {
	r.t.Helper()
	select {
	case invocation := <-r.host.invocations:
		return invocation
	case <-time.After(watchdog):
		r.t.Fatal("no invocation reached the host")
		return api.Invocation{}
	}
}

func (r *rig) noInvocation() {
	r.t.Helper()
	select {
	case invocation := <-r.host.invocations:
		r.t.Fatalf("unexpected invocation %+v", invocation)
	default:
	}
}

func viewCommand(id string, triggers ...api.Trigger) api.Command {
	return api.Command{ID: id, Title: strings.ToUpper(id[:1]) + id[1:], Triggers: triggers}
}

func backgroundCommand(id string, triggers ...api.Trigger) api.Command {
	command := viewCommand(id, triggers...)
	command.Mode = api.ModeBackground
	return command
}

func TestOnlyCommandsWithTriggersOrABackgroundModeBecomeJobs(t *testing.T) {
	r := newRig(t)
	r.sync(
		viewCommand("palette"),
		backgroundCommand("manual"),
		viewCommand("scheduled", api.Trigger{Type: api.TriggerStartup}),
	)
	if _, ok := r.job(JobPrefix + extensionID + ".palette"); ok {
		t.Fatal("a view command without triggers became a job")
	}
	for _, id := range []string{"manual", "scheduled"} {
		snap, ok := r.job(JobPrefix + extensionID + "." + id)
		if !ok || snap.Owner != jobs.OwnerExtension || snap.Scope != extensionID {
			t.Fatalf("job %s = %+v, %v", id, snap, ok)
		}
	}
}

func TestIntervalTriggerRunsTheCommandWithItsLaunchContext(t *testing.T) {
	r := newRig(t)
	r.sync(viewCommand("tick", api.Trigger{Type: api.TriggerInterval, Every: "30s"}))
	r.clock.AwaitTimerDueIn(30 * time.Second)
	r.clock.Advance(30 * time.Second)

	invocation := r.nextInvocation()
	if invocation.Ref.ID != "tick" || invocation.Launch.Reason != "interval" || invocation.Launch.Trigger == nil || invocation.Launch.Trigger.Type != api.TriggerInterval {
		t.Fatalf("invocation = %+v", invocation)
	}
}

func TestManualRunOfABackgroundCommandReachesTheHost(t *testing.T) {
	r := newRig(t)
	r.sync(backgroundCommand("refresh"))
	if err := r.sched.RunNow(JobPrefix + extensionID + ".refresh"); err != nil {
		t.Fatal(err)
	}
	if invocation := r.nextInvocation(); invocation.Ref.ID != "refresh" || invocation.Launch.Reason != "manual" {
		t.Fatalf("invocation = %+v", invocation)
	}
}

func TestClipboardEventRunsTheCommand(t *testing.T) {
	r := newRig(t)
	r.sync(viewCommand("copied", api.Trigger{Type: api.TriggerClipboardChanged}))
	r.sched.Emit(ClipboardChangedEvent, nil)

	invocation := r.nextInvocation()
	if invocation.Launch.Event != ClipboardChangedEvent || invocation.Launch.Reason != "event:clipboard.changed" {
		t.Fatalf("launch = %+v", invocation.Launch)
	}
}

func TestExtensionFailureFailsTheJobWithItsMessage(t *testing.T) {
	r := newRig(t)
	r.host.failure = &api.Failure{Code: api.CodeInternal, Message: "the feed is down"}
	r.sync(backgroundCommand("fetch"))
	id := JobPrefix + extensionID + ".fetch"
	if err := r.sched.RunNow(id); err != nil {
		t.Fatal(err)
	}
	r.nextInvocation()
	changes, cancel := r.sched.Subscribe()
	defer cancel()
	deadline := time.After(watchdog)
	for {
		snap, _ := r.job(id)
		if snap.Status == jobs.StatusFailed {
			if snap.LastError != "the feed is down" {
				t.Fatalf("last error = %q", snap.LastError)
			}
			return
		}
		select {
		case <-changes:
		case <-deadline:
			t.Fatalf("job never failed: %+v", snap)
		}
	}
}

func TestStoppedExtensionLosesItsJobsAndAnUnansweredOneKeepsThem(t *testing.T) {
	r := newRig(t)
	r.sync(backgroundCommand("kept"), backgroundCommand("dropped"))
	r.host.setCommands(manager.CommandResult{ExtensionID: extensionID, Failure: &api.Failure{Code: api.CodeTimeout}})
	if err := r.adapter.reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.job(JobPrefix + extensionID + ".kept"); !ok {
		t.Fatal("an extension that did not answer lost its job")
	}

	r.host.setCommands()
	if err := r.adapter.reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.sched.Snapshot()) != 0 {
		t.Fatalf("jobs of a stopped extension remain: %+v", r.sched.Snapshot())
	}
}

func TestUnsupportedTriggerIsSkippedAndTheRestOfTheCommandStillRuns(t *testing.T) {
	r := newRig(t)
	r.sync(viewCommand("mixed",
		api.Trigger{Type: api.TriggerType("device.shaken")},
		api.Trigger{Type: api.TriggerStartup},
	))
	snap, ok := r.job(JobPrefix + extensionID + ".mixed")
	if !ok || len(snap.Triggers) != 1 || snap.Triggers[0].Kind != jobs.TriggerStartup {
		t.Fatalf("job triggers = %+v, %v", snap.Triggers, ok)
	}
}

func TestFileTriggerWatchesItsRootAndPassesOnlyMatchingFiles(t *testing.T) {
	r := newRig(t)
	if err := r.files.MkdirAll(watchedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	r.sync(viewCommand("shots", api.Trigger{
		Type:       api.TriggerFilesChanged,
		Roots:      []string{watchedRoot},
		Extensions: []string{".PNG"},
	}))
	// Ignored writes come first; the watcher delivers in order, so a wrong pass would show up before the match.
	mustWrite(t, r.files, filepath.Join(watchedRoot, "notes.txt"))
	mustWrite(t, r.files, filepath.Join(watchedRoot, ".secret.png"))
	mustWrite(t, r.files, filepath.Join(watchedRoot, "shot.png"))

	invocation := r.nextInvocation()
	if len(invocation.Launch.ChangedPaths) != 1 || invocation.Launch.ChangedPaths[0] != filepath.Join(watchedRoot, "shot.png") {
		t.Fatalf("changed paths = %v", invocation.Launch.ChangedPaths)
	}
	if len(invocation.Launch.Files) != 1 || invocation.Launch.Files[0].Kind != "image" || invocation.Launch.Event != "files.changed" {
		t.Fatalf("launch = %+v", invocation.Launch)
	}
	r.noInvocation()
}

func TestLoginIsSentOnceWhenALoginCommandFirstAppears(t *testing.T) {
	r := newRig(t)
	r.sync(viewCommand("hello", api.Trigger{Type: api.TriggerLogin}))
	r.nextInvocation()
	r.sync(viewCommand("hello", api.Trigger{Type: api.TriggerLogin}))
	r.noInvocation()
}

func TestSyncJobDoesNothingWhileExtensionsLoad(t *testing.T) {
	clock := fake.NewClock()
	sched, err := jobs.New(jobs.Config{Clock: clock, Reporter: safego.ReporterFunc(func(safego.Failure) string { return "" })})
	if err != nil {
		t.Fatal(err)
	}
	defer sched.Close()
	adapter := New(context.Background(), Config{Host: func() Host { return nil }, Registry: sched, Files: fake.NewFS()})
	defer adapter.Close()
	if err := adapter.reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sched.Snapshot()) != 0 {
		t.Fatal("jobs appeared before the extensions loaded")
	}
}

func mustWrite(t *testing.T, files *fake.FS, path string) {
	t.Helper()
	if err := files.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
