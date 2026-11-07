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

type viewActionWiringProbe struct{ entered, release chan struct{} }

func (p *viewActionWiringProbe) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.probe.view", Title: "private-title"}}
}
func (p *viewActionWiringProbe) RunAction(_ api.Context, _ api.Invocation) (api.Result, error) {
	p.entered <- struct{}{}
	<-p.release
	return api.Result{Toast: &api.Toast{Message: "private-result"}}, nil
}
func TestAppViewActionWiring(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200}, fake.Response{Status: 200})
	client := analytics.New(analytics.Config{Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey, Transport: services.HTTP, FS: services.FS, Clock: services.Clock, Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true})
	app := &App{Palette: palette.New(palette.Options{}), ports: services.Ports(), commandTiming: timing.New(services.Clock, client)}
	state, err := store.Open(services.FS, services.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	probe := &viewActionWiringProbe{make(chan struct{}, 1), make(chan struct{}, 1)}
	m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{probe}, RunObserver: extensionRunObserver(app.commandTiming)})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	deps := app.extensionViewDeps(m)
	invoke := func(run func(string, api.Invocation, func(api.Result, *api.Failure)), early bool) {
		t.Helper()
		result := make(chan api.Result, 1)
		failures := make(chan *api.Failure, 1)
		run("nevermind.probe.view", api.Invocation{Ref: api.ActionRef{ID: "private-action", Args: map[string]string{"key": "private-args"}}, Value: "private-query"}, func(r api.Result, f *api.Failure) { result <- r; failures <- f })
		<-probe.entered
		if early {
			if err := client.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(services.HTTP.Requests()) != 0 {
				t.Fatal("event before completion")
			}
		}
		probe.release <- struct{}{}
		r := <-result
		if f := <-failures; f != nil || r.Toast == nil || r.Toast.Message != "private-result" {
			t.Fatalf("result=%#v failure=%#v", r, f)
		}
		app.settleExtensionCalls()
	}
	invoke(deps.InvokeViewAction, true)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	invoke(deps.Invoke, false)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	var events []struct {
		Event      string         `json:"event"`
		Properties map[string]any `json:"properties"`
	}
	for _, req := range services.HTTP.Requests() {
		for _, secret := range []string{"private-title", "private-action", "private-args", "private-query", "private-result"} {
			if bytes.Contains(req.Body, []byte(secret)) {
				t.Fatalf("leaked %q", secret)
			}
		}
		var batch struct {
			Batch []struct {
				Event      string         `json:"event"`
				Properties map[string]any `json:"properties"`
			} `json:"batch"`
		}
		if err := json.Unmarshal(req.Body, &batch); err != nil {
			t.Fatal(err)
		}
		events = append(events, batch.Batch...)
	}
	if len(events) != 2 {
		t.Fatalf("events=%#v", events)
	}
	var operation, usage map[string]any
	for _, e := range events {
		switch e.Event {
		case analytics.EventOperationCompleted:
			operation = e.Properties
		case analytics.EventFeatureUsed:
			usage = e.Properties
		}
	}
	if operation["$session_id"] == nil || operation["$session_id"] != usage["$session_id"] || operation["journey_id"] == nil {
		t.Fatalf("missing session or journey: %#v %#v", operation, usage)
	}
	if operation["operation"] != "view.action" || operation["label"] != "Extension view action" || usage["feature"] != analytics.FeatureViewAction || usage["variant"] != "extension-action:nevermind.probe.view" {
		t.Fatalf("operation=%#v usage=%#v", operation, usage)
	}
}
