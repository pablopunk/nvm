package app

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/diagnostics"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

type blockingReportSurface struct {
	entered, release chan struct{}
	once             sync.Once
	calls            atomic.Int32
}

func (s *blockingReportSurface) Recent() []api.DiagnosticRecord {
	return []api.DiagnosticRecord{{Ref: "NVM-SAFE1", Component: api.DiagnosticsProblemComponent}}
}
func (s *blockingReportSurface) Export(string) (string, bool) { return "", false }
func (s *blockingReportSurface) Reporting() string            { return api.DiagnosticsRequested }
func (s *blockingReportSurface) ReportProblem() string {
	s.calls.Add(1)
	s.once.Do(func() { close(s.entered) })
	<-s.release
	return "NVM-SAFE1"
}

type sessionObservingSink struct {
	*analytics.Client
	snapshots chan analytics.SessionSnapshot
	calls     atomic.Int32
}

func (s *sessionObservingSink) TouchSession() (analytics.SessionSnapshot, bool) {
	s.calls.Add(1)
	snapshot, ok := s.Client.TouchSession()
	if ok {
		s.snapshots <- snapshot
	}
	return snapshot, ok
}

func TestReportProblemAcceptedCommandReachesCollector(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
	client := analytics.New(analytics.Config{Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey, Transport: services.HTTP, FS: services.FS, Clock: services.Clock, Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true})
	sink := &sessionObservingSink{Client: client, snapshots: make(chan analytics.SessionSnapshot, 1)}
	app := &App{Palette: palette.New(palette.Options{}), ports: services.Ports(), commandTiming: timing.New(services.Clock, sink)}
	state, err := store.Open(services.FS, services.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	surface := &blockingReportSurface{entered: make(chan struct{}), release: make(chan struct{})}
	m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock, Diagnostics: surface}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{diagnostics.New()}, RunObserver: extensionRunObserver(app.commandTiming)})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if commands := m.Commands(context.Background()); len(commands) != 1 || commands[0].Failure != nil || len(commands[0].Commands) != 1 {
		t.Fatalf("discovery: %#v", commands)
	}
	type response struct {
		result  api.Result
		failure *api.Failure
	}
	done := make(chan response, 1)
	app.extensionViewDeps(m).InvokeCommand(diagnostics.ID, api.Invocation{Ref: api.ActionRef{ID: diagnostics.ReportAction, Args: map[string]string{"private": "args-canary"}}, Value: "query-canary", Background: true}, func(result api.Result, failure *api.Failure) { done <- response{result, failure} })
	<-surface.entered
	var snapshot analytics.SessionSnapshot
	select {
	case snapshot = <-sink.snapshots:
	default:
		t.Fatal("dispatch did not touch session before report entered")
	}
	if sink.calls.Load() != 1 || snapshot.SessionID() == "" {
		t.Fatalf("touch calls=%d session=%q", sink.calls.Load(), snapshot.SessionID())
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(commandTimingEvents(t, services)) != 0 || len(commandUsageEvents(t, services)) != 0 {
		t.Fatal("completion emitted before report returned")
	}
	close(surface.release)
	got := <-done
	app.settleExtensionCalls()
	if got.failure != nil || got.result.View == nil || got.result.Toast == nil || got.result.Toast.Message != "Problem report NVM-SAFE1 recorded" {
		t.Fatalf("result: %#v", got)
	}
	if surface.calls.Load() != 1 {
		t.Fatalf("report calls = %d", surface.calls.Load())
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	ops, usage := commandTimingEvents(t, services), commandUsageEvents(t, services)
	if len(ops) != 1 || len(usage) != 1 {
		t.Fatalf("events: operations=%#v usage=%#v", ops, usage)
	}
	if ops[0]["operation"] != "command.execute" || ops[0]["outcome"] != "success" || ops[0]["journey_id"] == nil {
		t.Fatalf("operation: %#v", ops[0])
	}
	if usage[0]["feature"] != analytics.FeatureCommandRun || usage[0]["variant"] != "extension-command:nevermind.diagnostics:report-problem" || usage[0]["outcome"] != "success" {
		t.Fatalf("usage: %#v", usage[0])
	}
	if sink.calls.Load() != 1 || ops[0]["$session_id"] != snapshot.SessionID() || usage[0]["$session_id"] != snapshot.SessionID() {
		t.Fatalf("session mismatch: %#v %#v", ops[0], usage[0])
	}
	for _, request := range services.HTTP.Requests() {
		for _, secret := range []string{"args-canary", "query-canary", "NVM-SAFE1"} {
			if bytes.Contains(request.Body, []byte(secret)) {
				t.Fatalf("private value %q leaked: %s", secret, request.Body)
			}
		}
	}
}
