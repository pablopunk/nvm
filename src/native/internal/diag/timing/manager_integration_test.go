package timing

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

type probeExtension struct {
	entered chan struct{}
	release chan struct{}
}

func (e *probeExtension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.probe", Title: "Probe"}}
}

func (e *probeExtension) RunAction(api.Context, api.Invocation) (api.Result, error) {
	close(e.entered)
	<-e.release
	return api.Result{Toast: &api.Toast{Message: "canary"}}, nil
}

func TestManagerRunReportsOnlyCompletedOperation(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	state, err := store.Open(set.FS, set.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	enabled := enablement.Open(state)
	ext := &probeExtension{entered: make(chan struct{}), release: make(chan struct{})}
	host, err := manager.New(manager.Ports{Files: set.FS, Clock: set.Clock}, "/extensions", enabled, manager.Options{BuiltIns: []api.Extension{ext}, RunObserver: ExtensionRunObserver(New(set.Clock, client))})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	type response struct {
		result  api.Result
		failure *api.Failure
		err     error
	}
	completed := make(chan response, 1)
	invocation := api.Invocation{Ref: api.ActionRef{ID: "probe", Args: map[string]string{"private": "args-canary"}}}
	go func() {
		result, failure, runErr := host.Run(manager.WithCommandRun(context.Background()), "nevermind.probe", invocation)
		completed <- response{result, failure, runErr}
	}()
	<-ext.entered
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := payloads(t, set); len(got) != 0 {
		t.Fatalf("events before operation completed: %v", got)
	}
	set.Clock.Advance(1500 * time.Microsecond)
	close(ext.release)
	got := <-completed
	if got.err != nil || got.failure != nil || got.result.Toast == nil || got.result.Toast.Message != "canary" {
		t.Fatalf("Run result not preserved: %#v", got)
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	props := payloads(t, set)
	if len(props) != 1 {
		t.Fatalf("operation events=%d, want 1", len(props))
	}
	p := props[0]
	if p["duration_ms"] != float64(2) || p["outcome"] != "success" || p["stage"] != "invoke" || p["operation"] != "command.execute" || p["release"] != "test-release" {
		t.Fatalf("unexpected operation event: %#v", p)
	}
	for _, key := range []string{"boot_id", "journey_id", "action_id"} {
		id, ok := p[key].(string)
		if !ok || len(id) != 32 {
			t.Fatalf("%s is not a 32 character ID: %#v", key, p[key])
		}
	}
	for _, req := range set.HTTP.Requests() {
		if bytes.Contains(req.Body, []byte("args-canary")) || bytes.Contains(req.Body, []byte("canary")) {
			t.Fatalf("private invocation/result leaked: %s", req.Body)
		}
	}
}

func TestManagerAdapterOnlyCapturesMarkedCommandRuns(t *testing.T) {
	for _, tc := range []struct {
		name               string
		marked, background bool
		fail               bool
		want               int
	}{
		{"unmarked failure", false, false, true, 0},
		{"marked background command", true, true, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set, client := newTestTracker(t)
			set.HTTP.Script(fake.Response{Status: 200})
			state, err := store.Open(set.FS, set.Clock, "/state")
			if err != nil {
				t.Fatal(err)
			}
			defer state.Close()
			ext := &failureExtension{fail: tc.fail}
			host, err := manager.New(manager.Ports{Files: set.FS, Clock: set.Clock}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{ext}, RunObserver: ExtensionRunObserver(New(set.Clock, client))})
			if err != nil {
				t.Fatal(err)
			}
			defer host.Close()
			ctx := context.Background()
			if tc.marked {
				ctx = manager.WithCommandRun(ctx)
			}
			_, _, _ = host.Run(ctx, "nevermind.failure", api.Invocation{Background: tc.background})
			if err := client.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			got := payloads(t, set)
			if len(got) != tc.want {
				t.Fatalf("events=%d want %d: %#v", len(got), tc.want, got)
			}
			if tc.want == 1 && got[0]["operation"] != "command.execute" {
				t.Fatalf("unexpected event: %#v", got[0])
			}
		})
	}
}

type failureExtension struct{ fail bool }

func (e *failureExtension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.failure", Title: "Failure"}}
}
func (e *failureExtension) RunAction(api.Context, api.Invocation) (api.Result, error) {
	if e.fail {
		return api.Result{}, context.Canceled
	}
	return api.Result{}, nil
}
