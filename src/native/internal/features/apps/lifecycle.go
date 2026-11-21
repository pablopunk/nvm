package apps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ScriptFailure is an AppleScript or a macOS program that ran and exited with an error: its exit code and what it
// printed. The host shows its reason in the HUD and the log, so it is kept apart from an ordinary failure.
type ScriptFailure struct {
	ExitCode int
	Stderr   string
}

func (f ScriptFailure) Error() string {
	return fmt.Sprintf("script exited with %d", f.ExitCode)
}

// lifecycleScriptTimeout bounds one AppleScript or killall run. The quit request waits its grace period on top.
const (
	lifecycleScriptTimeout = 10 * time.Second
	osascriptBinary        = "/usr/bin/osascript"
	killallBinary          = "/usr/bin/killall"
	quitPollInterval       = 250 * time.Millisecond
)

// macQuitLines ask the app at the path argument to quit, as the 1.x host did. The app may refuse, for example with an
// unsaved-changes sheet; the request does not force it.
var macQuitLines = []string{
	"on run argv",
	"set targetApp to application ((POSIX file (item 1 of argv)) as text)",
	"if targetApp is running then tell targetApp to quit",
	"end run",
}

// macQuitAllLines quit every visible app but Nevermind and Finder, and return the names it asked, one per line.
var macQuitAllLines = []string{
	"set quitList to {}",
	`tell application "System Events"`,
	"set appProcesses to every process whose background only is false",
	"repeat with p in appProcesses",
	"set nm to name of p",
	`if nm is not "Nevermind" and nm is not "Finder" then copy nm to end of quitList`,
	"end repeat",
	"end tell",
	"repeat with appName in quitList",
	"try",
	"tell application appName to quit",
	"end try",
	"end repeat",
	"set AppleScript's text item delimiters to linefeed",
	"return quitList as text",
}

// macHideLines hide the process named by the argument, which keeps its windows and its app running.
var macHideLines = []string{
	"on run argv",
	`tell application "System Events" to set visible of process (item 1 of argv) to false`,
	"end run",
}

// RunningApps returns the apps of list that have a live process, from the running state the palette keeps. A stale
// state starts a refresh, and the palette runs its rows again when the running set changes, so the answer never waits.
func (s *Services) RunningApps(list []App) ([]App, error) {
	s.Running.ScheduleRefresh("extension")
	marked := s.Running.RunningAmong(list)
	out := []App{}
	for _, app := range list {
		if marked[app.Path] {
			out = append(out, app)
		}
	}
	return out, nil
}

// QuitApp asks app to quit and waits the grace period for it to go. On macOS the app itself is asked, as 1.x did; a
// save prompt stays with the app. Elsewhere the app's processes are signalled.
func (s *Services) QuitApp(app App) (QuitOutcome, error) {
	if s.platform == "darwin" {
		return s.quitMacApp(app)
	}
	return s.Quitter.Quit(s.ProcessNames(app))
}

// ForceQuitApp ends app at once, without asking it.
func (s *Services) ForceQuitApp(app App) (QuitOutcome, error) {
	if s.platform == "darwin" {
		return s.forceQuitMacApp(app)
	}
	return s.Quitter.ForceQuit(s.ProcessNames(app))
}

// QuitAllApps asks every running app but Nevermind and the desktop's own apps to quit. It returns the apps it asked.
func (s *Services) QuitAllApps() ([]App, error) {
	if s.platform == "darwin" {
		out, err := s.runMacScript(macQuitAllLines)
		if err != nil {
			return nil, err
		}
		return namedApps(out), nil
	}
	running, err := s.RunningApps(s.Index.Apps())
	if err != nil {
		return nil, err
	}
	var names []string
	quit := []App{}
	for _, app := range running {
		names = append(names, quitAllNames(s.ProcessNames(app))...)
		quit = append(quit, app)
	}
	if len(names) == 0 {
		return quit, nil
	}
	_, err = s.Quitter.Quit(names)
	return quit, err
}

// HideApp hides the windows of app. Only macOS can hide an app; elsewhere the answer is platform.ErrUnsupported.
func (s *Services) HideApp(app App) error {
	if s.platform != "darwin" {
		return platform.ErrUnsupported
	}
	_, err := s.runMacScript(macHideLines, app.Name)
	return err
}

// quitAllNames keeps the process names that Quit All may ask to close.
func quitAllNames(names []string) []string {
	var kept []string
	for _, name := range names {
		if !IsExcludedFromQuitAll(name) {
			kept = append(kept, name)
		}
	}
	return kept
}

func (s *Services) quitMacApp(app App) (QuitOutcome, error) {
	if !strings.HasSuffix(app.Path, ".app") {
		return QuitOutcome{}, platform.ErrUnsupported
	}
	running, err := runningMacApp(s.ports.Helper, app.Path)
	if err != nil || !running {
		return QuitOutcome{}, err
	}
	if _, err := s.runMacScript(macQuitLines, app.Path); err != nil {
		return QuitOutcome{Found: 1, Failed: 1}, err
	}
	if s.waitMacAppGone(app) {
		return QuitOutcome{Found: 1, Terminated: 1}, nil
	}
	return QuitOutcome{Found: 1}, nil
}

// waitMacAppGone polls the process list for the grace period and reports whether the app went.
func (s *Services) waitMacAppGone(app App) bool {
	var waited time.Duration
	for waited < DefaultQuitGrace {
		<-s.ports.Clock.After(quitPollInterval)
		waited += quitPollInterval
		if running, err := runningMacApp(s.ports.Helper, app.Path); err == nil && !running {
			return true
		}
	}
	return false
}

func (s *Services) forceQuitMacApp(app App) (QuitOutcome, error) {
	running, err := runningMacApp(s.ports.Helper, app.Path)
	if err != nil || !running {
		return QuitOutcome{}, err
	}
	for _, name := range s.ProcessNames(app) {
		ctx, cancel := context.WithTimeout(context.Background(), lifecycleScriptTimeout)
		result, err := s.ports.Helper.Run(ctx, killallBinary, []string{"-KILL", name}, nil)
		cancel()
		if err == nil && result.ExitCode == 0 {
			return QuitOutcome{Found: 1, Killed: 1}, nil
		}
	}
	return QuitOutcome{Found: 1, Failed: 1}, fmt.Errorf("could not force quit %s", app.Name)
}

// runMacScript runs an AppleScript with the arguments as its argv, and returns what it printed. A script that exits
// with an error is a ScriptFailure carrying its stderr.
func (s *Services) runMacScript(lines []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), lifecycleScriptTimeout)
	defer cancel()
	result, err := s.ports.Helper.Run(ctx, osascriptBinary, scriptArguments(lines, args), nil)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", ScriptFailure{ExitCode: result.ExitCode, Stderr: string(result.Stderr)}
	}
	return string(result.Stdout), nil
}

// scriptArguments is the osascript command line for the lines, each given with -e, then the argv.
func scriptArguments(lines []string, args []string) []string {
	out := make([]string, 0, 2*len(lines)+len(args))
	for _, line := range lines {
		out = append(out, "-e", line)
	}
	return append(out, args...)
}

// namedApps reads the names AppleScript printed, one per line, as apps with only a name.
func namedApps(output string) []App {
	out := []App{}
	for _, line := range strings.Split(output, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			out = append(out, App{Name: name})
		}
	}
	return out
}

// IndexedApps is the index's list of apps, sorted by name, with the time each was first seen.
func (s *Services) IndexedApps() []App {
	return s.Index.Apps()
}

// DiscoverUninstall finds what uninstalling the app at appPath would move to the Trash.
func (s *Services) DiscoverUninstall(appPath string) Discovery {
	return s.Uninstaller.Discover(appPath)
}

// TrashUninstall moves the reviewed items the values select to the Trash, checking each one again first.
func (s *Services) TrashUninstall(snapshot UninstallSnapshot, values map[string]bool) TrashResult {
	return s.Uninstaller.Trash(snapshot, values)
}
