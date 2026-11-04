package app

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	paldiag "github.com/pablopunk/nvm/native/internal/palette/diagnostics"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// releaseBuild is "true" in release builds, the only builds with an analytics endpoint. It is set at link time, so a
// development build never sends.
var releaseBuild = "false"

// quitFlushTimeout bounds the diagnostics sent while the app quits, so an unreachable endpoint cannot hold the quit.
const quitFlushTimeout = 2 * time.Second

// logDirectoryName is the folder under the config directory that holds the log file.
const logDirectoryName = "logs"

// openDiagnostics builds the log file, the recorder, the analytics client and the failure reporter. It runs before the
// store opens, so a failure in any background call from the start is recorded.
func (p *App) openDiagnostics(ports platform.Ports) {
	openLogFile(ports)
	p.recorder = diag.NewRecorder(searchClock, logx.NewRedactor())
	p.analytics = analytics.New(analytics.Config{
		Endpoint:  analyticsEndpoint(),
		APIKey:    analytics.IngestKey,
		Transport: ports.HTTP,
		FS:        ports.FS,
		Clock:     ports.Clock,
		Dir:       dataDir(),
		Version:   nativeClientVersion,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Consent:   !p.diagnosticsOff(),
	})
	p.failures = analytics.NewFailureReporter(diag.NewReporter(logx.Default(), p.recorder), p.analytics, p.recorder)
	p.startAnalytics()
}

// openLogFile writes the log to a size-rotated file under the config directory and mirrors it to stderr, so a
// development run still shows it in the terminal.
func openLogFile(ports platform.Ports) {
	logger, err := logx.New(ports.FS, ports.Clock, filepath.Join(dataDir(), logDirectoryName), logx.Options{Stderr: os.Stderr})
	if err != nil {
		log.Println("log file unavailable:", err)
		return
	}
	logx.SetDefault(logger)
}

// analyticsEndpoint is the ingest route in release builds, and nowhere otherwise.
func analyticsEndpoint() string {
	if releaseBuild == "true" {
		return analytics.IngestEndpoint
	}
	return ""
}

// startAnalytics runs the sender and records the launch. On quit the sender stops first, so an in-flight request does
// not hold the quit, and then what is queued gets one bounded flush.
func (p *App) startAnalytics() {
	ctx, stop := context.WithCancel(context.Background())
	p.analytics.Start(ctx)
	mygo.App.OnQuit(func() {
		stop()
		flushCtx, cancel := context.WithTimeout(context.Background(), quitFlushTimeout)
		defer cancel()
		_ = p.analytics.Flush(flushCtx)
	})
	p.analytics.AppLaunched()
}

// installFailureReporting makes the app's failure reporting the safego reporter. flushOnPanic, when the store is open,
// writes unsaved drafts before a panic is reported.
func (p *App) installFailureReporting(flushOnPanic func()) {
	safego.SetDefaults(safego.Defaults{
		Clock: searchClock,
		Reporter: safego.ReporterFunc(func(f safego.Failure) string {
			if f.Panic && flushOnPanic != nil {
				flushOnPanic()
			}
			return p.failures.Report(f)
		}),
	})
}

// trackedRun runs a palette command and counts it as feature use, under the command's closed-set id. A command outside
// that set is counted without an id.
func (p *App) trackedRun(cmd command) func(h search.Host) {
	return func(h search.Host) {
		id, _ := analytics.CommandVariant(cmd.Title)
		p.analytics.FeatureUsed(analytics.FeatureCommandRun, id)
		cmd.Run(h)
	}
}

// diagnosticsView is the Recent Errors page over the app's recorder.
func (p *App) diagnosticsView() *palette.View {
	var records paldiag.Records = paldiag.Static{}
	if p.recorder != nil {
		records = p.recorder
	}
	return paldiag.View(records, p.reportingState, p.copyText)
}

// reportingState says how records are reported now, for the page's labels.
func (p *App) reportingState() analytics.Reporting {
	return p.analytics.Reporting()
}

// reportProblem records a problem report, shows its reference, and lists the recent errors where it appears.
func (p *App) reportProblem(h search.Host) {
	ref := analytics.ReportProblem(p.recorder, p.analytics)
	h.Toast("Problem report " + ref + " recorded")
	h.Push(p.diagnosticsView())
}

// toggleDiagnostics flips "Send Technical Diagnostics". The client gets the new consent at once, so a withdrawal drops
// what is queued before the next batch goes out.
func (p *App) toggleDiagnostics() {
	off := !p.diagnosticsOff()
	if err := p.setDiagnosticsOff(off); err != nil {
		log.Println("diagnostics setting not saved:", err)
	}
	p.analytics.SetConsent(!off)
}

// diagnosticsLabel is the Settings accessory for the consent switch.
func diagnosticsLabel(off bool) string {
	if off {
		return "Off"
	}
	return "On"
}
