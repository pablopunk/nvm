package update_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/update"
	updatefake "github.com/pablopunk/nvm/native/internal/update/fake"
)

const running = "2.0.0"

// recorder keeps every state a service reports. A test waits for the status it expects and can read the history.
type recorder struct {
	mu      sync.Mutex
	history []update.State
	cursor  int
	wake    chan struct{}
}

func newRecorder() *recorder {
	return &recorder{wake: make(chan struct{}, 1)}
}

func (r *recorder) OnChange(s update.State) {
	r.mu.Lock()
	r.history = append(r.history, s)
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *recorder) await(t *testing.T, want update.Status) update.State {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		r.mu.Lock()
		for ; r.cursor < len(r.history); r.cursor++ {
			if s := r.history[r.cursor]; s.Status == want {
				r.cursor++
				r.mu.Unlock()
				return s
			}
		}
		r.mu.Unlock()
		select {
		case <-r.wake:
		case <-timeout:
			t.Fatalf("no %q state", want)
		}
	}
}

func (r *recorder) all() []update.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]update.State(nil), r.history...)
}

type memoryChannels struct {
	channel update.Channel
	saved   []update.Channel
}

func (m *memoryChannels) Channel() update.Channel { return m.channel }

func (m *memoryChannels) SetChannel(c update.Channel) error {
	m.channel = c
	m.saved = append(m.saved, c)
	return nil
}

func newService(e update.Engine, channels update.ChannelStore) (*update.Service, *recorder) {
	rec := newRecorder()
	svc := update.NewService(update.Options{Engine: e, Clock: fake.NewClock(), Channels: channels, Current: running, OnChange: rec.OnChange})
	return svc, rec
}

func offering(version string) updatefake.Check {
	return updatefake.Check{Offer: update.Offer{Version: version}, Found: true}
}

func installed(total int64) updatefake.Install {
	return updatefake.Install{Progress: []int64{total / 2, total}, Total: total}
}

func TestNewerVersionDownloadsInTheBackgroundAndWaitsForARestart(t *testing.T) {
	e := updatefake.NewEngine([]updatefake.Check{offering("2.0.1")}, installed(100))
	svc, rec := newService(e, nil)

	svc.Check(context.Background())
	ready := rec.await(t, update.StatusDownloaded)
	if ready.LatestVersion != "2.0.1" || e.Installs() != 1 {
		t.Fatalf("ready = %+v, installs = %d", ready, e.Installs())
	}
	if e.Relaunches() != 0 {
		t.Fatal("a finished download must not restart the app by itself")
	}
	if err := svc.RestartToUpdate(); err != nil || e.Relaunches() != 1 {
		t.Fatalf("restart = %v, relaunches = %d", err, e.Relaunches())
	}
}

func TestSameAndOlderVersionsAreNeverOfferedOrDownloaded(t *testing.T) {
	for _, version := range []string{running, "1.9.0", "2.0.0-next.3"} {
		e := updatefake.NewEngine([]updatefake.Check{offering(version)})
		svc, rec := newService(e, nil)
		if got := svc.Check(context.Background()); got.Status != update.StatusCurrent {
			t.Fatalf("offer %s: status = %q, want current", version, got.Status)
		}
		rec.await(t, update.StatusCurrent)
		if e.Installs() != 0 {
			t.Fatalf("offer %s was downloaded", version)
		}
	}
}

func TestNoNewerVersionIsUpToDate(t *testing.T) {
	e := updatefake.NewEngine([]updatefake.Check{{Found: false}})
	svc, _ := newService(e, nil)
	if got := svc.Check(context.Background()); got.Status != update.StatusCurrent || update.Message(got) != "Nevermind is up to date" {
		t.Fatalf("state = %+v", got)
	}
}

// TestBuildThatCannotUpdateSaysSoBeforeAnyCheck: the page must not show "up to date" or "not checked" in a build with no
// updater, and a channel switch there is refused with the reason instead of being saved silently.
func TestBuildThatCannotUpdateSaysSoBeforeAnyCheck(t *testing.T) {
	svc, _ := newService(updatefake.Disabled(), nil)
	if got := svc.State(); got.Status != update.StatusUnsupported || update.Message(got) != "Updates are not available in this build yet" {
		t.Fatalf("state = %+v, want unsupported before any check", got)
	}
	if err := svc.SetChannel(update.ChannelNightly); !errors.Is(err, update.ErrChannelUnavailable) {
		t.Fatalf("switch = %v, want ErrChannelUnavailable", err)
	}
	if svc.Channel() != update.ChannelStable {
		t.Fatalf("channel = %q after a refused switch, want stable", svc.Channel())
	}
}

func TestNetworkFailureReportsTheCheckStage(t *testing.T) {
	e := updatefake.NewEngine([]updatefake.Check{{Err: errors.New("offline")}})
	svc, _ := newService(e, nil)
	got := svc.Check(context.Background())
	if got.Status != update.StatusFailed || got.Stage != update.StageCheck || update.Message(got) != "Update check failed: offline" {
		t.Fatalf("state = %+v", got)
	}
	if e.Installs() != 0 {
		t.Fatal("a failed check must not download")
	}
}

func TestBadSignatureIsRefusedAndTheNextCheckRetries(t *testing.T) {
	bad := updatefake.Install{Total: 100, Err: errors.New("the update is not signed with the app's key")}
	e := updatefake.NewEngine([]updatefake.Check{offering("2.0.1")}, bad, installed(100))
	svc, rec := newService(e, nil)

	svc.Check(context.Background())
	failed := rec.await(t, update.StatusFailed)
	if failed.Stage != update.StageInstall || update.Message(failed) != "Update install failed: the update is not signed with the app's key" {
		t.Fatalf("failed = %+v", failed)
	}
	if err := svc.RestartToUpdate(); !errors.Is(err, update.ErrNotReady) {
		t.Fatalf("restart after a refused update = %v, want ErrNotReady", err)
	}

	svc.Check(context.Background())
	rec.await(t, update.StatusDownloaded)
	if e.Installs() != 2 {
		t.Fatalf("installs = %d, want the retry too", e.Installs())
	}
}

func TestPartialDownloadFailsWithTheBytesItGot(t *testing.T) {
	partial := updatefake.Install{Progress: []int64{40}, Total: 100, Err: errors.New("downloaded 40 bytes, want 100")}
	e := updatefake.NewEngine([]updatefake.Check{offering("2.0.1")}, partial)
	svc, rec := newService(e, nil)
	svc.Check(context.Background())
	failed := rec.await(t, update.StatusFailed)
	if update.Message(failed) != "Update install failed: downloaded 40 bytes, want 100" {
		t.Fatalf("message = %q", update.Message(failed))
	}
}

func TestDownloadProgressReportsTheFraction(t *testing.T) {
	steps := updatefake.Install{Progress: []int64{10, 10, 50, 100}, Total: 100}
	e := updatefake.NewEngine([]updatefake.Check{offering("2.0.1")}, steps)
	svc, rec := newService(e, nil)
	svc.Check(context.Background())
	rec.await(t, update.StatusDownloaded)
	var fractions []float64
	for _, s := range rec.all() {
		if s.Status == update.StatusDownloading {
			fractions = append(fractions, s.Progress)
		}
	}
	want := []float64{0, 0.1, 0.5, 1}
	if !slices.Equal(fractions, want) {
		t.Fatalf("fractions = %v, want %v: the repeated 10 percent is not announced again", fractions, want)
	}
}

func TestAChecksInProgressOrAnUpdateInProgressAreNotDuplicated(t *testing.T) {
	e := updatefake.NewEngine([]updatefake.Check{offering("2.0.1")}, updatefake.Install{Total: 100, Block: true})
	svc, rec := newService(e, nil)
	svc.Check(context.Background())
	rec.await(t, update.StatusDownloading)
	if got := svc.Check(context.Background()); got.Status != update.StatusDownloading {
		t.Fatalf("check during a download = %q", got.Status)
	}
	if e.Checks() != 1 {
		t.Fatalf("checks = %d; a busy service must not ask the feed again", e.Checks())
	}
}

func TestUnsupportedBuildNeverContactsTheFeed(t *testing.T) {
	e := updatefake.Disabled()
	svc, _ := newService(e, nil)
	got := svc.Check(context.Background())
	if got.Status != update.StatusUnsupported || update.Message(got) != "Updates are not available in this build yet" {
		t.Fatalf("state = %+v", got)
	}
	if e.Checks() != 0 {
		t.Fatal("a build without updates asked the feed")
	}
}

func TestSwitchingChannelPersistsItDropsADownloadAndFollowsPrereleases(t *testing.T) {
	channels := &memoryChannels{channel: update.ChannelStable}
	e := updatefake.NewEngine([]updatefake.Check{offering("2.0.1")}, updatefake.Install{Total: 100, Block: true})
	svc, rec := newService(e, channels)
	svc.Check(context.Background())
	rec.await(t, update.StatusDownloading)

	if err := svc.SetChannel(update.ChannelNightly); err != nil {
		t.Fatal(err)
	}
	if got := svc.State(); got.Status != update.StatusIdle {
		t.Fatalf("state after the switch = %+v, want idle", got)
	}
	if !e.Prereleases() || svc.Channel() != update.ChannelNightly || len(channels.saved) != 1 {
		t.Fatalf("prereleases = %v, channel = %q, saved = %v", e.Prereleases(), svc.Channel(), channels.saved)
	}
}

func TestChannelChoiceIsRestoredFromTheStore(t *testing.T) {
	channels := &memoryChannels{channel: update.ChannelNightly}
	e := updatefake.NewEngine(nil)
	newService(e, channels)
	if !e.Prereleases() {
		t.Fatal("a nightly store must start with prereleases on")
	}
}

func TestAChannelSwitchIsRefusedWhileAnUpdateWaitsForItsRestart(t *testing.T) {
	e := updatefake.NewEngine([]updatefake.Check{offering("2.0.1")}, installed(100))
	svc, rec := newService(e, &memoryChannels{channel: update.ChannelStable})
	svc.Check(context.Background())
	rec.await(t, update.StatusDownloaded)
	if err := svc.SetChannel(update.ChannelNightly); !errors.Is(err, update.ErrUpdateReady) {
		t.Fatalf("switch = %v, want ErrUpdateReady", err)
	}
	if svc.Channel() != update.ChannelStable {
		t.Fatal("a refused switch changed the channel")
	}
}

func TestRestartIsRefusedWithoutAnUpdate(t *testing.T) {
	svc, _ := newService(updatefake.NewEngine(nil), nil)
	if err := svc.RestartToUpdate(); !errors.Is(err, update.ErrNotReady) {
		t.Fatalf("restart = %v, want ErrNotReady", err)
	}
}

func TestRunChecksAfterTheStartupDelayThenEveryInterval(t *testing.T) {
	clock := fake.NewClock()
	e := updatefake.NewEngine([]updatefake.Check{{Found: false}})
	rec := newRecorder()
	svc := update.NewService(update.Options{Engine: e, Clock: clock, Current: running, OnChange: rec.OnChange})
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan struct{})
	go func() {
		svc.Run(ctx)
		close(done)
	}()

	clock.AwaitTimerDueIn(update.StartupDelay)
	if e.Checks() != 0 {
		t.Fatal("a check ran before the startup delay")
	}
	clock.Advance(update.StartupDelay)
	rec.await(t, update.StatusCurrent)
	clock.AwaitTimerDueIn(update.CheckInterval)
	clock.Advance(update.CheckInterval)
	rec.await(t, update.StatusCurrent)
	stop()
	<-done
	if e.Checks() != 2 {
		t.Fatalf("checks = %d, want the startup check and one interval check", e.Checks())
	}
}

func TestRunReturnsAtOnceInABuildThatCannotUpdate(t *testing.T) {
	svc := update.NewService(update.Options{Engine: updatefake.Disabled(), Clock: fake.NewClock(), Current: running})
	svc.Run(context.Background())
}
