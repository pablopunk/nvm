//go:build rig

package rigtest

import (
	"encoding/json"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/perf"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The performance measurements of PLAN.md section 3.2 (WP0.5). They launch the app built with
// -tags automation,perfdebug, measure it from outside, and write perf-report.json. They run only
// with NVM_RIG_PERF=1; scripts/rig/perf.sh sets it, builds both binaries, and compares the report
// with testdata/budgets.json.
//
// Environment: NVM_RIG_BIN is the rig build, NVM_PERF_SHIPPED_BIN the default build (B6),
// NVM_PERF_REPORT the report path (default: the test's artifacts directory), NVM_PERF_LABEL and
// NVM_PERF_RENDERER describe the machine in the report.

const (
	perfGate = "NVM_RIG_PERF"
	// perfSettle is how long the app runs after "ready" before the measurements start. The file index
	// of the J29 data set starts two seconds after ready and walks 50k files, so it must finish first.
	perfSettle = 4 * time.Second
	// launchPoll is how often the launch measurement reads the log. It sets the resolution of B3.
	launchPoll  = 5 * time.Millisecond
	bytesPerMiB = 1 << 20
)

// perfRecorder collects the measurements of one run.
type perfRecorder struct {
	report perf.Report
}

func (r *perfRecorder) set(metric string, value float64, unit string, samples []float64, note string) {
	r.report.Measurements[metric] = perf.Measurement{Value: value, Unit: unit, Samples: samples, Note: note}
}

func newPerfRecorder(t *testing.T) *perfRecorder {
	t.Helper()
	kernel, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		kernel = []byte("unknown")
	}
	label := os.Getenv("NVM_PERF_LABEL")
	if label == "" {
		label = "unlabelled"
	}
	return &perfRecorder{report: perf.Report{
		Environment: perf.Environment{
			Label:     label,
			Renderer:  "unknown",
			CPUs:      runtime.NumCPU(),
			Kernel:    strings.TrimSpace(string(kernel)),
			GoVersion: runtime.Version(),
			Display:   os.Getenv("DISPLAY"),
			Recorded:  time.Now().UTC().Format(time.RFC3339),
		},
		Measurements: map[string]perf.Measurement{},
	}}
}

// TestPerfRig measures the budgets B1 to B7 and writes the report. The soak length is set with
// -args -soak=<duration>.
func TestPerfRig(t *testing.T) {
	if os.Getenv(perfGate) != "1" {
		t.Skipf("set %s=1 to measure the budgets (scripts/rig/perf.sh)", perfGate)
	}
	rec := newPerfRecorder(t)
	t.Cleanup(func() { writePerfReport(t, rec.report) })
	t.Run("B6 binary size", func(t *testing.T) { measureBinarySize(t, rec) })
	t.Run("interactive", func(t *testing.T) { measureInteractive(t, rec) })
	t.Run("B7 soak", func(t *testing.T) { measureSoak(t, rec) })
}

func measureInteractive(t *testing.T, rec *perfRecorder) {
	app, ready := launchPerfApp(t, "")
	rec.noteRenderer(app.Log())
	rec.set("B3.cold_start_ready_ms", ms(ready), "ms", nil,
		"process start to the ready log line; fresh HOME seeded with the J29 clipboard and file counts; binary already in the page cache")
	time.Sleep(perfSettle)
	t.Run("B4 idle hidden", func(t *testing.T) { measureIdleHidden(t, app, rec) })
	t.Run("B1 hotkey to frame", func(t *testing.T) { measureHotkeyToFrame(t, app, rec) })
	t.Run("B2 keystroke to results", func(t *testing.T) { measureKeystrokeToResults(t, app, rec) })
	t.Run("B5 shown idle", func(t *testing.T) { measureShownIdle(t, app, rec) })
}

func measureBinarySize(t *testing.T, rec *perfRecorder) {
	shipped := os.Getenv("NVM_PERF_SHIPPED_BIN")
	if shipped == "" {
		t.Fatal("NVM_PERF_SHIPPED_BIN is empty: build the default binary and point it there (scripts/rig/perf.sh)")
	}
	rec.set("B6.binary_mib", fileMiB(t, shipped), "MiB", nil, "default build (what ships); no automation, no perfdebug")
	rec.set("B6.rig_binary_mib", fileMiB(t, os.Getenv("NVM_RIG_BIN")), "MiB", nil,
		"rig build with -tags automation,perfdebug; information only")
}

func fileMiB(t *testing.T, path string) float64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return float64(info.Size()) / bytesPerMiB
}

// launchPerfApp starts the rig binary like Launch does and returns when its log says ready. Unlike
// Launch it polls at launchPoll, so the returned duration is the start-up time to within that poll.
// debugAddr, when set, is the loopback address of the perfdebug counters endpoint.
func launchPerfApp(t *testing.T, debugAddr string) (*App, time.Duration) {
	t.Helper()
	bin := os.Getenv("NVM_RIG_BIN")
	if bin == "" {
		t.Fatal("NVM_RIG_BIN is empty: build the app with -tags automation,perfdebug and point it there")
	}
	display := os.Getenv("DISPLAY")
	if display == "" {
		t.Fatal("DISPLAY is empty: start Xvfb first (scripts/rig/entrypoint.sh)")
	}
	if debugAddr != "" {
		t.Setenv("NVM_PERF_DEBUG", debugAddr)
	}
	base, err := os.MkdirTemp("", "rig")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	a := &App{
		t:         t,
		display:   display,
		hotkey:    DefaultHotkey,
		base:      base,
		socket:    filepath.Join(base, "auto.sock"),
		artifacts: ArtifactsDir(t),
		done:      make(chan struct{}),
	}
	a.logPath = filepath.Join(a.artifacts, "app.log")
	home := filepath.Join(base, "home")
	runtimeDir := filepath.Join(base, "xdg")
	for _, dir := range []string{home, runtimeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	seedJ29(t, home)
	started := time.Now()
	a.start(t, bin, home, runtimeDir, "")
	t.Cleanup(a.Stop)
	deadline := started.Add(readyTimeout)
	for !hasLine(a.Log(), "ready") {
		if log := a.Log(); strings.Contains(log, shortcutFailureMarker) {
			t.Fatalf("hotkey registration failed; see %s:\n%s", a.logPath, tail(log, 15))
		}
		select {
		case <-a.done:
			t.Fatalf("app exited before ready; see %s:\n%s", a.logPath, tail(a.Log(), 15))
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("app not ready after %s; see %s", readyTimeout, a.logPath)
		}
		time.Sleep(launchPoll)
	}
	ready := time.Since(started)
	a.client, err = dialAutomation(a.socket, readyTimeout)
	if err != nil {
		t.Fatalf("automation socket: %v", err)
	}
	return a, ready
}

// perfFrameTimeout bounds one wait for a palette state in a measurement.
const perfFrameTimeout = 5 * time.Second

// paletteResults is the text of the palette's result rows, read from its elements. The search field
// is left out, so a change means the results changed and not only the typed text.
func paletteResults(a *App) (string, error) {
	w, err := a.PaletteWindow()
	if err != nil {
		return "", err
	}
	raw, err := a.client.call("tree", map[string]any{"window": w.ID})
	if err != nil {
		return "", err
	}
	var tree Tree
	if err := json.Unmarshal(raw, &tree); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, node := range tree.Nodes {
		if node.Role == "textField" {
			continue
		}
		b.WriteString(node.Label)
		b.WriteByte('\x1f')
		b.WriteString(node.Value)
		b.WriteByte('\x1e')
	}
	return b.String(), nil
}

// rendererMarker is the line MyGo logs when native UI draws in software; its parenthesized part names
// the renderer, such as "llvmpipe (LLVM 19.1.7, 256 bits)".
const rendererMarker = "OpenGL draws on the CPU ("

// noteRenderer records the renderer the app logged, when it logged one. Not every launch logs the
// line, so the report says "unknown" rather than guess.
func (r *perfRecorder) noteRenderer(log string) {
	for _, line := range strings.Split(log, "\n") {
		start := strings.Index(line, rendererMarker)
		if start < 0 {
			continue
		}
		rest := line[start+len(rendererMarker):]
		if end := strings.LastIndex(rest, ")"); end > 0 {
			r.report.Environment.Renderer = rest[:end]
		}
		return
	}
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// freeLoopbackAddr returns a loopback address nothing listens on, for the perfdebug endpoint.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

// writePerfReport writes the report next to the other artifacts, or to NVM_PERF_REPORT.
func writePerfReport(t *testing.T, report perf.Report) {
	t.Helper()
	path := os.Getenv("NVM_PERF_REPORT")
	if path == "" {
		path = filepath.Join(ArtifactsDir(t), "perf-report.json")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("perf report: %s (%d measurements)", path, len(report.Measurements))
}

// liveProc returns the machine's /proc; the measurements need Linux.
func liveProc(t *testing.T) fs.FS {
	t.Helper()
	procFS, err := perf.Live()
	if err != nil {
		t.Fatal(err)
	}
	return procFS
}

// wallClock is the real clock the CPU windows sleep on. Only Now and Sleep are used by the perf
// package, so the rest of the platform.Clock port is left unset.
type wallClock struct{ platform.Clock }

func (wallClock) Now() time.Time        { return time.Now() }
func (wallClock) Sleep(d time.Duration) { time.Sleep(d) }
