package timing

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

func TestManagerViewActionWaitsForCompletionAndRedactsPrivateData(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	state, err := store.Open(set.FS, set.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	ext := &probeExtension{entered: make(chan struct{}), release: make(chan struct{})}
	tracker := New(set.Clock, client)
	host, err := manager.New(manager.Ports{Files: set.FS, Clock: set.Clock}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{ext}, RunObserver: ExtensionRunObserver(tracker)})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	origin := host.ResolveViewActionOrigin("nevermind.probe")
	ctx := manager.WithViewActionRunOrigin(context.Background(), origin)
	completed := make(chan struct {
		result  api.Result
		failure *api.Failure
		err     error
	}, 1)
	go func() {
		result, failure, runErr := host.Run(ctx, "nevermind.probe", api.Invocation{Ref: api.ActionRef{ID: "probe", Args: map[string]string{"private": "args-canary"}}})
		completed <- struct {
			result  api.Result
			failure *api.Failure
			err     error
		}{result, failure, runErr}
	}()
	<-ext.entered
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if events := commandEvents(t, set); len(events) != 0 {
		t.Fatalf("events before release: %#v", events)
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
	events := commandEvents(t, set)
	if len(events) != 2 {
		t.Fatalf("events=%#v, want operation and usage", events)
	}
	var operation, usage map[string]any
	for _, event := range events {
		switch event["event"] {
		case analytics.EventOperationCompleted:
			operation = event["properties"].(map[string]any)
		case analytics.EventFeatureUsed:
			usage = event["properties"].(map[string]any)
		}
	}
	if operation == nil || usage == nil {
		t.Fatalf("missing event: %#v", events)
	}
	if operation["operation"] != "view.action" || operation["outcome"] != "success" || operation["release"] != "test-release" {
		t.Fatalf("operation properties: %#v", operation)
	}
	if usage["feature"] != analytics.FeatureViewAction || usage["variant"] != "extension-action:nevermind.probe" || usage["outcome"] != "success" || usage["release"] != "test-release" {
		t.Fatalf("usage properties: %#v", usage)
	}
	if operation["$session_id"] == nil || operation["$session_id"] != usage["$session_id"] {
		t.Fatalf("operation and usage session differs: %#v %#v", operation, usage)
	}
	for _, key := range []string{"journey_id", "action_id", "boot_id"} {
		if _, exists := usage[key]; exists {
			t.Fatalf("usage included %s", key)
		}
	}
	for _, request := range set.HTTP.Requests() {
		for _, secret := range []string{"args-canary", "canary", "private-command-canary"} {
			if bytes.Contains(request.Body, []byte(secret)) {
				t.Fatalf("private data leaked: %s", request.Body)
			}
		}
	}
}
