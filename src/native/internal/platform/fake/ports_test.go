package fake

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestShellRecordsCallsInOrderAndSkipsFailedOnes(t *testing.T) {
	shell := NewShell()
	boom := errors.New("no trash here")
	shell.FailNext(OpShellTrash, boom)

	if err := shell.Trash("/x"); !errors.Is(err, boom) {
		t.Fatalf("Trash = %v, want injected error", err)
	}
	if err := shell.OpenURL("https://example.test"); err != nil {
		t.Fatalf("OpenURL: %v", err)
	}
	shell.Beep()
	want := []ShellCall{{Op: OpShellOpenURL, Arg: "https://example.test"}, {Op: "Beep"}}
	if got := shell.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Calls = %+v, want %+v", got, want)
	}
}

func TestShellServesAppsAndIconsFromFixtures(t *testing.T) {
	shell := NewShell()
	apps := []platform.App{{Name: "Notes", ID: "notes", Path: "/Applications/Notes.app"}}
	shell.SetApps(apps, map[string][]byte{"/Applications/Notes.app": []byte("icon")})

	got, err := shell.InstalledApps()
	if err != nil || !reflect.DeepEqual(got, apps) {
		t.Fatalf("InstalledApps = %+v, %v", got, err)
	}
	icon, err := shell.AppIcon("/Applications/Notes.app", 64)
	if err != nil || string(icon) != "icon" {
		t.Fatalf("AppIcon = %q, %v", icon, err)
	}
	if _, err := shell.AppIcon("/missing.app", 64); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("AppIcon of unknown path: %v, want ErrNotFound", err)
	}
}

func TestScreensPickTheDisplayContainingThePoint(t *testing.T) {
	screens := NewScreens()
	left := platform.Display{ID: 1, Bounds: platform.Rect{Width: 1000, Height: 800}, Primary: true}
	right := platform.Display{ID: 2, Bounds: platform.Rect{X: 1000, Width: 1000, Height: 800}}
	screens.SetDisplays(left, right)

	if got := screens.NearestDisplay(platform.Point{X: 1500, Y: 10}); got.ID != 2 {
		t.Fatalf("NearestDisplay on the right = %d, want 2", got.ID)
	}
	if got := screens.NearestDisplay(platform.Point{X: -50, Y: -50}); got.ID != 1 {
		t.Fatalf("NearestDisplay off every display = %d, want the primary 1", got.ID)
	}
	screens.SetDisplays()
	if got := screens.NearestDisplay(platform.Point{}); got != (platform.Display{}) {
		t.Fatalf("NearestDisplay with no displays = %+v, want zero value", got)
	}
}

func TestNotifierRecordsAndCanRefuse(t *testing.T) {
	notes := NewNotifier()
	if err := notes.Show("Timer", "Tea is ready"); err != nil {
		t.Fatalf("Show: %v", err)
	}
	notes.FailNext(OpNotifierShow, platform.ErrDenied)
	if err := notes.Show("Timer", "again"); !errors.Is(err, platform.ErrDenied) {
		t.Fatalf("Show refused = %v", err)
	}
	if got := notes.Shown(); len(got) != 1 || got[0].Body != "Tea is ready" {
		t.Fatalf("Shown = %+v", got)
	}
}

func TestKeychainStoresCopiesAndReportsMissingSecrets(t *testing.T) {
	kc := NewKeychain()
	if _, err := kc.Get("nvm", "token"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
	value := []byte("secret")
	if err := kc.Set("nvm", "token", value); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value[0] = 'X'
	got, err := kc.Get("nvm", "token")
	if err != nil || string(got) != "secret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if _, err := kc.Get("other", "token"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("secrets are keyed by service: %v", err)
	}
	if err := kc.Delete("nvm", "token"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := kc.Delete("nvm", "token"); err != nil {
		t.Fatalf("Delete of a missing secret must succeed: %v", err)
	}
}

func TestProcessesSignalRemovesTerminatedProcessesOnly(t *testing.T) {
	procs := NewProcesses(
		platform.Process{PID: 7, Name: "editor", ResidentKB: 900},
		platform.Process{PID: 3, Name: "shell", ResidentKB: 10},
	)
	if err := procs.Signal(3, platform.SignalInterrupt); err != nil {
		t.Fatalf("interrupt: %v", err)
	}
	if err := procs.Signal(7, platform.SignalTerminate); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	list, _ := procs.List()
	if len(list) != 1 || list[0].PID != 3 {
		t.Fatalf("List after signals = %+v, want only PID 3", list)
	}
	if err := procs.Signal(7, platform.SignalKill); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("signal to exited process = %v, want ErrNotFound", err)
	}
	if got := procs.Signals(); len(got) != 3 {
		t.Fatalf("Signals recorded %d calls, want 3 including the failed one", len(got))
	}
}

func TestHelperReturnsScriptedOutputAndCapsIt(t *testing.T) {
	helper := NewHelper()
	huge := make([]byte, platform.MaxHelperOutputBytes+10)
	helper.Script("/bin/tool", HelperScript{Stdout: huge, ExitCode: 2}, HelperScript{Timeout: true})

	result, err := helper.Run(context.Background(), "/bin/tool", []string{"-x"}, []byte("in"))
	if err != nil || result.ExitCode != 2 || !result.Truncated || len(result.Stdout) != platform.MaxHelperOutputBytes {
		t.Fatalf("Run = %d bytes, exit %d, truncated %v, err %v", len(result.Stdout), result.ExitCode, result.Truncated, err)
	}
	if _, err := helper.Run(context.Background(), "/bin/tool", nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout script returned %v", err)
	}
	if _, err := helper.Run(context.Background(), "/bin/tool", nil, nil); !errors.Is(err, ErrNoHelperScript) {
		t.Fatalf("exhausted script returned %v, want ErrNoHelperScript", err)
	}
	calls := helper.Calls()
	if len(calls) != 3 || calls[0].Binary != "/bin/tool" || string(calls[0].Stdin) != "in" {
		t.Fatalf("Calls = %+v", calls)
	}
}

func TestHotkeysRefuseDuplicatesAndFireBoundHandlers(t *testing.T) {
	keys := NewHotkeys()
	fired := 0
	if err := keys.Register("Command+Alt+K", func() { fired++ }); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := keys.Register("Command+Alt+K", func() {}); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("second Register = %v, want ErrConflict", err)
	}
	if err := keys.Fire("Command+Alt+K"); err != nil || fired != 1 {
		t.Fatalf("Fire: err %v, fired %d", err, fired)
	}
	keys.Unregister("Command+Alt+K")
	if err := keys.Fire("Command+Alt+K"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Fire after Unregister = %v, want ErrNotFound", err)
	}
}

func TestTrayRecordsMenuInvokesActionsAndStopsAfterClose(t *testing.T) {
	host := NewTrayHost()
	item, err := host.NewTray(platform.TrayOptions{ID: "clock", Title: "12:00"})
	if err != nil {
		t.Fatalf("NewTray: %v", err)
	}
	ran := false
	item.SetTitle("12:01")
	item.SetMenu([]platform.MenuItem{
		{ID: "copy", Label: "Copy", Action: func() { ran = true }},
		{Separator: true},
	})
	tray := host.Trays()[0]
	if err := tray.Invoke("copy"); err != nil || !ran {
		t.Fatalf("Invoke copy: %v, ran %v", err, ran)
	}
	if err := tray.Invoke(""); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Invoke of a separator = %v, want ErrNotFound", err)
	}
	if tray.Title() != "12:01" || len(tray.Items()) != 2 {
		t.Fatalf("title %q items %+v", tray.Title(), tray.Items())
	}
	item.Close()
	item.SetTitle("ignored")
	if !tray.Closed() || tray.Title() != "12:01" || tray.Invoke("copy") == nil {
		t.Fatal("closed tray still accepts changes or invokes")
	}
}

func TestAutostartStartsDisabledAndStoresTheFlag(t *testing.T) {
	auto := NewAutostart()
	if enabled, _ := auto.Enabled(); enabled {
		t.Fatal("a fresh autostart is enabled")
	}
	if err := auto.Set(true); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if enabled, _ := auto.Enabled(); !enabled {
		t.Fatal("Enabled after Set(true) is false")
	}
}

func TestSetPortsSharesStateWithTypedFakes(t *testing.T) {
	set := NewSet()
	ports := set.Ports()
	if err := ports.Clipboard.WriteText("copied"); err != nil {
		t.Fatalf("WriteText through the port: %v", err)
	}
	if got, _ := set.Clipboard.ReadText(); got != "copied" {
		t.Fatalf("typed fake sees %q, want the port's write", got)
	}
	if !ports.Capabilities.Supported(platform.CapabilityProcessSignal) {
		t.Fatal("NewSet must support every capability by default")
	}
}
