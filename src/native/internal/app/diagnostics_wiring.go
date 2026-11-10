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
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// releaseBuild is the legacy link-time release marker. Packaged builds also send from preview packaging; unpackaged
// development builds remain endpoint-free.
var releaseBuild = "false"

// analyticsAppVersion is linked to the packaged MyGo version; development uses the backend client version.
var analyticsAppVersion = nativeClientVersion

// analyticsSourceCommit is linked from packaging provenance; development builds omit it.
var analyticsSourceCommit = ""

// quitFlushTimeout bounds the diagnostics sent while the app quits, so an unreachable endpoint cannot hold the quit.
const quitFlushTimeout = 2 * time.Second

// logDirectoryName is the folder under the config directory that holds the log file.
const logDirectoryName = "logs"

// openDiagnostics builds the log file, the recorder, the analytics client and the failure reporter. It runs before the
// store opens, so a failure in any background call from the start is recorded.
func (p *App) openDiagnostics(ports platform.Ports) {
	openLogFile(ports)
	p.recorder = diag.NewRecorder(searchClock, logx.NewRedactor())
	osRelease := availableOSRelease(ports.OSRelease)
	p.analytics = analytics.New(analytics.Config{
		Endpoint:  analyticsEndpoint(packagedBuild()),
		APIKey:    analytics.IngestKey,
		Transport: ports.HTTP,
		FS:        ports.FS,
		Clock:     ports.Clock,
		Dir:       dataDir(),
		Version:   analyticsAppVersion,
		Release:   analyticsReleaseIdentity(analyticsAppVersion, analyticsSourceCommit, runtime.GOOS),
		OS:        runtime.GOOS,
		OSRelease: osRelease,
		Arch:      runtime.GOARCH,
		Consent:   !p.diagnosticsOff(),
	})
	p.commandTiming = timing.New(ports.Clock, p.analytics)
	p.failures = analytics.NewFailureReporter(diag.NewReporter(logx.Default(), p.recorder), p.analytics, p.recorder)
	p.startAnalytics()
}

// availableOSRelease omits the optional kernel release when the platform does not provide it or cannot read it.
func availableOSRelease(source platform.OSRelease) string {
	if source == nil {
		return ""
	}
	release, err := source.KernelRelease()
	if err != nil {
		return ""
	}
	return release
}

// openLogFile writes the log to a size-rotated file under the config directory and mirrors it to stderr, so a
// development run still shows it in the terminal. The standard library's log lines go to the same file at warn, so
// an app launched from Finder, whose stderr goes nowhere, keeps them.
func openLogFile(ports platform.Ports) {
	logger, err := logx.New(ports.FS, ports.Clock, filepath.Join(dataDir(), logDirectoryName), logx.Options{Stderr: os.Stderr})
	if err != nil {
		log.Println("log file unavailable:", err)
		return
	}
	logx.SetDefault(logger)
	log.SetFlags(0)
	log.SetOutput(logger.Writer(logx.Warn))
}

// analyticsEndpoint enables ingest for packaged builds and legacy release builds only.
func analyticsEndpoint(packaged bool) string {
	if packaged || releaseBuild == "true" {
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
		if cmd.Complete != nil {
			variant, _ := analytics.CommandVariant(cmd.Title)
			trackedNativeRun(p.commandTiming, variant, func(ctx context.Context, complete func(analytics.OperationOutcome)) {
				cmd.Complete(ctx, h, complete)
			})(context.Background())
			return
		}
		id, _ := analytics.CommandVariant(cmd.Title)
		p.analytics.FeatureUsed(analytics.FeatureCommandRun, id)
		cmd.Run(h)
	}
}

// reportingState says how records are reported now, for the page's labels.
func (p *App) reportingState() analytics.Reporting {
	return p.analytics.Reporting()
}

// toggleDiagnostics flips "Send Technical Diagnostics". The client gets the new consent at once, so a withdrawal drops
// what is queued before the next batch goes out.
func (p *App) toggleDiagnostics() {
	off := !p.diagnosticsOff()
	if err := p.setDiagnosticsOff(off); err != nil {
		log.Println("diagnostics setting not saved:", err)
	}
	p.updateAnalyticsConsent(!off)
}

// updateAnalyticsConsent applies a user-facing consent change and identifies the current owner on opt-in.
func (p *App) updateAnalyticsConsent(on bool) {
	if p.analytics == nil {
		return
	}
	p.analytics.SetConsent(on)
	if on {
		p.observeAnalyticsOwner()
	}
}
