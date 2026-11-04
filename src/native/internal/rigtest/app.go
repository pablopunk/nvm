//go:build rig

package rigtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	readyTimeout  = 30 * time.Second
	stopTimeout   = 5 * time.Second
	settleAfterUp = time.Second // documented workaround: the first hotkey press after "ready" can be lost
	// The app logs `shortcut <accelerator> <error>` when it cannot grab the hotkey.
	shortcutFailureMarker = " shortcut "
)

// Options configure one launch. Zero values keep the app's own defaults.
type Options struct {
	// Theme is NVM_THEME ("dark" forces the dark palette). Empty keeps the light default.
	Theme string
	// Hotkey is written to the isolated settings.json, for a rig that must not share a grab with
	// another app on the display. Empty keeps the app default, Alt+Shift+Space.
	Hotkey string
	// Home is the HOME of this launch, kept by the caller. A second launch with the same Home runs
	// the app over the state the first one left (J10). Empty gives the launch a fresh HOME.
	Home string
	// Run names this launch in the artifacts (app-<Run>.log), for a test that launches more than once.
	Run string
	// AllowUnboundHotkey lets the launch go on when the hotkey is taken, for a journey that starts a
	// second copy on the same display on purpose (J11). Launch otherwise fails on the log line.
	AllowUnboundHotkey bool
	// Backend is the origin of the fake backend the app talks to (StartFakeBackend). Empty points the app
	// at an unreachable port, so a launch never reaches the production backend.
	Backend string
}

// App is one launched copy of the app, isolated from the user's and from other launches: its own
// HOME, XDG_RUNTIME_DIR (which keys the single-instance socket), automation socket and log.
type App struct {
	t         *testing.T
	pid       int
	display   string
	hotkey    string
	base      string
	home      string
	socket    string
	artifacts string
	logPath   string
	done      chan struct{}
	exited    bool
	client    *automationClient
	// unboundOK is Options.AllowUnboundHotkey: a hotkey failure in the log is expected.
	unboundOK bool
}

// Launch starts NVM_RIG_BIN (the app built with -tags automation) detached on DISPLAY and waits
// until it logs "ready" and answers on its automation socket. The app is stopped at test cleanup.
func Launch(t *testing.T, opts Options) *App {
	t.Helper()
	bin := os.Getenv("NVM_RIG_BIN")
	if bin == "" {
		t.Fatal("NVM_RIG_BIN is empty: build the app with -tags automation and point it there (scripts/rig/README.md)")
	}
	display := os.Getenv("DISPLAY")
	if display == "" {
		t.Fatal("DISPLAY is empty: start Xvfb first (scripts/rig/entrypoint.sh)")
	}
	// A short base path keeps the Unix socket paths under the 108-byte limit.
	base, err := os.MkdirTemp("", "rig")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })

	hotkey := opts.Hotkey
	if hotkey == "" {
		hotkey = DefaultHotkey
	}
	a := &App{
		t:         t,
		display:   display,
		hotkey:    hotkey,
		base:      base,
		socket:    filepath.Join(base, "auto.sock"),
		artifacts: ArtifactsDir(t),
		done:      make(chan struct{}),
		unboundOK: opts.AllowUnboundHotkey,
	}
	a.logPath = filepath.Join(a.artifacts, "app.log")
	if opts.Run != "" {
		a.logPath = filepath.Join(a.artifacts, "app-"+sanitizeName(opts.Run)+".log")
	}
	home := filepath.Join(base, "home")
	if opts.Home != "" {
		home = opts.Home
	}
	a.home = home
	runtimeDir := filepath.Join(base, "xdg")
	for _, dir := range []string{home, runtimeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if opts.Hotkey != "" {
		if err := writeHotkeySettings(home, opts.Hotkey); err != nil {
			t.Fatal(err)
		}
	}
	a.start(t, bin, home, runtimeDir, opts.Theme, opts.Backend)
	t.Cleanup(a.Stop)
	a.waitReady(t)
	a.client, err = dialAutomation(a.socket, readyTimeout)
	if err != nil {
		t.Fatalf("automation socket: %v", err)
	}
	time.Sleep(settleAfterUp)
	return a
}

func (a *App) start(t *testing.T, bin, home, runtimeDir, theme, backend string) {
	t.Helper()
	logFile, err := os.Create(a.logPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(a.systemLog())
	cmd := exec.Command(bin)
	cmd.Dir = home
	cmd.Env = append(launchEnvironment(os.Environ(), home, runtimeDir, a.socket, theme, backend),
		"NVM_RIG_SYSTEM_LOG="+a.systemLog())
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start %s: %v", bin, err)
	}
	_ = logFile.Close()
	a.pid = cmd.Process.Pid
	t.Logf("launched pid %d on %s, artifacts in %s", a.pid, a.display, a.artifacts)
	go func() {
		_ = cmd.Wait()
		close(a.done)
	}()
}

// systemLog is the file the app records its system commands in: each launch starts with none.
func (a *App) systemLog() string {
	return filepath.Join(a.artifacts, "system-calls.log")
}

// unreachableBackend is the origin of a launch without a fake backend: the discard port refuses every connection.
const unreachableBackend = "http://127.0.0.1:9"

// launchEnvironment replaces the variables that decide where the app keeps its state and which backend it talks to,
// and keeps the display and session bus of the rig.
func launchEnvironment(parent []string, home, runtimeDir, socket, theme, backend string) []string {
	if backend == "" {
		backend = unreachableBackend
	}
	env := make([]string, 0, len(parent)+7)
	for _, kv := range parent {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "HOME", "XDG_RUNTIME_DIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "MYGO_AUTOMATION", "NVM_THEME", "XDG_ACTIVATION_TOKEN", "DESKTOP_STARTUP_ID", "NVM_RIG_BACKEND":
			continue
		}
		env = append(env, kv)
	}
	// XDG_DATA_HOME is the isolated HOME's data folder, so desktop entries and the Trash (which the desktop's trash
	// implementation writes under it) stay inside the temp HOME, never the user's.
	env = append(env,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"XDG_RUNTIME_DIR="+runtimeDir,
		"MYGO_AUTOMATION="+socket,
		"NVM_RIG_BACKEND="+backend,
	)
	if theme != "" {
		env = append(env, "NVM_THEME="+theme)
	}
	return env
}

func writeHotkeySettings(home, hotkey string) error {
	dir := filepath.Join(home, ".config", "nvm-native")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data := fmt.Sprintf("{\"Hotkey\": %q}\n", hotkey)
	return os.WriteFile(filepath.Join(dir, "settings.json"), []byte(data), 0o600)
}

// waitReady polls the app log for the "ready" line and fails early on a hotkey registration error
// or an exit.
func (a *App) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(readyTimeout)
	for {
		log := a.Log()
		if !a.unboundOK && strings.Contains(log, shortcutFailureMarker) {
			t.Fatalf("hotkey registration failed; see %s:\n%s", a.logPath, tail(log, 15))
		}
		if hasLine(log, "ready") {
			return
		}
		select {
		case <-a.done:
			t.Fatalf("app exited before ready; see %s:\n%s", a.logPath, tail(log, 15))
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("app not ready after %s; see %s:\n%s", readyTimeout, a.logPath, tail(log, 15))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Stop asks the app to quit (SIGTERM to its pid only), escalates to SIGKILL after a grace period
// and removes its socket. It is idempotent and runs at test cleanup.
func (a *App) Stop() {
	if a.pid == 0 || a.isExited() {
		return
	}
	_ = syscall.Kill(a.pid, syscall.SIGTERM)
	select {
	case <-a.done:
	case <-time.After(stopTimeout):
		_ = syscall.Kill(a.pid, syscall.SIGKILL)
		<-a.done
	}
	_ = os.Remove(a.socket)
}

// KillHard sends SIGKILL to the launched pid and waits for it to be reaped. Journeys use it to
// prove state survives a crash.
func (a *App) KillHard() error {
	if a.isExited() {
		return fmt.Errorf("pid %d already exited", a.pid)
	}
	if err := syscall.Kill(a.pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("kill -9 %d: %w", a.pid, err)
	}
	select {
	case <-a.done:
		return nil
	case <-time.After(stopTimeout):
		return fmt.Errorf("pid %d did not exit after SIGKILL", a.pid)
	}
}

// Pid is the launched process, for the logs and for KillHard.
func (a *App) Pid() int { return a.pid }

// Home is the isolated HOME the app runs with. Journeys seed their fixtures under it.
func (a *App) Home() string { return a.home }

// Log returns the app's log so far (stdout and stderr, file-backed).
func (a *App) Log() string {
	data, _ := os.ReadFile(a.logPath)
	return string(data)
}

func (a *App) isExited() bool {
	if a.exited {
		return true
	}
	select {
	case <-a.done:
		a.exited = true
	default:
	}
	return a.exited
}

// ArtifactsDir is the per-test directory for screenshots and logs. NVM_RIG_ARTIFACTS selects the
// root; CI uploads it.
func ArtifactsDir(t *testing.T) string {
	t.Helper()
	root := os.Getenv("NVM_RIG_ARTIFACTS")
	if root == "" {
		root = filepath.Join(os.TempDir(), "nvm-rig-artifacts")
	}
	dir := filepath.Join(root, sanitizeName(t.Name()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// hasLine reports whether a log line is exactly message, after the timestamp log.Print adds.
func hasLine(log, message string) bool {
	for _, got := range strings.Split(log, "\n") {
		if strings.TrimSpace(got) == message || strings.HasSuffix(got, " "+message) {
			return true
		}
	}
	return false
}

func tail(s string, lines int) string {
	parts := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}
