package app

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestTrackedLoginHeldCompatibility(t *testing.T) {
	for _, mode := range []string{"accepted", "refused", "cancelled", "startRejected"} {
		t.Run(mode, func(t *testing.T) {
			set := fake.NewSet()
			logger, err := logx.New(nil, set.Clock, "", logx.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Close()
			changed := make(chan struct{}, 8)
			session, err := auth.New(auth.Deps{HTTP: set.HTTP, Clock: set.Clock, Keychain: set.Keychain, Logger: logger, DeviceLabel: "private-canary@example.test token-private-canary code-private-canary", OnChange: func() { changed <- struct{}{} }})
			if err != nil {
				t.Fatal(err)
			}
			manifest, err := os.ReadFile("../../testdata/contracts/desktop-v1/compatibility-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "refused" {
				manifest = []byte(strings.Replace(string(manifest), `"compatible": true`, `"compatible": false`, 1))
			}
			entered, release, deviceEntered := make(chan struct{}), make(chan struct{}), make(chan struct{})
			set.HTTP.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/compatibility" {
					close(entered)
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(manifest)
					return
				}
				close(deviceEntered)
				<-r.Context().Done()
			}))
			collector := fake.NewSet()
			collector.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
			client := analytics.New(analytics.Config{Endpoint: "https://collector.test", APIKey: analytics.IngestKey, Transport: collector.HTTP, FS: collector.FS, Clock: collector.Clock, Dir: "/analytics", Consent: true})
			queue := make(completionUIQueue, 4)
			p := &App{account: session, commandTiming: timing.New(collector.Clock, client), uiQueue: queue}
			// Keep a known compatibility value so an unchanged manifest does not enqueue a menu refresh.
			p.compat.record(session.Snapshot().BaseURL, true)
			host := &recordingHost{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			starts := 0
			if mode == "accepted" {
				p.trackedRun(p.signInOrOutCommands()[0])(host)
			} else if mode == "startRejected" {
				host.Push(p.accountView())
				trackedNativeRun(p.commandTiming, "", func(ctx context.Context, done func(analytics.OperationOutcome)) {
					p.startCheckedSignInWithCompletion(ctx, host, func() error { starts++; return auth.ErrSignInInProgress }, done)
				})(ctx)
			} else {
				trackedNativeRun(p.commandTiming, "", func(ctx context.Context, done func(analytics.OperationOutcome)) {
					p.signInFromRootWithCompletion(ctx, host, done)
				})(ctx)
			}
			<-entered
			flushNativeAnalytics(t, client)
			if ops, uses := nativeCompletionEvents(t, collector); len(ops) != 0 || len(uses) != 0 {
				t.Fatalf("early events: %v %v", ops, uses)
			}
			if mode == "cancelled" {
				cancel()
			}
			close(release)
			(<-queue)()
			if mode == "refused" {
				(<-queue)()
			}
			flushNativeAnalytics(t, client)
			ops, uses := nativeCompletionEvents(t, collector)
			want := map[string]string{"accepted": "success", "refused": "failed", "cancelled": "cancelled", "startRejected": "failed"}[mode]
			wantUses := 1
			if mode == "cancelled" {
				wantUses = 0
			}
			if len(host.pushed) != 1 || len(ops) != 1 || ops[0]["outcome"] != want || len(uses) != wantUses {
				t.Fatalf("views=%d operations=%v uses=%v", len(host.pushed), ops, uses)
			}
			if wantUses == 1 && (uses[0]["variant"] != nil || uses[0]["outcome"] != want || ops[0]["$session_id"] == nil || uses[0]["$session_id"] != ops[0]["$session_id"]) {
				t.Fatalf("events: %v %v", ops, uses)
			}
			if mode == "startRejected" && starts != 1 {
				t.Fatalf("start calls = %d", starts)
			}
			if mode == "accepted" {
				<-deviceEntered
				<-changed
				session.CancelSignIn()
				<-changed
				if session.Snapshot().SignIn.Phase != auth.PhaseCancelled {
					t.Fatal("flow did not cancel")
				}
			} else if len(set.HTTP.Requests()) != 1 {
				t.Fatalf("unexpected requests: %v", set.HTTP.Requests())
			}
			for _, req := range collector.HTTP.Requests() {
				if strings.Contains(string(req.Body), "private-canary") {
					t.Fatal("private data in analytics")
				}
			}
		})
	}
}
