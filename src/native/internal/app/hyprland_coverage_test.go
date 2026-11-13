package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/shortcuts/compositor"
)

// hyprlandSession is a fake Hyprland 0.54 whose entry file already includes Nevermind's module, so it counts reloads.
type hyprlandSession struct {
	module  string
	reloads atomic.Int32
	mu      sync.Mutex
}

func newHyprlandSession(t *testing.T, owned bool) (*hyprlandSession, *compositor.Service) {
	t.Helper()
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "sig")
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hyprland.conf"), []byte("monitor = , preferred, auto, 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	s := &hyprlandSession{module: filepath.Join(dir, "nevermind.conf")}
	run := func(_ context.Context, name string, args ...string) ([]byte, int, error) {
		switch call := name + " " + strings.Join(args, " "); call {
		case "hyprctl version -j":
			return []byte(`{"version":"0.54.0"}`), 0, nil
		case "Hyprland --help":
			return []byte("--verify-config"), 0, nil
		case "hyprctl reload":
			s.reloads.Add(1)
		}
		return []byte("[]"), 0, nil
	}
	env := hyprland.Env{
		Getenv: func(k string) string { return map[string]string{"HYPRLAND_INSTANCE_SIGNATURE": "sig"}[k] },
		Home:   home, Run: run, UndoFile: filepath.Join(home, "undo.json"),
	}
	statePath := filepath.Join(home, "state.json")
	if owned {
		if err := os.WriteFile(statePath, []byte(`{"ownsShortcuts":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return s, compositor.New(env, statePath, time.Now)
}

func (s *hyprlandSession) text(t *testing.T) string {
	t.Helper()
	data, _ := os.ReadFile(s.module)
	return string(data)
}

func waitForReloads(t *testing.T, s *hyprlandSession, want int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for s.reloads.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("reloads %d, want %d", s.reloads.Load(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Under Hyprland with the module in place, every runnable shortcut (built-in, window management, app and extension) is
// written to the module and none goes through the portal; one that arrives with a late extension listing syncs again.
func TestOwnedModuleCarriesEveryRunnableShortcutAndTheLateOnesTriggerASecondSync(t *testing.T) {
	session, svc := newHyprlandSession(t, true)
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	p.compositor = svc
	p.shortcutMgr = nil
	app := "/usr/share/applications/org.gnome.Calculator.desktop"
	if err := p.ports.FS.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	chosen := map[string]string{
		shortcuts.CommandKeyboardShortcuts:                          "Control+Alt+A",
		shortcuts.CommandClipboardHistory:                           "Control+Alt+B",
		shortcuts.CommandEmojiSymbols:                               "Control+Alt+C",
		shortcuts.CommandDictate:                                    "Control+Alt+D",
		shortcuts.CommandNewFloatingNote:                            "Super+Ctrl+Alt+Y",
		"extension-action:window-management:layout.reasonable-size": "Control+Alt+E",
		shortcuts.AppActionID(app):                                  "Control+Alt+F",
	}
	late := "extension:acme.theme:toggle-theme"
	chosen[late] = "Control+Alt+G"
	m := p.shortcutsManager()
	if status := m.Start(); status.Unavailable() {
		t.Fatalf("palette status %+v", status)
	}
	for id, accelerator := range chosen {
		if err := m.SetAction(id, accelerator); err != nil {
			t.Fatal(err)
		}
	}
	if portal := p.ports.Hotkeys.(interface{ Registered() []string }).Registered(); len(portal) != 0 {
		t.Fatalf("the portal bound %v while the compositor owns the shortcuts", portal)
	}
	p.compositorReady.Store(true)
	p.requestCompositorSync()
	waitForReloads(t, session, 1)
	first := session.text(t)
	for id := range chosen {
		if id != late && !strings.Contains(first, id) {
			t.Errorf("module lacks %s:\n%s", id, first)
		}
	}
	if strings.Contains(first, late) {
		t.Fatal("a command that no extension listed yet was written")
	}
	mgr := p.extensionManager()
	if err := mgr.Install("acme.theme", fixtureFiles(t, "acme.theme"), true); err != nil {
		t.Fatal(err)
	}
	waitForState(t, mgr, "acme.theme", supervisor.StateRunning)
	mgr.Commands(context.Background())
	p.extensionCommandsChanged()
	waitForReloads(t, session, 2)
	if !strings.Contains(session.text(t), late) {
		t.Fatalf("the late command is not in the module:\n%s", session.text(t))
	}
}

// The first run under Hyprland binds through the portal, because the compositor does not own the shortcuts yet. The
// first sync writes the module, and the portal then holds none of them.
func TestFirstRunHandsEveryRunnableShortcutFromThePortalToTheModule(t *testing.T) {
	session, svc := newHyprlandSession(t, false)
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	p.compositor = svc
	p.shortcutMgr = nil
	chosen := map[string]string{
		shortcuts.CommandKeyboardShortcuts:                          "Control+Alt+A",
		shortcuts.CommandClipboardHistory:                           "Control+Alt+B",
		shortcuts.CommandEmojiSymbols:                               "Control+Alt+C",
		"extension-action:window-management:layout.reasonable-size": "Control+Alt+E",
	}
	m := p.shortcutsManager()
	for id, accelerator := range chosen {
		if err := m.SetAction(id, accelerator); err != nil {
			t.Fatal(err)
		}
	}
	m.Start()
	portal := p.ports.Hotkeys.(interface{ Registered() []string })
	if len(portal.Registered()) == 0 {
		t.Fatal("the portal bound nothing before the compositor owned the shortcuts")
	}
	p.compositorReady.Store(true)
	p.requestCompositorSync()
	waitForReloads(t, session, 1)
	deadline := time.Now().Add(5 * time.Second)
	for len(portal.Registered()) != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if left := portal.Registered(); len(left) != 0 {
		t.Fatalf("the portal still holds %v", left)
	}
	text := session.text(t)
	for id := range chosen {
		if !strings.Contains(text, id) {
			t.Errorf("module lacks %s:\n%s", id, text)
		}
	}
}

// A change made while a sync runs is not lost to it: the sync runs once more from the shortcuts as they are, so the module
// ends holding the last change.
func TestAChangeDuringASyncIsSyncedAfterIt(t *testing.T) {
	session, svc := newHyprlandSession(t, true)
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	p.compositor = svc
	p.shortcutMgr = nil
	m := p.shortcutsManager()
	p.compositorReady.Store(true)
	for _, accelerator := range []string{"Control+Alt+A", "Control+Alt+B", "Control+Alt+C"} {
		if err := m.SetAction(shortcuts.CommandKeyboardShortcuts, accelerator); err != nil {
			t.Fatal(err)
		}
		p.requestCompositorSync()
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(session.text(t), "CTRL ALT, C") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if text := session.text(t); !strings.Contains(text, "CTRL + ALT + C") && !strings.Contains(text, "CTRL ALT, C") {
		t.Fatalf("the last change never reached the module:\n%s", text)
	}
}
