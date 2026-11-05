package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// testEndpoint is a placeholder URL. The fake transport answers it; nothing in these tests reaches a network.
const testEndpoint = "https://ingest.test/api/ingest/batch/"

// newTestClient returns a consenting client over fakes, and the set that holds them.
func newTestClient(t *testing.T, set *fake.Set) *Client {
	t.Helper()
	return New(Config{
		Endpoint:  testEndpoint,
		APIKey:    IngestKey,
		Transport: set.HTTP,
		FS:        set.FS,
		Clock:     set.Clock,
		Dir:       "/config",
		Version:   "0.9.0",
		OS:        "darwin",
		Arch:      "arm64",
		Consent:   true,
	})
}

// sentBatches decodes every batch the fake transport received.
func sentBatches(t *testing.T, set *fake.Set) []batchPayload {
	t.Helper()
	var out []batchPayload
	for _, req := range set.HTTP.Requests() {
		var payload batchPayload
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatalf("batch is not JSON: %v", err)
		}
		out = append(out, payload)
	}
	return out
}

func eventNames(batch batchPayload) []string {
	names := make([]string, 0, len(batch.Batch))
	for _, event := range batch.Batch {
		names = append(names, event.Event)
	}
	return names
}

func TestDevelopmentBuildWithoutEndpointNeverKeepsOrSends(t *testing.T) {
	set := fake.NewSet()
	c := New(Config{Transport: set.HTTP, FS: set.FS, Clock: set.Clock, Dir: "/config", Consent: true})
	if c.AppLaunched() {
		t.Fatal("an endpoint-less build kept an event")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if len(set.HTTP.Requests()) != 0 {
		t.Fatal("an endpoint-less build sent a request")
	}
	if c.Reporting() != LocalOnly {
		t.Fatalf("Reporting = %v, want LocalOnly", c.Reporting())
	}
}

func TestConsentOffKeepsNothing(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	c.SetConsent(false)
	if c.AppLaunched() || c.FeatureUsed(FeatureCommandRun, "settings") {
		t.Fatal("events were kept with consent off")
	}
	if c.Reporting() != Off {
		t.Fatalf("Reporting = %v, want Off", c.Reporting())
	}
}

func TestWithdrawingConsentDropsTheQueueAndSendsNothing(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	c.AppLaunched()
	c.FeatureUsed(FeatureCommandRun, "settings")
	c.SetConsent(false)
	if c.Pending() != 0 || c.Dropped() != 2 {
		t.Fatalf("pending %d dropped %d, want 0 and 2", c.Pending(), c.Dropped())
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := sentBatches(t, set); len(got) != 0 {
		t.Fatalf("sent %d batches after consent was withdrawn", len(got))
	}
}

func TestFlushSendsOneBatchInThePostHogFormat(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	c.AppLaunched()
	c.FeatureUsed(FeatureCommandRun, "clipboard-history")
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	batches := sentBatches(t, set)
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(batches))
	}
	if batches[0].APIKey != IngestKey {
		t.Fatalf("api_key = %q", batches[0].APIKey)
	}
	if got := eventNames(batches[0]); !slices.Equal(got, []string{EventAppLaunched, EventFeatureUsed}) {
		t.Fatalf("events = %q", got)
	}
	usage := batches[0].Batch[1]
	if usage.Properties["feature"] != FeatureCommandRun || usage.Properties["variant"] != "clipboard-history" {
		t.Fatalf("feature.used properties = %v", usage.Properties)
	}
	if usage.Properties["$process_person_profile"] != false || usage.Properties["channel"] != "stable" {
		t.Fatalf("common properties = %v", usage.Properties)
	}
	if usage.Properties["os_type"] != "darwin" || usage.Properties["arch"] != "arm64" || usage.Properties["app_version"] != "0.9.0" {
		t.Fatalf("platform properties = %v", usage.Properties)
	}
	if usage.DistinctID == "" || usage.Timestamp == "" {
		t.Fatalf("missing distinct id or timestamp: %+v", usage)
	}
	if c.Pending() != 0 {
		t.Fatalf("pending = %d after a sent batch", c.Pending())
	}
}

func TestKeepAllowedDropsKeysOutsideTheSchemaAndCannotOverrideCommonFields(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	kept := c.keepAllowed(map[string]any{
		"feature":     "command.run",
		"text":        "copied secret",
		"file_name":   "tax-return.pdf",
		"app_version": "spoofed",
	}, schema[EventFeatureUsed])
	for _, key := range []string{"text", "file_name"} {
		if _, ok := kept[key]; ok {
			t.Fatalf("content key %q was kept", key)
		}
	}
	if kept["app_version"] != "0.9.0" {
		t.Fatalf("app_version = %v, want the build's version", kept["app_version"])
	}
	if kept["feature"] != "command.run" {
		t.Fatalf("schema key dropped: %v", kept)
	}
}

func TestOfflineKeepsTheQueueAndSendsItWhenBackOnline(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Err: errors.New("network unreachable")}, fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	c.AppLaunched()
	c.SupportReport("NVM-AB23C")
	if err := c.Flush(context.Background()); err == nil {
		t.Fatal("Flush succeeded while offline")
	}
	if c.Pending() != 2 {
		t.Fatalf("pending = %d while offline, want 2", c.Pending())
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush after reconnect: %v", err)
	}
	batches := sentBatches(t, set)
	last := batches[len(batches)-1]
	if got := eventNames(last); !slices.Equal(got, []string{EventAppLaunched, EventSupportReport}) {
		t.Fatalf("events sent after reconnect = %q, want both in order", got)
	}
	if c.Pending() != 0 {
		t.Fatalf("pending = %d after reconnect", c.Pending())
	}
}

func TestUnavailableEndpointGetsThreeTriesThenTheBatchIsDropped(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(
		fake.Response{Status: http.StatusServiceUnavailable},
		fake.Response{Status: http.StatusServiceUnavailable},
		fake.Response{Status: http.StatusServiceUnavailable},
	)
	c := newTestClient(t, set)
	c.AppLaunched()
	for attempt := 1; attempt < maxAttempts; attempt++ {
		if err := c.Flush(context.Background()); err == nil {
			t.Fatalf("attempt %d succeeded against a 503", attempt)
		}
		if c.Pending() != 1 {
			t.Fatalf("pending = %d after attempt %d, want the batch kept", c.Pending(), attempt)
		}
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush on the last attempt: %v", err)
	}
	if c.Pending() != 0 {
		t.Fatalf("pending = %d after the last attempt, want the batch dropped", c.Pending())
	}
}

func TestRefusedBatchIsDroppedAtOnce(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusBadRequest})
	c := newTestClient(t, set)
	c.AppLaunched()
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush after a refusal: %v", err)
	}
	if c.Pending() != 0 {
		t.Fatalf("pending = %d after a refusal, want 0", c.Pending())
	}
	if len(set.HTTP.Requests()) != 1 {
		t.Fatalf("requests = %d, want 1: a refused batch is not retried", len(set.HTTP.Requests()))
	}
}

func TestQueueIsBoundedWhileTheEndpointIsUnreachable(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	for i := 0; i < maxQueue+5; i++ {
		c.AppLaunched()
	}
	if c.Pending() != maxQueue || c.Dropped() != 5 {
		t.Fatalf("pending %d dropped %d, want %d and 5", c.Pending(), c.Dropped(), maxQueue)
	}
}

func TestObserveInstallOwnerHashesAndDeduplicates(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	if !c.ObserveInstallOwner("private-account") || c.ObserveInstallOwner("private-account") {
		t.Fatal("owner observation should be accepted once")
	}
	if !c.ObserveInstallOwner("another-account") {
		t.Fatal("different owner should be accepted")
	}
	if err := c.Flush(context.Background()); err != nil { t.Fatal(err) }
	events := sentBatches(t, set)[0].Batch
	if len(events) != 2 { t.Fatalf("events = %d, want 2", len(events)) }
	wantHash := sha256.Sum256([]byte("private-account"))
	if events[0].Properties["owner_id"] != hex.EncodeToString(wantHash[:]) || events[0].Properties["owner_source"] != "account" {
		t.Fatalf("owner properties = %v", events[0].Properties)
	}
	if strings.Contains(string(set.HTTP.Requests()[0].Body), "private-account") || events[0].DistinctID == "" || events[0].DistinctID == events[0].Properties["owner_id"] {
		t.Fatal("account ID leaked or anonymous distinct ID changed")
	}
	if events[0].Properties["$process_person_profile"] != false { t.Fatal("person profile enabled") }
}

func TestObserveInstallOwnerConsentAndQueueRejectionDoNotMark(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	c.SetConsent(false)
	if c.ObserveInstallOwner("id") { t.Fatal("accepted with consent off") }
	c.SetConsent(true)
	if !c.ObserveInstallOwner("id") { t.Fatal("did not accept after consent enabled") }
	c = newTestClient(t, fake.NewSet())
	for i := 0; i < maxQueue; i++ { c.AppLaunched() }
	if c.ObserveInstallOwner("id") { t.Fatal("accepted into full queue") }
	c.mu.Lock(); c.queue = nil; c.mu.Unlock()
	if !c.ObserveInstallOwner("id") { t.Fatal("queue rejection incorrectly marked owner") }
}

func TestObserveInstallOwnerDoesNotWaitForFlushNetwork(t *testing.T) {
	set := fake.NewSet()
	entered, release := make(chan struct{}), make(chan struct{})
	var enteredOnce sync.Once
	set.HTTP.Handle(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { enteredOnce.Do(func() { close(entered) }); <-release; w.WriteHeader(http.StatusOK) }))
	c := newTestClient(t, set)
	c.AppLaunched()
	flushDone := make(chan struct{})
	go func() { _ = c.Flush(context.Background()); close(flushDone) }()
	<-entered
	ownerDone := make(chan bool, 1)
	go func() { ownerDone <- c.ObserveInstallOwner("id") }()
	select {
	case accepted := <-ownerDone:
		if !accepted { t.Fatal("owner was rejected") }
	case <-time.After(time.Second):
		t.Fatal("owner observation blocked on network flush")
	}
	close(release)
	<-flushDone
}

func TestInstallIdentifierIsCreatedOnceAndReusedByLaterLaunches(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK}, fake.Response{Status: http.StatusOK})
	first := newTestClient(t, set)
	first.AppLaunched()
	if err := first.Flush(context.Background()); err != nil {
		t.Fatalf("first Flush: %v", err)
	}
	stored, err := set.FS.ReadFile("/config/analytics-id")
	if err != nil || len(stored) == 0 {
		t.Fatalf("install id not saved: %v", err)
	}
	second := newTestClient(t, set)
	second.AppLaunched()
	if err := second.Flush(context.Background()); err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	batches := sentBatches(t, set)
	if batches[0].Batch[0].DistinctID != string(stored) || batches[1].Batch[0].DistinctID != string(stored) {
		t.Fatalf("distinct ids %q and %q, want the stored %q", batches[0].Batch[0].DistinctID, batches[1].Batch[0].DistinctID, stored)
	}
}

func TestFailureIsSentUnderItsSupportReferenceWithTheCanaryRedacted(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	records := diag.NewRecorder(set.Clock, logx.NewRedactor())
	logger, err := logx.New(set.FS, set.Clock, "/log", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	reporter := NewFailureReporter(diag.NewReporter(logger, records), c, records)
	failure := errors.New("sync failed: Authorization: Bearer CANARY-TOKEN-7f3a9c")
	ref := reporter.Report(safego.Failure{Name: "clipboard.poll", Err: failure})
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	batches := sentBatches(t, set)
	event := batches[0].Batch[0]
	if event.Event != EventException || event.Properties["support_ref"] != ref {
		t.Fatalf("event = %+v, want an exception for %s", event, ref)
	}
	message, _ := event.Properties["message"].(string)
	if strings.Contains(message, "CANARY-TOKEN") {
		t.Fatalf("message carried the canary: %q", message)
	}
	if event.Properties["component"] != "clipboard.poll" {
		t.Fatalf("component = %v", event.Properties["component"])
	}
}

func TestProblemReportCarriesOnlyItsReference(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	records := diag.NewRecorder(set.Clock, logx.NewRedactor())
	ref := ReportProblem(records, c)
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	event := sentBatches(t, set)[0].Batch[0]
	if event.Event != EventSupportReport || event.Properties["support_ref"] != ref {
		t.Fatalf("event = %+v, want a support report for %s", event, ref)
	}
	if _, ok := records.Find(ref); !ok {
		t.Fatal("the problem report is not in the recorder")
	}
}

func TestSenderFlushesOnItsTickAndWhenTenEventsWait(t *testing.T) {
	set := fake.NewSet()
	sent := make(chan struct{}, 4)
	transport := &signalingHTTP{inner: set.HTTP, sent: sent}
	c := New(Config{Endpoint: testEndpoint, APIKey: IngestKey, Transport: transport, FS: set.FS, Clock: set.Clock, Dir: "/config", Version: "0.9.0", OS: "darwin", Arch: "arm64", Consent: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	set.Clock.AwaitTimerDueIn(flushInterval)
	c.AppLaunched()
	set.Clock.Advance(flushInterval)
	<-sent
	for range flushAt {
		c.AppLaunched()
	}
	<-sent
}

// signalingHTTP tells the test each request was answered, so the test waits on the send, not on time.
type signalingHTTP struct {
	inner *fake.HTTP
	sent  chan struct{}
}

func (s *signalingHTTP) Do(req *http.Request) (*http.Response, error) {
	resp, err := s.inner.Do(req)
	s.sent <- struct{}{}
	return resp, err
}

var _ platform.HTTP = (*signalingHTTP)(nil)

func TestEveryEventSchemaAvoidsContentFields(t *testing.T) {
	forbidden := regexp.MustCompile(`text|content|clipboard|prompt|path|file|name|email|token|secret|url|screen|title|query`)
	for event, keys := range schema {
		for _, key := range keys {
			if forbidden.MatchString(key) {
				t.Errorf("%s allows key %q, which can carry content", event, key)
			}
		}
	}
}

func TestCommandVariantsAreAClosedSetOfIDs(t *testing.T) {
	if id, ok := CommandVariant("Clipboard History"); !ok || id != "clipboard-history" {
		t.Fatalf("CommandVariant(Clipboard History) = %q, %v", id, ok)
	}
	if _, ok := CommandVariant("My Snippet Titled Tax Notes"); ok {
		t.Fatal("a user-chosen title became a variant")
	}
	idShape := regexp.MustCompile(`^[a-z0-9-]+$`)
	for title, id := range commandVariants {
		if !idShape.MatchString(id) {
			t.Errorf("variant %q for %q is not an id", id, title)
		}
	}
}

func TestChannelSeparatesStableFromPrereleaseBuilds(t *testing.T) {
	cases := map[string]string{
		"0.9.0":         "stable",
		"2.0.0-go":      "stable",
		"2026.1030.911": "prerelease",
		"dev":           "prerelease",
		"":              "prerelease",
	}
	for version, want := range cases {
		if got := Channel(version); got != want {
			t.Errorf("Channel(%q) = %q, want %q", version, got, want)
		}
	}
}
