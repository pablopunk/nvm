package app

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

type commandTimingProbe struct {
	id string
}

func (e commandTimingProbe) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: e.id, Title: e.id}}
}

func (e commandTimingProbe) RunAction(api.Context, api.Invocation) (api.Result, error) {
	return api.Result{Toast: &api.Toast{Message: "private-result-canary"}}, nil
}

func TestExtensionRunObserverSharesAppCommandTimingAcrossManagers(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
	client := analytics.New(analytics.Config{
		Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey,
		Transport: services.HTTP, FS: services.FS, Clock: services.Clock,
		Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true,
	})
	app := &App{commandTiming: timing.New(services.Clock, client)}
	state, err := store.Open(services.FS, services.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	enabled := enablement.Open(state)
	newManager := func(id string) *manager.Manager {
		t.Helper()
		extensionManager, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/extensions", enabled, manager.Options{
			BuiltIns: []api.Extension{commandTimingProbe{id: id}}, RunObserver: extensionRunObserver(app.commandTiming),
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { extensionManager.Close() })
		return extensionManager
	}

	first, second := newManager("nevermind.probe.first"), newManager("nevermind.probe.second")
	if _, _, err := first.Run(context.Background(), "nevermind.probe.first", api.Invocation{Ref: api.ActionRef{ID: "unmarked"}}); err != nil {
		t.Fatal(err)
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := commandTimingEvents(t, services); len(got) != 0 {
		t.Fatalf("unmarked run emitted events: %#v", got)
	}

	for _, run := range []struct {
		mgr    *manager.Manager
		extID  string
		action string
	}{{first, "nevermind.probe.first", "first-action"}, {second, "nevermind.probe.second", "second-action"}} {
		result, _, err := run.mgr.Run(manager.WithCommandRun(context.Background()), run.extID, api.Invocation{Ref: api.ActionRef{ID: run.action}})
		if err != nil || result.Toast == nil || result.Toast.Message != "private-result-canary" {
			t.Fatalf("Run result not preserved: result=%#v err=%v", result, err)
		}
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := commandTimingEvents(t, services)
	if len(events) != 2 {
		t.Fatalf("marked run events=%d, want 2: %#v", len(events), events)
	}
	bootIDs, journeys, actions := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, event := range events {
		if event["operation"] != "command.execute" || event["stage"] != "invoke" || event["outcome"] != "success" || event["release"] != "test-release" {
			t.Fatalf("unexpected command event: %#v", event)
		}
		bootIDs[event["boot_id"].(string)] = true
		journeys[event["journey_id"].(string)] = true
		actions[event["action_id"].(string)] = true
	}
	if len(bootIDs) != 1 || len(journeys) != 2 || len(actions) != 2 {
		t.Fatalf("unexpected correlation: boots=%v journeys=%v actions=%v", bootIDs, journeys, actions)
	}
	for _, request := range services.HTTP.Requests() {
		if len(request.Body) == 0 || bytes.Contains(request.Body, []byte("private-result-canary")) {
			t.Fatalf("private command data leaked in analytics: %s", request.Body)
		}
	}
}

func TestExtensionRunObserverReturnsNilWithoutTracker(t *testing.T) {
	if observer := extensionRunObserver(nil); observer != nil {
		t.Fatalf("nil tracker produced observer: %#v", observer)
	}
}

func commandTimingEvents(t *testing.T, services *fake.Set) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, request := range services.HTTP.Requests() {
		var batch struct {
			Batch []struct {
				Event      string         `json:"event"`
				Properties map[string]any `json:"properties"`
			} `json:"batch"`
		}
		if err := json.Unmarshal(request.Body, &batch); err != nil {
			t.Fatal(err)
		}
		for _, event := range batch.Batch {
			if event.Event == analytics.EventOperationCompleted {
				events = append(events, event.Properties)
			}
		}
	}
	return events
}
