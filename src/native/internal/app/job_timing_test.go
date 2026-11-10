package app

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

func TestSchedulerJobTimingUsesAppTracker(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200})
	client := analytics.New(analytics.Config{
		Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey,
		Transport: services.HTTP, FS: services.FS, Clock: services.Clock,
		Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true,
	})
	app := &App{ports: services.Ports(), commandTiming: timing.New(services.Clock, client)}
	cfg := app.jobConfig()
	cfg.Reporter = safego.ReporterFunc(func(safego.Failure) string { return "" })
	scheduler, err := jobs.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()

	started, release := make(chan struct{}), make(chan struct{})
	if err := scheduler.Register(jobs.Definition{
		ID: "private.job.id", Title: "Private job title", StartDisabled: true,
		Run: func(ctx context.Context, _ jobs.Run) error {
			close(started)
			select {
			case <-release:
				return errors.New("private job error canary")
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}); err != nil {
		t.Fatal(err)
	}
	changes, unsubscribe := scheduler.Subscribe()
	defer unsubscribe()
	if err := scheduler.SetEnabled("private.job.id", true); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.RunNow("private.job.id"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not start the job")
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := services.HTTP.Requests(); len(got) != 0 {
		t.Fatalf("job emitted analytics before completion: %d requests", len(got))
	}
	services.Clock.Advance(1500 * time.Microsecond)
	close(release)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-changes:
			state := scheduler.Snapshot()[0]
			if !state.Running && len(state.History) > 0 {
				goto finished
			}
		case <-deadline:
			t.Fatal("scheduler did not publish job completion")
		}
	}
finished:
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := commandTimingEvents(t, services)
	if len(events) != 1 {
		t.Fatalf("operation events=%d, want 1: %#v", len(events), events)
	}
	event := events[0]
	if event["operation"] != "job.run" || event["outcome"] != "failed" || event["stage"] != "dispatch" || event["duration_ms"] != float64(2) || event["release"] != "test-release" {
		t.Fatalf("unexpected job operation event: %#v", event)
	}
	for _, request := range services.HTTP.Requests() {
		if bytes.Contains(request.Body, []byte("private.job.id")) || bytes.Contains(request.Body, []byte("Private job title")) || bytes.Contains(request.Body, []byte("private job error canary")) {
			t.Fatalf("private job data leaked in analytics: %s", request.Body)
		}
	}
}

func TestJobConfigHasNoObserverWithoutAppTracker(t *testing.T) {
	services := fake.NewSet()
	if observer := (&App{ports: services.Ports()}).jobConfig().RunObserver; observer != nil {
		t.Fatalf("nil tracker produced a job observer: %#v", observer)
	}
}
