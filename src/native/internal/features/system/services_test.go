package system

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// servicesOver builds the services over a fake set with every capability.
func servicesOver(goos string, set *fake.Set) *Services {
	return NewServices(Options{Ports: set.Ports(), Platform: goos, Home: "/home/alice", SettingsTitle: "Open Settings"})
}

// TestRunCallsTheMatchingPortOperation runs each command and checks the single port call it makes.
func TestRunCallsTheMatchingPortOperation(t *testing.T) {
	cases := []struct {
		id   CommandID
		want string
	}{
		{CommandLockScreen, fake.OpSystemLock},
		{CommandSleep, fake.OpSystemSleep},
		{CommandRestart, fake.OpSystemRestart},
		{CommandShutDown, fake.OpSystemShutDown},
		{CommandLogOut, fake.OpSystemLogOut},
		{CommandEmptyTrash, fake.OpSystemEmptyTrash},
		{CommandSystemPanes, fake.OpSystemOpenSettings},
	}
	for _, tc := range cases {
		set := fake.NewSet()
		if err := servicesOver("linux", set).Run(tc.id); err != nil {
			t.Fatalf("Run(%s): %v", tc.id, err)
		}
		calls := set.System.Calls()
		if len(calls) != 1 || calls[0].Op != tc.want {
			t.Fatalf("Run(%s) calls = %+v, want one %s", tc.id, calls, tc.want)
		}
	}
}

// TestRunUnknownCommandIsUnsupported runs nothing for an ID the catalog does not have.
func TestRunUnknownCommandIsUnsupported(t *testing.T) {
	set := fake.NewSet()
	if err := servicesOver("linux", set).Run("made-up"); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Run = %v, want ErrUnsupported", err)
	}
	if calls := set.System.Calls(); len(calls) != 0 {
		t.Fatalf("an unknown command reached the system: %+v", calls)
	}
}

// TestRunPropagatesThePortError reports the port's failure rather than hiding it.
func TestRunPropagatesThePortError(t *testing.T) {
	set := fake.NewSet()
	boom := errors.New("refused")
	set.System.FailNext(fake.OpSystemSleep, boom)
	if err := servicesOver("linux", set).Run(CommandSleep); !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the port error", err)
	}
}

// TestOpenPaneSendsThePaneAndAnchor opens a macOS pane at its section.
func TestOpenPaneSendsThePaneAndAnchor(t *testing.T) {
	set := fake.NewSet()
	pane := Pane{ID: "com.apple.settings.PrivacySecurity.extension", Title: "Privacy & Security"}
	if err := servicesOver("darwin", set).OpenPane(pane, "Microphone"); err != nil {
		t.Fatalf("OpenPane: %v", err)
	}
	want := []fake.SystemCall{{Op: fake.OpSystemOpenSettings, Arg: pane.ID + "/Microphone"}}
	if got := set.System.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %+v, want %+v", got, want)
	}
}

// TestSettingsPanesReadTheSidebarOnMacOSOnly lists the panes on macOS and never reads a sidebar elsewhere.
func TestSettingsPanesReadTheSidebarOnMacOSOnly(t *testing.T) {
	set := fake.NewSet()
	if err := set.FS.MkdirAll(filepath.Dir(macSidebarPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := set.FS.WriteFile(macSidebarPath, []byte(`<string>com.apple.Sound-Settings.extension</string>`), 0o600); err != nil {
		t.Fatal(err)
	}
	mac := servicesOver("darwin", set).SettingsPanes()
	if len(mac) != 1 || mac[0].Title != "Sound" {
		t.Fatalf("darwin panes = %+v, want Sound", mac)
	}
	if linux := servicesOver("linux", set).SettingsPanes(); len(linux) != 0 {
		t.Fatalf("linux panes = %+v, want none", linux)
	}
}

// TestOpenPlaceOpensTheFolderThroughTheShell hands the folder to the shell's default application.
func TestOpenPlaceOpensTheFolderThroughTheShell(t *testing.T) {
	set := fake.NewSet()
	service := servicesOver("linux", set)
	place := service.Places()[0]
	if err := service.OpenPlace(place); err != nil {
		t.Fatalf("OpenPlace: %v", err)
	}
	calls := set.Shell.Calls()
	if len(calls) != 1 || calls[0].Arg != place.Path {
		t.Fatalf("shell calls = %+v, want OpenPath of %s", calls, place.Path)
	}
}

// TestRequestPermissionReturnsUnsupportedWithoutAPrompt tells the caller to open settings instead.
func TestRequestPermissionReturnsUnsupportedWithoutAPrompt(t *testing.T) {
	set := fake.NewSet()
	set.Permissions = fake.NewPermissions()
	set.Permissions.SetState(platform.PermissionMicrophone, platform.PermissionNotDetermined)
	service := servicesOver("linux", set)
	if err := service.RequestPermission(platform.PermissionMicrophone); err != nil {
		t.Fatalf("RequestPermission with a prompt: %v", err)
	}
	rows := service.PermissionRows()
	if rows[0].State != platform.PermissionAllowed {
		t.Fatalf("microphone after the prompt = %q, want allowed", rows[0].State)
	}
}
