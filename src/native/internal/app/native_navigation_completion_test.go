package app

import (
	"runtime"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

func TestSynchronousNavigationCommandsUseCompletionTracking(t *testing.T) {
	commands := []struct {
		id      string
		variant string
	}{
		{shortcuts.CommandExtensions, "extensions"},
		{shortcuts.CommandKeyboardShortcuts, "keyboard-shortcuts"},
		{shortcuts.CommandBackgroundTasks, "background-tasks"},
		{shortcuts.CommandShowRecentErrors, "show-recent-errors"},
	}
	for _, test := range commands {
		t.Run(test.id, func(t *testing.T) {
			services := fake.NewSet()
			services.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
			client := analytics.New(analytics.Config{Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey, Transport: services.HTTP, FS: services.FS, Clock: services.Clock, Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true})
			p := &App{commandTiming: timing.New(services.Clock, client), analytics: client}
			p.systemSvc = newSystemServices(services.Ports(), runtime.GOOS)
			var selected command
			all := append(leadingCommands(p), trailingCommands(p)...)
			for _, candidate := range all {
				if candidate.ID == test.id {
					selected = candidate
					break
				}
			}
			if selected.Complete == nil {
				t.Fatalf("%q has no terminal completion callback", test.id)
			}
			host := &recordingHost{}
			p.trackedRun(selected)(host)
			if len(host.pushed) != 1 {
				t.Fatalf("%q navigation pushes = %d, want one", test.id, len(host.pushed))
			}
			flushNativeAnalytics(t, client)
			operations, usages := nativeCompletionEvents(t, services)
			if len(operations) != 1 || len(usages) != 1 {
				t.Fatalf("%q events = operations %d, usages %d; want one each and no legacy duplicate", test.id, len(operations), len(usages))
			}
			if usages[0]["variant"] != test.variant || usages[0]["outcome"] != "success" {
				t.Fatalf("%q usage is not safely labeled: %#v", test.id, usages[0])
			}
		})
	}
}

func TestOSPermissionsNavigationRemainsUntrackedWithoutClosedVariant(t *testing.T) {
	services := fake.NewSet()
	client := analytics.New(analytics.Config{Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey, Transport: services.HTTP, FS: services.FS, Clock: services.Clock, Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true})
	p := &App{commandTiming: timing.New(services.Clock, client), analytics: client}
	p.systemSvc = newSystemServices(services.Ports(), runtime.GOOS)
	var selected command
	for _, candidate := range append(leadingCommands(p), trailingCommands(p)...) {
		if candidate.ID == shortcuts.CommandOSPermissions {
			selected = candidate
			break
		}
	}
	if selected.Complete != nil {
		t.Fatal("OS Permissions gained completion tracking without a closed analytics variant")
	}
	if _, ok := analytics.CommandVariant(selected.Title); ok {
		t.Fatalf("OS Permissions unexpectedly has a closed analytics variant: %q", selected.Title)
	}
	host := &recordingHost{}
	p.trackedRun(selected)(host)
	if len(host.pushed) != 1 {
		t.Fatalf("OS Permissions navigation pushes = %d, want one", len(host.pushed))
	}
}
