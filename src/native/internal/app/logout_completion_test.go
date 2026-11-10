package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/egoist/mygo"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

type logoutHUDWindow struct {
	visible bool
	height  int
}

func (w *logoutHUDWindow) Update(fn func())     { fn() }
func (w *logoutHUDWindow) SetSize(_, h int)     { w.height = h }
func (w *logoutHUDWindow) SetPosition(_, _ int) {}
func (w *logoutHUDWindow) ShowInactive()        { w.visible = true }
func (w *logoutHUDWindow) Hide()                { w.visible = false }
func (w *logoutHUDWindow) IsVisible() bool      { return w.visible }

func TestTrackedLogoutCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, outcome string
		status        int
		cancel        bool
	}{
		{"success", "success", http.StatusOK, false},
		{"failed", "failed", http.StatusInternalServerError, false},
		{"cancelled", "cancelled", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) { testTrackedLogout(t, tc.outcome, tc.status, tc.cancel) })
	}
}

func testTrackedLogout(t *testing.T, outcome string, status int, cancelRequest bool) {
	set := fake.NewSet()
	logger, err := logx.New(nil, set.Clock, "", logx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()
	st, err := store.Open(set.FS, set.Clock, "/config")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	profile := auth.ProfileDocument(st)
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.Update(func(p *auth.Profile) { p.Connections[env.BaseURL] = auth.Connection{Email: "private@example.test"} }); err != nil {
		t.Fatal(err)
	}
	if err := auth.NewVault(set.Keychain).SaveToken(env.BaseURL, backend.Token("private-token")); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	set.HTTP.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		if cancelRequest {
			<-r.Context().Done()
		} else {
			<-release
			w.WriteHeader(status)
		}
	}))
	collector := fake.NewSet()
	collector.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
	client := analytics.New(analytics.Config{Endpoint: "https://collector.test", APIKey: analytics.IngestKey, Transport: collector.HTTP, FS: collector.FS, Clock: collector.Clock, Dir: "/analytics", Consent: true})
	session, err := auth.New(auth.Deps{HTTP: set.HTTP, Clock: set.Clock, Keychain: set.Keychain, Logger: logger, Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	queue := make(completionUIQueue, 1)
	hudWindow := &logoutHUDWindow{}
	p := &App{Palette: palette.New(palette.Options{}), panel: &mygo.Window{}, account: session, commandTiming: timing.New(collector.Clock, client), uiQueue: queue}
	p.hud = feedback.New(func() feedback.Window { return hudWindow }, set.Clock, func(_, _ int) (int, int) { return 0, 0 })
	host := &recordingHost{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	trackedNativeRun(p.commandTiming, "", func(ctx context.Context, complete func(analytics.OperationOutcome)) {
		p.signOutFromRootWithCompletion(ctx, host, complete)
	})(ctx)
	<-entered
	flushNativeAnalytics(t, client)
	if ops, uses := nativeCompletionEvents(t, collector); len(ops) != 0 || len(uses) != 0 {
		t.Fatalf("events before revoke: %v %v", ops, uses)
	}
	if cancelRequest {
		cancel()
	} else {
		close(release)
	}
	callback := <-queue
	callback()
	(<-queue)()
	tone := feedback.ToneSuccess
	if outcome != "success" {
		tone = feedback.ToneError
	}
	message := signOutMessage("private@example.test", outcome == "success")
	if !hudWindow.visible || hudWindow.height != feedback.Height(message, tone) {
		t.Fatalf("HUD visible=%v height=%d", hudWindow.visible, hudWindow.height)
	}
	flushNativeAnalytics(t, client)
	ops, uses := nativeCompletionEvents(t, collector)
	if len(ops) != 1 || len(uses) != map[bool]int{true: 0, false: 1}[cancelRequest] {
		t.Fatalf("events: %v %v", ops, uses)
	}
	if ops[0]["outcome"] != outcome || (!cancelRequest && (uses[0]["outcome"] != outcome || uses[0]["variant"] != nil || ops[0]["$session_id"] != uses[0]["$session_id"] || ops[0]["$session_id"] == nil)) {
		t.Fatalf("unexpected events: %v %v", ops, uses)
	}
	for _, req := range collector.HTTP.Requests() {
		if strings.Contains(string(req.Body), "private") {
			t.Fatal("private data in analytics")
		}
	}
	if _, err := auth.NewVault(set.Keychain).LoadToken(env.BaseURL); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("token remains: %v", err)
	}
	if session.Snapshot().Connected {
		t.Fatal("session remains connected")
	}
	if len(set.HTTP.Requests()) != 1 {
		t.Fatalf("revoke calls: %d", len(set.HTTP.Requests()))
	}
}

func TestTrackedLogoutWithoutSessionBlocked(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: 200})
	client := analytics.New(analytics.Config{Endpoint: "https://collector.test", APIKey: analytics.IngestKey, Transport: set.HTTP, FS: set.FS, Clock: set.Clock, Dir: "/analytics", Consent: true})
	p := &App{commandTiming: timing.New(set.Clock, client)}
	p.trackedRun(command{Title: "Log out of Nevermind", Complete: p.signOutFromRootWithCompletion})(&recordingHost{})
	flushNativeAnalytics(t, client)
	ops, uses := nativeCompletionEvents(t, set)
	if len(ops) != 1 || ops[0]["outcome"] != "blocked" || len(uses) != 1 || uses[0]["outcome"] != "blocked" {
		t.Fatalf("events: %v", ops)
	}
}
