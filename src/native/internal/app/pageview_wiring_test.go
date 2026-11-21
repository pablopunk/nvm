package app

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestPaletteNavigationWiringRecordsPageViewsWithSession(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200})
	client := analytics.New(analytics.Config{
		Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey,
		Transport: services.HTTP, FS: services.FS, Clock: services.Clock,
		Dir: "/analytics", Version: "test", Release: "test-release", Consent: true,
	})
	app := &App{analytics: client}
	const titleCanary = "private-page-title-canary"
	const queryCanary = "private-page-query-canary"
	page := palette.New(palette.Options{
		Root: &palette.View{Title: titleCanary, Search: func(string) []search.Item {
			return []search.Item{{Title: titleCanary}}
		}},
		Clock: services.Clock, Synchronous: true, OnNavigation: app.recordPaletteNavigation,
	})
	page.Attach(&testWindow{})
	tester := ui.NewTester(page.View, width, height)
	page.Show()
	page.SetQuery(queryCanary)
	tester.Frame()
	page.Refresh()
	tester.Frame()
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := pageViewEvents(t, services); len(got) != 1 || got[0]["view"] != string(analytics.PageViewPalette) {
		t.Fatalf("page view events = %#v", got)
	} else if session, ok := got[0]["$session_id"].(string); !ok || len(session) != 36 {
		t.Fatalf("page view session = %#v", got[0]["$session_id"])
	}
	if client.Pending() != 0 {
		t.Fatalf("pending events after flush = %d", client.Pending())
	}
	for _, request := range services.HTTP.Requests() {
		if bytes.Contains(request.Body, []byte(titleCanary)) || bytes.Contains(request.Body, []byte(queryCanary)) {
			t.Fatal("rendered title or query leaked into pageview")
		}
	}
}

func TestPaletteNavigationWiringHonorsConsentAndNilClient(t *testing.T) {
	services := fake.NewSet()
	services.HTTP.Script(fake.Response{Status: 200})
	client := analytics.New(analytics.Config{
		Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey,
		Transport: services.HTTP, FS: services.FS, Clock: services.Clock,
		Dir: "/analytics", Version: "test", Release: "test-release", Consent: false,
	})
	(&App{analytics: client}).recordPaletteNavigation(analytics.PageViewPalette)
	if client.Pending() != 0 {
		t.Fatalf("consent-off queued %d events", client.Pending())
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests := services.HTTP.Requests(); len(requests) != 0 {
		t.Fatalf("consent-off sent requests: %d", len(requests))
	}
	(&App{}).recordPaletteNavigation(analytics.PageViewPalette)
}

func pageViewEvents(t *testing.T, services *fake.Set) []map[string]any {
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
			if event.Event == analytics.EventPageview {
				events = append(events, event.Properties)
			}
		}
	}
	return events
}
