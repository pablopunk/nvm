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
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

type commandUsageProbe struct{}

func (commandUsageProbe) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.probe.usage", Title: "Private extension title"}}
}

func (commandUsageProbe) RunAction(_ api.Context, in api.Invocation) (api.Result, error) {
	return api.Result{Toast: &api.Toast{Message: "private-result"}}, nil
}

func (commandUsageProbe) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "run", Title: "Private command title"}}, nil
}

func TestAppCommandInvocationRecordsSafeUsageOnlyForPrimaryCommand(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
	client := analytics.New(analytics.Config{
		Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey,
		Transport: services.HTTP, FS: services.FS, Clock: services.Clock,
		Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true,
	})
	app := &App{Palette: palette.New(palette.Options{}), ports: services.Ports(), commandTiming: timing.New(services.Clock, client)}
	state, err := store.Open(services.FS, services.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/extensions", enablement.Open(state), manager.Options{
		BuiltIns: []api.Extension{commandUsageProbe{}}, RunObserver: extensionRunObserver(app.commandTiming),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if commands := m.Commands(context.Background()); len(commands) != 1 || commands[0].Failure != nil || len(commands[0].Commands) != 1 {
		t.Fatalf("command discovery failed: %#v", commands)
	}

	deps := app.extensionViewDeps(m)
	invoke := func(run func(string, api.Invocation, func(api.Result, *api.Failure))) api.Result {
		t.Helper()
		resultCh := make(chan api.Result, 1)
		failureCh := make(chan *api.Failure, 1)
		run("nevermind.probe.usage", api.Invocation{
			Ref:   api.ActionRef{ID: "run", Args: map[string]string{"private-ref-arg": "secret-ref"}},
			Value: "private-query-canary", Background: true,
		}, func(result api.Result, failure *api.Failure) {
			resultCh <- result
			failureCh <- failure
		})
		result := <-resultCh
		if failure := <-failureCh; failure != nil {
			t.Fatalf("invocation failed: %#v", failure)
		}
		app.settleExtensionCalls()
		return result
	}

	primary := invoke(deps.InvokeCommand)
	if primary.Toast == nil || primary.Toast.Message != "private-result" {
		t.Fatalf("primary result not preserved: %#v", primary)
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if events := commandUsageEvents(t, services); len(events) != 1 {
		t.Fatalf("primary usage events = %d, want one: %#v", len(events), events)
	} else if events[0]["feature"] != analytics.FeatureCommandRun || events[0]["variant"] != "extension-command:nevermind.probe.usage:run" || events[0]["outcome"] != "success" {
		t.Fatalf("unexpected primary usage event: %#v", events[0])
	}

	secondary := invoke(deps.Invoke)
	if secondary.Toast == nil || secondary.Toast.Message != "private-result" {
		t.Fatalf("generic result not preserved: %#v", secondary)
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if events := commandUsageEvents(t, services); len(events) != 1 {
		t.Fatalf("generic invoke emitted usage; events=%#v", events)
	}
	for _, request := range services.HTTP.Requests() {
		for _, secret := range []string{"Private extension title", "Private command title", "private-query-canary", "secret-ref", "private-result"} {
			if bytes.Contains(request.Body, []byte(secret)) {
				t.Fatalf("private value %q leaked in analytics: %s", secret, request.Body)
			}
		}
	}
}

func commandUsageEvents(t *testing.T, services *fake.Set) []map[string]any {
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
			if event.Event == analytics.EventFeatureUsed {
				events = append(events, event.Properties)
			}
		}
	}
	return events
}
