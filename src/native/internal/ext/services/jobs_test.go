package services

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeJobs is the host's background jobs: one job, and the calls the controls received.
type fakeJobs struct {
	ran     []string
	enabled map[string]bool
	cleared []string
	refusal error
}

func (f *fakeJobs) List() []api.Job {
	return []api.Job{{ID: "extension.acme.feed.fetch", Title: "Fetch Feed", Owner: api.JobOwnerExtension, Status: api.JobStatusIdle}}
}

func (f *fakeJobs) Run(id string) error {
	f.ran = append(f.ran, id)
	return f.refusal
}

func (f *fakeJobs) SetEnabled(id string, enabled bool) error {
	if f.enabled == nil {
		f.enabled = map[string]bool{}
	}
	f.enabled[id] = enabled
	return f.refusal
}

func (f *fakeJobs) ClearError(id string) error {
	f.cleared = append(f.cleared, id)
	return f.refusal
}

func jobsHandler(t *testing.T, surface JobsSurface, capabilities ...api.Capability) *Handler {
	t.Helper()
	_, deps := newDesktopHost(t)
	deps.Jobs = surface
	return desktopHandler(t, deps, capabilities...)
}

func TestJobsAreListedWithoutADeclaration(t *testing.T) {
	h := jobsHandler(t, &fakeJobs{})
	var got []api.Job
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceJobs, protocol.OpList, nil)), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ID != "extension.acme.feed.fetch" || got[0].Owner != api.JobOwnerExtension {
		t.Fatalf("jobs = %+v", got)
	}
}

func TestChangingAJobNeedsTheJobsDeclaration(t *testing.T) {
	surface := &fakeJobs{}
	h := jobsHandler(t, surface)
	requireCode(t, serve(t, h, protocol.ServiceJobs, protocol.OpRun, protocol.JobArgs{ID: "x"}), api.CodePermission, "jobs")
	requireCode(t, serve(t, h, protocol.ServiceJobs, protocol.OpClearError, protocol.JobArgs{ID: "x"}), api.CodePermission, "jobs")
	if len(surface.ran) != 0 || len(surface.cleared) != 0 {
		t.Fatalf("an undeclared call reached the host: ran %v, cleared %v", surface.ran, surface.cleared)
	}
}

func TestRunEnableAndClearReachTheHostWithTheDeclaration(t *testing.T) {
	surface := &fakeJobs{}
	h := jobsHandler(t, surface, api.CapabilityJobs)
	mustOK(t, serve(t, h, protocol.ServiceJobs, protocol.OpRun, protocol.JobArgs{ID: "extension.acme.feed.fetch"}))
	mustOK(t, serve(t, h, protocol.ServiceJobs, protocol.OpSetEnabled, protocol.JobEnabledArgs{ID: "extension.acme.feed.fetch", Enabled: false}))
	mustOK(t, serve(t, h, protocol.ServiceJobs, protocol.OpClearError, protocol.JobArgs{ID: "extension.acme.feed.fetch"}))
	if len(surface.ran) != 1 || surface.enabled["extension.acme.feed.fetch"] || len(surface.cleared) != 1 {
		t.Fatalf("ran %v, enabled %v, cleared %v", surface.ran, surface.enabled, surface.cleared)
	}
}

func TestAHostRefusalReachesTheExtensionAsItsReason(t *testing.T) {
	h := jobsHandler(t, &fakeJobs{refusal: errors.New("jobs: the job is disabled")}, api.CapabilityJobs)
	reply := serve(t, h, protocol.ServiceJobs, protocol.OpRun, protocol.JobArgs{ID: "x"})
	if reply.Failure == nil || reply.Failure.Message != "jobs: the job is disabled" {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestAJobNeedsAnIDAndAnUnlistedOperationIsRefused(t *testing.T) {
	h := jobsHandler(t, &fakeJobs{}, api.CapabilityJobs)
	requireCode(t, serve(t, h, protocol.ServiceJobs, protocol.OpRun, protocol.JobArgs{}), api.CodeInvalid, "id")
	requireCode(t, serve(t, h, protocol.ServiceJobs, protocol.OpSetEnabled, protocol.JobEnabledArgs{Enabled: true}), api.CodeInvalid, "id")
	requireCode(t, serve(t, h, protocol.ServiceJobs, "delete", nil), api.CodeInvalid, "jobs")
}

func TestJobsWithoutASurfaceAreUnavailable(t *testing.T) {
	h := jobsHandler(t, nil, api.CapabilityJobs)
	requireCode(t, serve(t, h, protocol.ServiceJobs, protocol.OpList, nil), api.CodeUnavailable, "jobs")
}
