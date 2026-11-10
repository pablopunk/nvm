package apps

import (
	"errors"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const notesBundle = "/Applications/Notes.app"

// macServices is the apps services on macOS over a scripted helper, with the ports the lifecycle reads.
func macServices(helper platform.Helper) *Services {
	return &Services{ports: platform.Ports{Helper: helper}, platform: "darwin"}
}

// TestRunningMacAppsMatchesBundleCommandsIgnoringCase: one process listing answers for every candidate, and a command
// counts when it runs from the bundle's Contents folder, whatever the case of the path.
func TestRunningMacAppsMatchesBundleCommandsIgnoringCase(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(psBinary, fake.HelperScript{Stdout: []byte("/Applications/Notes.app/Contents/MacOS/Notes\n/bin/zsh\n")})
	running, err := runningMacApps(helper, []App{{Name: "Notes", Path: notesBundle}, {Name: "Mail", Path: "/Applications/Mail.app"}})
	if err != nil {
		t.Fatalf("runningMacApps = %v", err)
	}
	if !running[notesBundle] || running["/Applications/Mail.app"] {
		t.Fatalf("running = %v; want Notes only", running)
	}
	if calls := helper.Calls(); len(calls) != 1 {
		t.Fatalf("process listings = %d; want one for every candidate", len(calls))
	}
}

// TestRunningMacAppsFailsWhenTheListingFails: an unreadable process list is an error, never a guess that nothing runs.
func TestRunningMacAppsFailsWhenTheListingFails(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(psBinary, fake.HelperScript{Err: errors.New("no ps")})
	if _, err := runningMacApps(helper, []App{{Path: notesBundle}}); err == nil {
		t.Fatal("a failed listing was read as nothing running")
	}
}

// TestHideRunsTheScriptWithTheProcessNameAsArgument: the name reaches AppleScript as argv, never as script source.
func TestHideRunsTheScriptWithTheProcessNameAsArgument(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(osascriptBinary, fake.HelperScript{})
	if err := macServices(helper).HideApp(App{Name: "Notes", Path: notesBundle}); err != nil {
		t.Fatalf("HideApp = %v", err)
	}
	calls := helper.Calls()
	if len(calls) != 1 || calls[0].Binary != osascriptBinary {
		t.Fatalf("calls = %+v; want one osascript run", calls)
	}
	if last := calls[0].Args[len(calls[0].Args)-1]; last != "Notes" {
		t.Fatalf("last argument = %q; want the process name", last)
	}
}

// TestHideReportsAScriptThatExitedWithAnError: a refused script comes back as a ScriptFailure with its stderr, so the
// host can show the reason and log it.
func TestHideReportsAScriptThatExitedWithAnError(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(osascriptBinary, fake.HelperScript{ExitCode: 1, Stderr: []byte("not allowed (-1743)")})
	err := macServices(helper).HideApp(App{Name: "Notes", Path: notesBundle})
	var failure ScriptFailure
	if !errors.As(err, &failure) || failure.ExitCode != 1 || failure.Stderr != "not allowed (-1743)" {
		t.Fatalf("HideApp = %v; want a ScriptFailure with the exit code and stderr", err)
	}
}

// TestHideIsUnsupportedOffMacOS: only macOS can hide an app, so elsewhere the call says so rather than doing nothing.
func TestHideIsUnsupportedOffMacOS(t *testing.T) {
	linux := &Services{ports: platform.Ports{Helper: fake.NewHelper()}, platform: "linux"}
	if err := linux.HideApp(App{Name: "Notes", Path: "/usr/share/applications/notes.desktop"}); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("HideApp on Linux = %v; want ErrUnsupported", err)
	}
}

// TestQuitAllOnMacReadsTheNamesAppleScriptAsked: Quit All returns the names its script printed, one per line.
func TestQuitAllOnMacReadsTheNamesAppleScriptAsked(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(osascriptBinary, fake.HelperScript{Stdout: []byte("Notes\nSafari\n\n")})
	asked, err := macServices(helper).QuitAllApps()
	if err != nil {
		t.Fatalf("QuitAllApps = %v", err)
	}
	if len(asked) != 2 || asked[0].Name != "Notes" || asked[1].Name != "Safari" {
		t.Fatalf("asked = %+v; want Notes and Safari", asked)
	}
}

// TestForceQuitOnMacKillsTheAppWithKillall: a running app is killed by name, and the outcome counts the kill.
func TestForceQuitOnMacKillsTheAppWithKillall(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script(psBinary, fake.HelperScript{Stdout: []byte(notesBundle + "/Contents/MacOS/Notes\n")})
	helper.Script(killallBinary, fake.HelperScript{})
	outcome, err := macServices(helper).ForceQuitApp(App{Name: "Notes", Path: notesBundle})
	if err != nil {
		t.Fatalf("ForceQuitApp = %v", err)
	}
	if outcome != (QuitOutcome{Found: 1, Killed: 1}) {
		t.Fatalf("outcome = %+v; want one app killed", outcome)
	}
	last := helper.Calls()[len(helper.Calls())-1]
	if !slices.Equal(last.Args, []string{"-KILL", "Notes"}) {
		t.Fatalf("killall arguments = %v; want -KILL Notes", last.Args)
	}
}

// TestScriptArgumentsPairEachLineWithDashE: the lines come first, each after -e, and the argv follows them.
func TestScriptArgumentsPairEachLineWithDashE(t *testing.T) {
	got := scriptArguments([]string{"on run argv", "end run"}, []string{"/Applications/Notes.app"})
	want := []string{"-e", "on run argv", "-e", "end run", "/Applications/Notes.app"}
	if !slices.Equal(got, want) {
		t.Fatalf("arguments = %v; want %v", got, want)
	}
}
