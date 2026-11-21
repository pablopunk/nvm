package extjobs

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

// recordingRegistry keeps what the adapter registers, removes and emits, without running any job.
type recordingRegistry struct {
	mu           sync.Mutex
	registered   []string
	unregistered []string
	emitted      []string
}

func (r *recordingRegistry) Register(def jobs.Definition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registered = append(r.registered, def.ID)
	return nil
}

func (r *recordingRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unregistered = append(r.unregistered, id)
}

func (r *recordingRegistry) Snapshot() []jobs.Snapshot { return nil }

func (r *recordingRegistry) Emit(event string, _ jobs.Payload) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.emitted = append(r.emitted, event)
}

// frontmostFake answers the front app the test sets, or the error it sets.
type frontmostFake struct {
	app string
	err error
}

func (f *frontmostFake) read(context.Context) (string, error) { return f.app, f.err }

func newFrontmostAdapter(reg *recordingRegistry, front *frontmostFake) *Adapter {
	return New(context.Background(), Config{Registry: reg, Frontmost: front.read})
}

func TestPollEmitsOnceForEachChangeOfFrontApp(t *testing.T) {
	reg := &recordingRegistry{}
	front := &frontmostFake{app: "com.apple.Notes"}
	adapter := newFrontmostAdapter(reg, front)
	ctx := context.Background()

	for _, app := range []string{"com.apple.Notes", "com.apple.Notes", "com.apple.Safari", "com.apple.Safari", "com.apple.Notes"} {
		front.app = app
		if err := adapter.pollFrontmost(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(reg.emitted, []string{FrontmostChangedEvent, FrontmostChangedEvent, FrontmostChangedEvent}) {
		t.Fatalf("emitted %v, want one event per change", reg.emitted)
	}
}

func TestFailedOrEmptyReadEmitsNothingAndFailsNoJob(t *testing.T) {
	reg := &recordingRegistry{}
	front := &frontmostFake{err: errors.New("busy")}
	adapter := newFrontmostAdapter(reg, front)

	if err := adapter.pollFrontmost(context.Background()); err != nil {
		t.Fatalf("a failed read failed the job: %v", err)
	}
	front.err, front.app = nil, ""
	if err := adapter.pollFrontmost(context.Background()); err != nil || len(reg.emitted) != 0 {
		t.Fatalf("empty read: err %v, emitted %v", err, reg.emitted)
	}
}

func TestPollRunsOnlyWhileACommandListensForFrontmostChanges(t *testing.T) {
	reg := &recordingRegistry{}
	adapter := newFrontmostAdapter(reg, &frontmostFake{app: "x"})

	adapter.syncFrontmostPoll(true)
	adapter.syncFrontmostPoll(true)
	if !slices.Equal(reg.registered, []string{FrontmostPollJobID}) {
		t.Fatalf("registered %v, want the poll once", reg.registered)
	}
	adapter.syncFrontmostPoll(false)
	adapter.syncFrontmostPoll(false)
	if !slices.Equal(reg.unregistered, []string{FrontmostPollJobID}) {
		t.Fatalf("unregistered %v, want the poll once", reg.unregistered)
	}
}

func TestNoPollWithoutAPlatformReader(t *testing.T) {
	reg := &recordingRegistry{}
	adapter := New(context.Background(), Config{Registry: reg})
	adapter.syncFrontmostPoll(true)
	if len(reg.registered) != 0 {
		t.Fatalf("registered %v without a reader", reg.registered)
	}
}

func TestFrontmostTriggerMapsToTheChangeEvent(t *testing.T) {
	command := viewCommand("switched", api.Trigger{Type: api.TriggerAppFrontmostChange})
	p, ok := planFor(extensionID, command, "/home/acme")
	if !ok || len(p.def.Triggers) != 1 || p.def.Triggers[0].Event != FrontmostChangedEvent {
		t.Fatalf("plan = %+v, ok %v", p.def.Triggers, ok)
	}
	if !declaresFrontmost(command) {
		t.Fatal("the command does not declare the frontmost trigger")
	}
}
