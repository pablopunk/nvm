package supervisor

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// listedWindows is the window engine the desktop fixture sees: one window and no placements.
type listedWindows struct{}

func (listedWindows) Displays(context.Context) ([]api.DisplayInfo, error) { return nil, nil }

func (listedWindows) Windows(context.Context, api.WindowQuery) ([]api.WindowInfo, error) {
	return []api.WindowInfo{{Handle: "w1", AppName: "Notes", Title: "Draft"}}, nil
}

func (listedWindows) Focus(context.Context, api.WindowHandle) (api.FocusResult, error) {
	return api.FocusResult{OK: true}, nil
}

func (listedWindows) ApplyTo(context.Context, api.WindowHandle, api.WindowLayout, api.WindowManagementOptions) (api.ApplyResult, error) {
	return api.ApplyResult{OK: true}, nil
}

func (listedWindows) Apply(context.Context, api.WindowLayout, api.WindowManagementOptions) (api.ApplyResult, error) {
	return api.ApplyResult{OK: true}, nil
}

func (listedWindows) FocusNeighbor(context.Context, string, bool) (api.FocusResult, error) {
	return api.FocusResult{OK: true}, nil
}

// fixedShortcuts is the shortcut table the desktop fixture reads.
type fixedShortcuts struct{}

func (fixedShortcuts) List() []api.ShortcutRecord {
	return []api.ShortcutRecord{{ActionID: "palette", Accelerator: "Command+Alt+K", Source: "user"}}
}

func (fixedShortcuts) Palette() api.ShortcutRecord {
	return api.ShortcutRecord{ActionID: "palette", Accelerator: "Command+Alt+K", Source: "user"}
}

func (fixedShortcuts) Aliases(string) map[string][]string { return nil }

// desktopRig is the services rig with every desktop port wired to a fake, so the fixtures run without a desktop.
type desktopRig struct {
	servicesRig
	shell     *fake.Shell
	clipboard *fake.Clipboard
}

func newDesktopRig(t *testing.T) desktopRig {
	t.Helper()
	rig := newServicesRig(t)
	shell := fake.NewShell()
	shell.SetApps([]platform.App{{Name: "Notes", ID: "com.example.notes", Path: "/Applications/Notes.app"}}, nil)
	clipboardPort := fake.NewClipboard()
	history, err := clipboard.Open(clipboard.NewMemoryPersistence(), fake.NewClock(), clipboard.Retention{MaxEntries: 50, MaxAge: 24 * time.Hour, MaxImageBytes: 1 << 20, MaxMemoryBytes: 1 << 20})
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	rig.deps.Clipboard = clipboardPort
	rig.deps.Shell = shell
	rig.deps.Windows = listedWindows{}
	rig.deps.Shortcuts = fixedShortcuts{}
	rig.deps.OCR = ocr.New(ocr.Deps{Text: fake.NewTextRecognizer(), Capture: fake.NewScreenCapturer(), FS: rig.deps.Files, TempDir: t.TempDir()})
	rig.deps.History = func() *clipboard.History { return history }
	rig.deps.OS = "linux"
	rig.deps.Frontmost = fixedFrontmost{}
	rig.deps.Notifier = fake.NewNotifier()
	rig.deps.Capabilities = platform.NewCapabilities(platform.CapabilityWindowManagement, platform.CapabilityTextRecognize, platform.CapabilityFrontmostApp, platform.CapabilityNotifications)
	return desktopRig{servicesRig: rig, shell: shell, clipboard: clipboardPort}
}

// TestDesktopFixtureUsesEveryDesktopServiceThroughTheChild runs the desktop fixture in its child process. Each row
// shows what the host answered, and the refusals never reach the fake shell.
func TestDesktopFixtureUsesEveryDesktopServiceThroughTheChild(t *testing.T) {
	rig := newDesktopRig(t)
	manifest := fixtureManifest("acme.desktop",
		api.CapabilityDesktopClipboard, api.CapabilityDesktopApps, api.CapabilityDesktopFiles,
		api.CapabilityWindowManagement, api.CapabilityClipboardHistory, api.CapabilityOCR, api.CapabilityShortcuts, api.CapabilitySystem)
	r := servicedRuntime(t, "acme.desktop", rig.servicesRig, manifest)

	want := map[string]string{
		"clipboard": "clipboard read=nvm desktop fixture",
		"windows":   "windows listed=1",
		"apps":      "apps listed=true",
		"frontmost": "frontmost named=true",
		"notify":    "notify shown=true",
		"history":   "history entries=0",
		"shortcuts": "shortcuts palette=Command+Alt+K",
		"system":    "system os=linux ocr=true",
		"ocr":       "ocr refused=true",
		"files":     "files refused=true",
		"links":     "links refused=true",
	}
	for query, title := range want {
		if got := rowTitle(t, r, query); got != title {
			t.Errorf("%s row = %q, want %q", query, got, title)
		}
	}
	if text, _ := rig.clipboard.ReadText(); text != "nvm desktop fixture" {
		t.Fatalf("clipboard holds %q", text)
	}
	if calls := rig.shell.Calls(); len(calls) != 0 {
		t.Fatalf("refused calls reached the shell: %+v", calls)
	}
}

// TestHostileFixtureWithNoDeclarationsIsRefusedByEveryProbe runs a fixture that declares nothing and calls each
// desktop service. Every probe that reports errors must be refused for permission.
func TestHostileFixtureWithNoDeclarationsIsRefusedByEveryProbe(t *testing.T) {
	rig := newDesktopRig(t)
	r := servicedRuntime(t, "acme.undeclared", rig.servicesRig, fixtureManifest("acme.undeclared"))
	if got := rowTitle(t, r, "undeclared"); got != "undeclared refused=7 of 7" {
		t.Fatalf("row = %q, want every probe refused", got)
	}
	if calls := rig.shell.Calls(); len(calls) != 0 {
		t.Fatalf("undeclared calls reached the shell: %+v", calls)
	}
}

// fixedFrontmost reports one app in front, as a window port does while that app has the keyboard.
type fixedFrontmost struct{}

func (fixedFrontmost) Frontmost(context.Context) (platform.WindowApp, bool, error) {
	return platform.WindowApp{PID: 42, BundleID: "com.example.notes", Name: "Notes"}, true, nil
}

var _ services.FrontmostSource = fixedFrontmost{}
var _ services.WindowEngine = listedWindows{}
var _ services.ShortcutTable = fixedShortcuts{}
