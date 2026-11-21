package app

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

func TestTrackedNativeRunWaitsForTerminalCompletion(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
	client := analytics.New(analytics.Config{
		Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey,
		Transport: services.HTTP, FS: services.FS, Clock: services.Clock,
		Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true,
	})
	tracker := timing.New(services.Clock, client)
	parent, parentCompletion := tracker.Start(context.Background())
	parentCompletion.Finish(analytics.OutcomeSuccess)

	var complete func(analytics.OperationOutcome)
	run := trackedNativeRun(tracker, "settings", func(ctx context.Context, callback func(analytics.OperationOutcome)) {
		if ctx == nil {
			t.Fatal("native runner received nil context")
		}
		complete = callback
	})
	run(parent) // Returning from host dispatch must not finish the operation.
	if complete == nil {
		t.Fatal("native runner did not receive completion callback")
	}
	flushNativeAnalytics(t, client)
	if operations, usages := nativeCompletionEvents(t, services); len(operations) != 1 || len(usages) != 0 {
		t.Fatalf("host dispatch return completed tracked run: operations=%#v usages=%#v", operations, usages)
	}

	complete(analytics.OutcomeSuccess)
	complete(analytics.OutcomeFailed)
	flushNativeAnalytics(t, client)
	operations, usages := nativeCompletionEvents(t, services)
	if len(operations) != 2 || len(usages) != 1 {
		t.Fatalf("terminal callback events: operations=%d usages=%d", len(operations), len(usages))
	}
	var child map[string]any
	for _, operation := range operations {
		if operation["action_id"] != operations[0]["action_id"] {
			if child != nil {
				t.Fatalf("duplicate native operation event: %#v", operations)
			}
			child = operation
		}
	}
	if child == nil {
		t.Fatalf("native operation label missing: %#v", operations)
	}
	if child["operation"] != "command.execute" || child["stage"] != "invoke" || child["outcome"] != "success" || child["release"] != "test-release" {
		t.Fatalf("unexpected native completion: %#v", child)
	}
	parentEvent := operations[0]
	if child["action_id"] == parentEvent["action_id"] || child["journey_id"] != parentEvent["journey_id"] || child["$session_id"] == nil {
		t.Fatalf("native completion lost inherited journey or start session: parent=%#v child=%#v", parentEvent, child)
	}
	if usages[0]["feature"] != analytics.FeatureCommandRun || usages[0]["variant"] != "settings" || usages[0]["outcome"] != "success" || usages[0]["$session_id"] != child["$session_id"] {
		t.Fatalf("unexpected native usage event: %#v", usages[0])
	}
}

func TestSettingsCommandUsesCompletionTracking(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
	client := analytics.New(analytics.Config{Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey, Transport: services.HTTP, FS: services.FS, Clock: services.Clock, Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true})
	app := &App{commandTiming: timing.New(services.Clock, client)}
	app.systemSvc = newSystemServices(services.Ports(), runtime.GOOS)
	var settings command
	for _, candidate := range trailingCommands(app) {
		if candidate.ID == shortcuts.CommandSettings {
			settings = candidate
		}
	}
	if settings.Complete == nil {
		t.Fatal("Settings has no terminal completion callback")
	}
	host := &recordingHost{}
	app.trackedRun(settings)(host)
	if len(host.pushed) != 1 {
		t.Fatalf("Settings navigation pushes = %d, want one", len(host.pushed))
	}
	flushNativeAnalytics(t, client)
	operations, usages := nativeCompletionEvents(t, services)
	if len(operations) != 1 || len(usages) != 1 {
		t.Fatalf("Settings events = operations %d, usages %d; want one each and no legacy duplicate", len(operations), len(usages))
	}
	if usages[0]["variant"] != "settings" || usages[0]["outcome"] != "success" {
		t.Fatalf("Settings usage is not safely labeled: %#v", usages[0])
	}
}

func flushNativeAnalytics(t *testing.T, client *analytics.Client) {
	t.Helper()
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func nativeCompletionEvents(t *testing.T, services *fake.Set) ([]map[string]any, []map[string]any) {
	t.Helper()
	var operations, usages []map[string]any
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
			switch event.Event {
			case analytics.EventOperationCompleted:
				operations = append(operations, event.Properties)
			case analytics.EventFeatureUsed:
				usages = append(usages, event.Properties)
			}
		}
	}
	return operations, usages
}
