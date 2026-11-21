package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPageViewedSendsClosedViewsWithCommonContextAndSession(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: 200})
	c := newTestClient(t, set)
	views := []PageView{PageViewPalette, PageViewExtension, PageViewAIChat}
	for _, view := range views {
		if !c.PageViewed(view) {
			t.Fatalf("PageViewed(%q) rejected", view)
		}
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	batches := sentBatches(t, set)
	if len(batches) != 1 || len(batches[0].Batch) != len(views) {
		t.Fatalf("batches = %#v", batches)
	}
	session := ""
	for i, event := range batches[0].Batch {
		if event.Event != EventPageview || event.Properties["view"] != string(views[i]) {
			t.Fatalf("event = %#v", event)
		}
		id, ok := event.Properties["$session_id"].(string)
		if !ok || len(id) != 36 {
			t.Fatalf("session = %#v", event.Properties["$session_id"])
		}
		if session != "" && id != session {
			t.Fatalf("session changed: %s / %s", session, id)
		}
		session = id
		for _, key := range []string{"app_version", "os_type", "os_release", "arch", "channel"} {
			if _, ok := event.Properties[key]; !ok {
				t.Errorf("missing common property %q", key)
			}
		}
	}
}

func TestInvalidPageViewDoesNotTouchSession(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: 200})
	c := newTestClient(t, set)
	if c.PageViewed(PageView("https://private.invalid/path")) {
		t.Fatal("invalid pageview accepted")
	}
	if c.session.id != "" || c.Pending() != 0 {
		t.Fatal("invalid view changed session or queue")
	}
	if !c.PageViewed(PageViewPalette) {
		t.Fatal("valid view rejected")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(sentBatches(t, set)); got != 1 {
		t.Fatalf("requests = %d", got)
	}
}

func TestPageViewedWithoutReportingDoesNotCreateActivity(t *testing.T) {
	set := fake.NewSet()
	for name, client := range map[string]*Client{
		"nil":         nil,
		"development": New(Config{Clock: set.Clock, Consent: true}),
		"consent-off": New(Config{Clock: set.Clock, Endpoint: testEndpoint}),
	} {
		t.Run(name, func(t *testing.T) {
			if client.PageViewed(PageViewPalette) || client.Pending() != 0 {
				t.Fatal("disabled reporting retained pageview")
			}
			if client != nil && client.session.id != "" {
				t.Fatal("disabled reporting minted a session")
			}
		})
	}
	if len(set.HTTP.Requests()) != 0 {
		t.Fatal("disabled reporting reached the transport")
	}
}

func TestPageViewedHonorsConsentAndSessionWindows(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	if !c.PageViewed(PageViewPalette) {
		t.Fatal("initial view rejected")
	}
	first := c.session.id
	set.Clock.Advance(sessionIdleLimit - time.Nanosecond)
	if !c.PageViewed(PageViewExtension) || c.session.id != first {
		t.Fatal("session rolled before idle threshold")
	}
	set.Clock.Advance(sessionIdleLimit)
	if !c.PageViewed(PageViewAIChat) || c.session.id == first {
		t.Fatal("session did not roll after idle threshold")
	}
	c.SetConsent(false)
	if c.PageViewed(PageViewPalette) || c.session.id != "" {
		t.Fatal("consent-off view created activity")
	}
	c.SetConsent(true)
	if !c.PageViewed(PageViewPalette) || c.session.id == first {
		t.Fatal("consent-on view failed to create fresh session")
	}
}
