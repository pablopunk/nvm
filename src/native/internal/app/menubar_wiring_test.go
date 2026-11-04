package app

import (
	"net/http"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/update"
	updatefake "github.com/pablopunk/nvm/native/internal/update/fake"
)

// wiredApp is a golden palette with the recorder, the analytics client and the updater attached over fakes. Its
// configuration directory is a temporary folder, so a setting saved in a test touches no fixture or real file.
func wiredApp(t *testing.T) (*App, *fake.Set) {
	t.Helper()
	p := goldenPalette(t)
	previous := configDir
	configDir = func() string { return t.TempDir() }
	t.Cleanup(func() { configDir = previous })
	set := fake.NewSet()
	p.recorder = diag.NewRecorder(set.Clock, logx.NewRedactor())
	p.analytics = analytics.New(analytics.Config{
		Endpoint: analytics.IngestEndpoint, APIKey: analytics.IngestKey, Transport: set.HTTP, FS: set.FS,
		Clock: set.Clock, Dir: "/config", Version: nativeClientVersion, OS: "linux", Arch: "amd64", Consent: true,
	})
	p.ports = set.Ports()
	p.updater = updatefake.New(nativeClientVersion, update.State{Status: update.StatusIdle, CurrentVersion: nativeClientVersion})
	return p, set
}

// stubWindow is the palette's window without a display: it records whether the palette is shown.
type stubWindow struct {
	shown bool
}

func (w *stubWindow) Update(fn func())     { fn() }
func (w *stubWindow) Show()                { w.shown = true }
func (w *stubWindow) Hide()                { w.shown = false }
func (w *stubWindow) IsVisible() bool      { return w.shown }
func (w *stubWindow) IsFocused() bool      { return w.shown }
func (w *stubWindow) Focus() {}
func (w *stubWindow) Size() (int, int)     { return width, height }
func (w *stubWindow) SetPosition(x, y int) {}

func TestMenuBarCreatesItsItemOverTheTrayHostAndShowsTheTimer(t *testing.T) {
	p, set := wiredApp(t)
	p.setUpMenubar()
	trays := set.Tray.Trays()
	if len(trays) != 1 || len(trays[0].Items()) != 6 {
		t.Fatalf("tray items = %d, rows = %d", len(trays), len(trays[0].Items()))
	}
	p.setTrayTitle("04:59")
	if trays[0].Title() != "04:59" {
		t.Fatalf("timer title = %q", trays[0].Title())
	}
}

func TestMenuSettingsRowOpensTheSettingsPage(t *testing.T) {
	p, set := wiredApp(t)
	window := &stubWindow{}
	p.Attach(window)
	p.setUpMenubar()
	if err := set.Tray.Trays()[0].Invoke("settings"); err != nil {
		t.Fatalf("Invoke settings: %v", err)
	}
	if top := p.Top(); top.Title != "Settings" {
		t.Fatalf("top page = %q, want Settings", top.Title)
	}
	if !window.shown {
		t.Fatal("the settings page did not show the palette")
	}
}

func TestOnlyKnownCommandTitlesAreCountedAsVariants(t *testing.T) {
	p, _ := wiredApp(t)
	noop := func(search.Host) {}
	p.trackedRun(command{Title: "Clipboard History", Run: noop})(p)
	p.trackedRun(command{Title: "My Private Snippet Name", Run: noop})(p)
	if p.analytics.Pending() != 2 {
		t.Fatalf("pending = %d, want 2 feature events", p.analytics.Pending())
	}
}

func TestReportProblemRecordsAReferenceAndQueuesOneReport(t *testing.T) {
	p, set := wiredApp(t)
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	ref := analytics.ReportProblem(p.recorder, p.analytics)
	if _, ok := p.recorder.Find(ref); !ok {
		t.Fatal("the problem report is not in the recorder")
	}
	if p.analytics.Pending() != 1 {
		t.Fatalf("pending = %d, want the report queued", p.analytics.Pending())
	}
}

func TestToggleDiagnosticsWithdrawsConsentAndDropsTheQueue(t *testing.T) {
	p, _ := wiredApp(t)
	p.analytics.AppLaunched()
	p.toggleDiagnostics()
	if !p.S.DiagnosticsOff || p.analytics.Consent() || p.analytics.Pending() != 0 {
		t.Fatalf("off %v consent %v pending %d", p.S.DiagnosticsOff, p.analytics.Consent(), p.analytics.Pending())
	}
	p.toggleDiagnostics()
	if p.S.DiagnosticsOff || !p.analytics.Consent() {
		t.Fatal("toggling back did not turn consent on")
	}
}

func TestCheckForUpdatesReportsTheUpdatersAnswerOnTheUIThread(t *testing.T) {
	p, _ := wiredApp(t)
	got := make(chan string, 1)
	p.checkForUpdates(func(msg string) { got <- msg })
	if msg := <-got; msg != "Nevermind is up to date" {
		t.Fatalf("message = %q", msg)
	}
}
