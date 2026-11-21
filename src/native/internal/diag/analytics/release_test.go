package analytics

import (
	"context"
	"net/http"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func releaseTestClient(set *fake.Set, release string, consent bool) *Client {
	return New(Config{Endpoint: testEndpoint, APIKey: IngestKey, Transport: set.HTTP, FS: set.FS, Clock: set.Clock,
		Dir: "/config", Version: "2.0.0", OS: "darwin", Arch: "arm64", Release: release, Consent: consent})
}

func TestReleaseSurvivesEventSchemaFiltering(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := releaseTestClient(set, "nevermind@2.0.0+"+fixtureReleaseCommit+".darwin", true)
	c.AppLaunched()
	c.Exception(diag.Record{Ref: "NVM-AB23C", Component: "store.open", Message: "failure", Attributes: map[string]string{"safe": "yes"}})
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	batches := sentBatches(t, set)
	if len(batches) != 1 || len(batches[0].Batch) != 2 {
		t.Fatalf("unexpected payload batches: %#v", batches)
	}
	for _, event := range batches[0].Batch {
		if event.Properties["release"] != c.cfg.Release {
			t.Fatalf("%s release = %#v", event.Event, event.Properties["release"])
		}
	}
	if got := batches[0].Batch[0].Properties; len(got) != len(c.common)+1 {
		t.Fatalf("launch properties = %v", got)
	}
	if got := batches[0].Batch[1].Properties; got["component"] != "store.open" || got["message"] != "failure" {
		t.Fatalf("exception properties were filtered incorrectly: %v", got)
	}
}

const fixtureReleaseCommit = "0123456789abcdef0123456789abcdef01234567"

func TestEmptyReleaseIsOmittedAndOtherEventsUnchanged(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := releaseTestClient(set, "", true)
	c.AppLaunched()
	c.FeatureUsed(FeatureCommandRun, "settings")
	if c.capture("unknown.event", map[string]any{"release": "spoof"}) {
		t.Fatal("unknown event passed the event allow-list")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	batches := sentBatches(t, set)
	if got := eventNames(batches[0]); len(got) != 2 || got[0] != EventAppLaunched || got[1] != EventFeatureUsed {
		t.Fatalf("events = %v", got)
	}
	for _, event := range batches[0].Batch {
		if _, ok := event.Properties["release"]; ok {
			t.Fatalf("empty release present in %s: %v", event.Event, event.Properties)
		}
	}
	if batches[0].Batch[1].Properties["feature"] != FeatureCommandRun || batches[0].Batch[1].Properties["variant"] != "settings" {
		t.Fatalf("feature event changed: %v", batches[0].Batch[1].Properties)
	}
}

func TestReleaseDoesNotBypassConsentAndNilClientIsSafe(t *testing.T) {
	set := fake.NewSet()
	c := releaseTestClient(set, "nevermind@2.0.0", false)
	if c.AppLaunched() || c.Exception(diag.Record{Ref: "NVM-AB23C"}) {
		t.Fatal("consent-off client kept an event")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(set.HTTP.Requests()) != 0 {
		t.Fatal("consent-off client sent a request")
	}
	var nilClient *Client
	if nilClient.AppLaunched() || nilClient.Exception(diag.Record{}) {
		t.Fatal("nil client unexpectedly kept an event")
	}
}
