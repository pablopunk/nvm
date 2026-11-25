package codemode

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeJobs is the host's scheduler: one enabled job with an error, and a record of what it was asked to do.
type fakeJobs struct {
	mu      sync.Mutex
	enabled bool
	ran     int
	cleared int
}

func (f *fakeJobs) List() []api.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	return []api.Job{{ID: "host.index", Title: "Index files", Owner: api.JobOwnerHost, Enabled: f.enabled, LastError: "disk full"}}
}

func (f *fakeJobs) Run(id string) error {
	return f.change(id, func() { f.ran++ })
}

func (f *fakeJobs) SetEnabled(id string, enabled bool) error {
	return f.change(id, func() { f.enabled = enabled })
}

func (f *fakeJobs) ClearError(id string) error {
	return f.change(id, func() { f.cleared++ })
}

func (f *fakeJobs) change(id string, apply func()) error {
	if id != "host.index" {
		return errors.New("no such job")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	apply()
	return nil
}

func (f *fakeJobs) isEnabled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enabled
}

func jobsFixture(t *testing.T) (*fixture, *fakeJobs) {
	jobs := &fakeJobs{enabled: true}
	return newFixture(t, func(opts *Options) { opts.Jobs = jobs }), jobs
}

func TestJobOperationsAreClassifiedByEffect(t *testing.T) {
	want := map[string]Class{
		protocol.OpList: ClassRead, protocol.OpSetEnabled: ClassSetting, protocol.OpRun: ClassExternal, protocol.OpClearError: ClassWrite,
	}
	rules := jobRules()
	if len(rules) != len(want) {
		t.Fatalf("the jobs table has %d rules; this test pins %d", len(rules), len(want))
	}
	for op, class := range want {
		rule, ok := rules[key{protocol.ServiceJobs, op}]
		if !ok || rule.class != class {
			t.Errorf("jobs.%s = %+v (listed %v); want %s", op, rule.class, ok, class)
		}
		if rule.label == "" || rule.describe == nil || rule.journal == nil {
			t.Errorf("jobs.%s lacks a label, a description or a journal line", op)
		}
	}
}

// Turning a job off asks with the job's title from the host, and runs once allowed.
func TestTurningAJobOffAsksWithItsTitleAndRunsWhenAllowed(t *testing.T) {
	f, jobs := jobsFixture(t)
	f.steps.decide = allow
	result := f.run(exampleBodies()["Turn off a background job (asks the user)"])
	if result.Failed || jobs.isEnabled() {
		t.Fatalf("result = %+v, enabled %v", result, jobs.isEnabled())
	}
	t.Logf("question: %s\nmodel reads: %s", lastRow(t, f, "Changing a background job").detail, result.Text)
	if row := lastRow(t, f, "Changing a background job"); !row.asked || row.detail != "Turn off the background job “Index files”" {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "turned off the background job “Index files” (the user allowed it)")
}

func TestADeclinedJobChangeChangesNothing(t *testing.T) {
	f, jobs := jobsFixture(t)
	f.steps.decide = decline
	result := f.run(`return nil, ctx.Jobs().SetEnabled("host.index", false)`)
	if !result.Failed || !jobs.isEnabled() {
		t.Fatalf("result = %+v, enabled %v", result, jobs.isEnabled())
	}
	t.Logf("model reads: %s", result.Text)
	contains(t, result.Text, "turn off the background job “Index files”: the user declined")
}

// A job the host does not list, or a change to the state it already has, fails without asking.
func TestAJobChangeTheHostRefusesFailsWithoutAsking(t *testing.T) {
	cases := map[string]struct{ body, reason string }{
		"unknown job":   {`return nil, ctx.Jobs().SetEnabled("nope", false)`, "no background job has the id"},
		"already on":    {`return nil, ctx.Jobs().SetEnabled("host.index", true)`, "is already on"},
		"unknown run":   {`return nil, ctx.Jobs().Run("nope")`, "no background job has the id"},
		"unknown clear": {`return nil, ctx.Jobs().ClearError("nope")`, "no background job has the id"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, jobs := jobsFixture(t)
			f.steps.decide = allow
			result := f.run(c.body)
			if !result.Failed || !strings.Contains(result.Text, c.reason) {
				t.Fatalf("result = %+v", result)
			}
			for _, row := range f.steps.snapshot() {
				if row.asked {
					t.Fatalf("a refused job change was put to the user: %+v", row)
				}
			}
			if !jobs.isEnabled() || jobs.ran != 0 {
				t.Fatalf("a refused change reached the scheduler")
			}
		})
	}
}

// Running a job runs extension code, so it asks; clearing its error does not.
func TestRunningAJobAsksAndClearingItsErrorDoesNot(t *testing.T) {
	f, jobs := jobsFixture(t)
	f.steps.decide = allow
	result := f.run(`if err := ctx.Jobs().ClearError("host.index"); err != nil { return nil, err }
return nil, ctx.Jobs().Run("host.index")`)
	if result.Failed || jobs.ran != 1 || jobs.cleared != 1 {
		t.Fatalf("result = %+v, ran %d, cleared %d", result, jobs.ran, jobs.cleared)
	}
	if row := lastRow(t, f, "Clearing a job error"); row.asked {
		t.Fatalf("clearing an error asked: %+v", row)
	}
	if row := lastRow(t, f, "Running a background job"); !row.asked || row.detail != "Run the background job “Index files” now" {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "cleared the last error of the background job “Index files”", "started the background job “Index files” (the user allowed it)")
}

func TestListingJobsDoesNotAsk(t *testing.T) {
	f, _ := jobsFixture(t)
	result := f.run(`return len(ctx.Jobs().List()), nil`)
	if result.Failed || !strings.Contains(result.Text, "Result: 1") {
		t.Fatalf("result = %+v", result)
	}
}
