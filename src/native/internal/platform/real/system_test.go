package real

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// installedSet returns a predicate over a fixed list of program names.
func installedSet(names ...string) func(string) bool {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return func(name string) bool { return set[name] }
}

// sortedCapabilities copies and sorts a capability list, so comparisons ignore order.
func sortedCapabilities(list []platform.Capability) []platform.Capability {
	out := slices.Clone(list)
	slices.Sort(out)
	return out
}

// TestSystemCapabilitiesMatchTheDocumentedInventoryPerOS is the inventory table: each OS, with the programs it
// has, supports exactly the listed system capabilities. Windows has no empty-Trash command; Linux has no Trash
// without a home folder and no log out without a user.
func TestSystemCapabilitiesMatchTheDocumentedInventoryPerOS(t *testing.T) {
	all := []platform.Capability{
		platform.CapabilitySystemEmptyTrash, platform.CapabilitySystemLock, platform.CapabilitySystemLogOut,
		platform.CapabilitySystemRestart, platform.CapabilitySystemSettings, platform.CapabilitySystemShutDown,
		platform.CapabilitySystemSleep,
	}
	windowsSet := []platform.Capability{
		platform.CapabilitySystemLock, platform.CapabilitySystemLogOut, platform.CapabilitySystemRestart,
		platform.CapabilitySystemSettings, platform.CapabilitySystemShutDown, platform.CapabilitySystemSleep,
	}
	cases := []struct {
		name string
		env  systemEnvironment
		want []platform.Capability
	}{
		{
			name: "macOS with its programs",
			env: systemEnvironment{goos: "darwin", user: "alice", home: "/Users/alice",
				installed: installedSet("osascript", "pmset", macLockCommand)},
			want: all,
		},
		{
			name: "Windows with its programs",
			env: systemEnvironment{goos: "windows", user: "alice", home: `C:\Users\alice`,
				installed: installedSet("rundll32.exe", "shutdown")},
			want: windowsSet,
		},
		{
			name: "Linux desktop with systemd, a locker and settings",
			env: systemEnvironment{goos: "linux", user: "alice", home: "/home/alice",
				installed: installedSet("loginctl", "systemctl", "gnome-control-center")},
			want: all,
		},
		{
			name: "Linux without the programs or a home folder",
			env:  systemEnvironment{goos: "linux", installed: installedSet()},
			want: nil,
		},
		{
			name: "Linux with systemd but no settings program",
			env: systemEnvironment{goos: "linux", user: "alice", home: "/home/alice",
				installed: installedSet("systemctl")},
			want: []platform.Capability{
				platform.CapabilitySystemEmptyTrash, platform.CapabilitySystemRestart,
				platform.CapabilitySystemShutDown, platform.CapabilitySystemSleep,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sortedCapabilities(systemCapabilities(tc.env))
			want := sortedCapabilities(tc.want)
			if len(got) == 0 && len(want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("capabilities = %v, want %v", got, want)
			}
		})
	}
}

// newTestSystem builds the system port over a fake helper and shell, with a launcher that records what it starts.
func newTestSystem(goos, user, home string, helper *fake.Helper, shell *fake.Shell) (*system, *[]string) {
	s := newSystem(helper, shell, goos, home, user)
	var started []string
	s.launch = func(name string, _ []string) error {
		started = append(started, name)
		return nil
	}
	return s, &started
}

// okScript is a helper run that exits 0.
var okScript = fake.HelperScript{ExitCode: 0}

// TestPowerCommandsRunTheExactProgramOfEachOS checks the argv each OS receives for each action.
func TestPowerCommandsRunTheExactProgramOfEachOS(t *testing.T) {
	cases := []struct {
		name string
		goos string
		user string
		run  func(*system) error
		want []fake.HelperCall
	}{
		{"macOS restart", "darwin", "", (*system).Restart, []fake.HelperCall{
			{Binary: "osascript", Args: []string{"-e", `tell application "System Events" to restart`}}}},
		{"macOS shut down", "darwin", "", (*system).ShutDown, []fake.HelperCall{
			{Binary: "osascript", Args: []string{"-e", `tell application "System Events" to shut down`}}}},
		{"macOS log out", "darwin", "", (*system).LogOut, []fake.HelperCall{
			{Binary: "osascript", Args: []string{"-e", `tell application "System Events" to log out`}}}},
		{"macOS empty trash", "darwin", "", (*system).EmptyTrash, []fake.HelperCall{
			{Binary: "osascript", Args: []string{"-e", `tell application "Finder" to empty trash`}}}},
		{"macOS sleep", "darwin", "", (*system).Sleep, []fake.HelperCall{
			{Binary: "pmset", Args: []string{"sleepnow"}}}},
		{"macOS lock", "darwin", "", (*system).LockScreen, []fake.HelperCall{
			{Binary: macLockCommand, Args: []string{"-suspend"}}}},
		{"Windows lock", "windows", "", (*system).LockScreen, []fake.HelperCall{
			{Binary: "rundll32.exe", Args: []string{"user32.dll,LockWorkStation"}}}},
		{"Windows sleep", "windows", "", (*system).Sleep, []fake.HelperCall{
			{Binary: "rundll32.exe", Args: []string{"powrprof.dll,SetSuspendState", "0,1,0"}}}},
		{"Windows restart", "windows", "", (*system).Restart, []fake.HelperCall{
			{Binary: "shutdown", Args: []string{"/r", "/t", "0"}}}},
		{"Windows shut down", "windows", "", (*system).ShutDown, []fake.HelperCall{
			{Binary: "shutdown", Args: []string{"/s", "/t", "0"}}}},
		{"Windows log out", "windows", "", (*system).LogOut, []fake.HelperCall{
			{Binary: "shutdown", Args: []string{"/l"}}}},
		{"Linux sleep", "linux", "alice", (*system).Sleep, []fake.HelperCall{
			{Binary: "systemctl", Args: []string{"suspend"}}}},
		{"Linux restart", "linux", "alice", (*system).Restart, []fake.HelperCall{
			{Binary: "systemctl", Args: []string{"reboot"}}}},
		{"Linux shut down", "linux", "alice", (*system).ShutDown, []fake.HelperCall{
			{Binary: "systemctl", Args: []string{"poweroff"}}}},
		{"Linux log out names the user", "linux", "alice", (*system).LogOut, []fake.HelperCall{
			{Binary: "loginctl", Args: []string{"terminate-user", "alice"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			helper := fake.NewHelper()
			for _, call := range tc.want {
				helper.Script(call.Binary, okScript)
			}
			s, _ := newTestSystem(tc.goos, tc.user, "/home/alice", helper, fake.NewShell())
			if err := tc.run(s); err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := helper.Calls(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("calls = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestLockFallsBackToTheNextLockerThatExitsCleanly tries loginctl first, then xdg-screensaver.
func TestLockFallsBackToTheNextLockerThatExitsCleanly(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script("loginctl", fake.HelperScript{ExitCode: 1})
	helper.Script("xdg-screensaver", okScript)
	s, _ := newTestSystem("linux", "alice", "/home/alice", helper, fake.NewShell())
	if err := s.LockScreen(); err != nil {
		t.Fatalf("LockScreen = %v, want the second locker to succeed", err)
	}
	want := []fake.HelperCall{{Binary: "loginctl", Args: []string{"lock-session"}}, {Binary: "xdg-screensaver", Args: []string{"lock"}}}
	if got := helper.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %+v, want %+v", got, want)
	}
}

// TestPowerCommandWithNoInstalledProgramIsUnsupported reports the intent as unsupported, not as a failed run.
func TestPowerCommandWithNoInstalledProgramIsUnsupported(t *testing.T) {
	helper := fake.NewHelper()
	missing := fmt.Errorf("exec: %w", exec.ErrNotFound)
	helper.Script("systemctl", fake.HelperScript{Err: missing})
	s, _ := newTestSystem("linux", "alice", "/home/alice", helper, fake.NewShell())
	if err := s.Sleep(); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Sleep = %v, want ErrUnsupported", err)
	}
}

// TestPowerCommandThatFailsKeepsItsReason reports a non-zero exit as a failure with the status.
func TestPowerCommandThatFailsKeepsItsReason(t *testing.T) {
	helper := fake.NewHelper()
	helper.Script("systemctl", fake.HelperScript{ExitCode: 3})
	s, _ := newTestSystem("linux", "alice", "/home/alice", helper, fake.NewShell())
	err := s.Restart()
	if err == nil || errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Restart = %v, want a failure that is not unsupported", err)
	}
}

// TestLogOutWithoutAUserIsUnsupportedAndRunsNothing never runs a log-out without knowing whose session it ends.
func TestLogOutWithoutAUserIsUnsupportedAndRunsNothing(t *testing.T) {
	helper := fake.NewHelper()
	s, _ := newTestSystem("linux", "", "/home/alice", helper, fake.NewShell())
	if err := s.LogOut(); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("LogOut = %v, want ErrUnsupported", err)
	}
	if calls := helper.Calls(); len(calls) != 0 {
		t.Fatalf("LogOut ran %+v without a user", calls)
	}
}

// TestOpenSettingsOnMacOSOpensThePaneURL opens the settings URL through the shell, with the pane and anchor.
func TestOpenSettingsOnMacOSOpensThePaneURL(t *testing.T) {
	shell := fake.NewShell()
	s, _ := newTestSystem("darwin", "alice", "/Users/alice", fake.NewHelper(), shell)
	if err := s.OpenSettings("com.apple.settings.PrivacySecurity.extension", "Microphone"); err != nil {
		t.Fatalf("OpenSettings: %v", err)
	}
	want := "x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Microphone"
	if got := shell.Calls(); len(got) != 1 || got[0].Arg != want {
		t.Fatalf("shell calls = %+v, want one OpenURL of %s", got, want)
	}
}

// TestMacSettingsURLRejectsUnsafeNames opens the top level for a pane name that is not an identifier, and drops an
// anchor that is not a plain word.
func TestMacSettingsURLRejectsUnsafeNames(t *testing.T) {
	cases := []struct{ pane, anchor, want string }{
		{"", "", "x-apple.systempreferences:"},
		{"com.apple.Sound-Settings.extension", "", "x-apple.systempreferences:com.apple.Sound-Settings.extension"},
		{"com.apple.Sound;rm -rf", "", "x-apple.systempreferences:"},
		{"com.apple.Sound-Settings.extension", "Mic rophone", "x-apple.systempreferences:com.apple.Sound-Settings.extension"},
	}
	for _, tc := range cases {
		if got := macSettingsURL(tc.pane, tc.anchor); got != tc.want {
			t.Errorf("macSettingsURL(%q, %q) = %q, want %q", tc.pane, tc.anchor, got, tc.want)
		}
	}
}

// TestOpenSettingsOnWindowsOpensTheSettingsApp opens the settings app through the shell.
func TestOpenSettingsOnWindowsOpensTheSettingsApp(t *testing.T) {
	shell := fake.NewShell()
	s, _ := newTestSystem("windows", "alice", `C:\Users\alice`, fake.NewHelper(), shell)
	if err := s.OpenSettings("", ""); err != nil {
		t.Fatalf("OpenSettings: %v", err)
	}
	if got := shell.Calls(); len(got) != 1 || got[0].Arg != "ms-settings:" {
		t.Fatalf("shell calls = %+v, want one OpenURL of ms-settings:", got)
	}
}

// TestOpenSettingsOnLinuxStartsTheFirstInstalledDesktopSettings starts the first settings program that exists and
// does not start the others.
func TestOpenSettingsOnLinuxStartsTheFirstInstalledDesktopSettings(t *testing.T) {
	s, _ := newTestSystem("linux", "alice", "/home/alice", fake.NewHelper(), fake.NewShell())
	var started []string
	s.launch = func(name string, _ []string) error {
		started = append(started, name)
		if name == "gnome-control-center" {
			return fmt.Errorf("start: %w", exec.ErrNotFound)
		}
		return nil
	}
	if err := s.OpenSettings("", ""); err != nil {
		t.Fatalf("OpenSettings: %v", err)
	}
	if want := []string{"gnome-control-center", "systemsettings"}; !reflect.DeepEqual(started, want) {
		t.Fatalf("started = %v, want %v", started, want)
	}
}

// TestEmptyTrashOnLinuxRemovesTrashContentsOnly empties the freedesktop Trash and leaves the rest of the home folder.
func TestEmptyTrashOnLinuxRemovesTrashContentsOnly(t *testing.T) {
	home := t.TempDir()
	files := filepath.Join(home, ".local", "share", "Trash", "files")
	info := filepath.Join(home, ".local", "share", "Trash", "info")
	documents := filepath.Join(home, "Documents")
	for _, dir := range []string{files, info, documents} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(files, "old.txt"), "gone")
	mustWrite(t, filepath.Join(info, "old.txt.trashinfo"), "gone")
	mustWrite(t, filepath.Join(documents, "keep.txt"), "kept")

	s, _ := newTestSystem("linux", "alice", home, fake.NewHelper(), fake.NewShell())
	if err := s.EmptyTrash(); err != nil {
		t.Fatalf("EmptyTrash: %v", err)
	}
	if entries, _ := os.ReadDir(files); len(entries) != 0 {
		t.Fatalf("Trash files still has %d entries", len(entries))
	}
	if entries, _ := os.ReadDir(info); len(entries) != 0 {
		t.Fatalf("Trash info still has %d entries", len(entries))
	}
	if _, err := os.Stat(filepath.Join(documents, "keep.txt")); err != nil {
		t.Fatalf("a file outside the Trash was removed: %v", err)
	}
}

// TestEmptyTrashWithoutAHomeFolderRemovesNothing refuses to act on a relative Trash folder.
func TestEmptyTrashWithoutAHomeFolderRemovesNothing(t *testing.T) {
	s, _ := newTestSystem("linux", "alice", "", fake.NewHelper(), fake.NewShell())
	if err := s.EmptyTrash(); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("EmptyTrash = %v, want ErrUnsupported", err)
	}
}

// TestEmptyTrashOnWindowsIsUnsupported keeps Windows honest: it has no empty-Trash command yet.
func TestEmptyTrashOnWindowsIsUnsupported(t *testing.T) {
	helper := fake.NewHelper()
	s, _ := newTestSystem("windows", "alice", `C:\Users\alice`, helper, fake.NewShell())
	if err := s.EmptyTrash(); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("EmptyTrash = %v, want ErrUnsupported", err)
	}
	if calls := helper.Calls(); len(calls) != 0 {
		t.Fatalf("EmptyTrash ran %+v on Windows", calls)
	}
}

// TestPermissionsReportUnknownUntilTheOSCanBeAsked never reports a permission as allowed.
func TestPermissionsReportUnknownUntilTheOSCanBeAsked(t *testing.T) {
	var ports permissions
	state, err := ports.State(platform.PermissionMicrophone)
	if state != platform.PermissionUnknown || !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("State = %q, %v, want unknown and ErrUnsupported", state, err)
	}
	if err := ports.Request(platform.PermissionMicrophone); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Request = %v, want ErrUnsupported", err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
