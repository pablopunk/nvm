package compositor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

func TestDesiredForBindsThePaletteAndActionsThroughTheCommandLine(t *testing.T) {
	d := DesiredFor("Control+Alt+K", map[string]string{
		shortcuts.CommandQuickNotes: "Alt+N",
		shortcuts.PaletteID:         "Control+Alt+K",
		"app:/opt/My App.desktop":   "Alt+A",
		shortcuts.CommandSnippets:   "",
	}, Identity{Executable: "/opt/Nevermind Next/nvm", Classes: []string{"nvm"}})
	var got []string
	for _, b := range d.Binds {
		got = append(got, b.Accelerator+" => "+b.Command)
	}
	want := []string{
		"Control+Alt+K => '/opt/Nevermind Next/nvm' run palette-toggle",
		"Alt+A => '/opt/Nevermind Next/nvm' run 'app:/opt/My App.desktop'",
		"Alt+N => '/opt/Nevermind Next/nvm' run command:quick-notes",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("binds\n%s", strings.Join(got, "\n"))
	}
	if !d.Appearance.Undecorated || d.Appearance.Classes[0] != "nvm" {
		t.Fatalf("appearance %+v", d.Appearance)
	}
}

func newService(t *testing.T) *Service {
	dir := t.TempDir()
	env := hyprland.Env{Getenv: func(string) string { return "" }, UndoFile: filepath.Join(dir, "undo.json")}
	return New(env, filepath.Join(dir, "state.json"), time.Now)
}

func TestOwnedShortcutsSkipThePortal(t *testing.T) {
	s := newService(t)
	portal := fake.NewHotkeys()
	changes := 0
	keys := s.Hotkeys(portal, func() { changes++ })
	if err := keys.Register("Control+Alt+K", func() {}); err != nil || len(portal.Registered()) != 1 {
		t.Fatalf("not owned: %v %v", err, portal.Registered())
	}
	keys.Unregister("Control+Alt+K")
	if err := s.saveState(State{OwnsShortcuts: true}); err != nil {
		t.Fatal(err)
	}
	if err := keys.Register("Control+Alt+K", func() {}); err != nil || len(portal.Registered()) != 0 {
		t.Fatalf("owned key reached the portal: %v %v", err, portal.Registered())
	}
	keys.Unregister("Control+Alt+K")
	if changes != 4 {
		t.Fatalf("changes %d", changes)
	}
	// Registering the same key again is not a change.
	keys.Register("Alt+N", func() {})
	keys.Register("Alt+N", func() {})
	if changes != 5 {
		t.Fatalf("changes %d", changes)
	}
}

func TestStateSurvivesARestart(t *testing.T) {
	s := newService(t)
	if err := s.saveState(State{OwnsShortcuts: true, Disabled: true}); err != nil {
		t.Fatal(err)
	}
	again := New(s.env, s.statePath, time.Now)
	if st := again.State(); !st.OwnsShortcuts || !st.Disabled {
		t.Fatalf("state lost: %+v", st)
	}
	if info, err := os.Stat(s.statePath); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("state file: %v %v", info, err)
	}
}

func TestLifecycleWithoutHyprlandFailsTruthfully(t *testing.T) {
	s := newService(t)
	if s.Present() {
		t.Fatal("present without a session")
	}
	if _, err := s.Preview(context.Background(), hyprland.Desired{}); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("preview: %v", err)
	}
	if _, err := s.Remove(context.Background()); err == nil {
		t.Fatal("removed without a session")
	}
	if last := s.Last(); last == nil || last.Err == nil || last.Step != "remove" {
		t.Fatalf("last %+v", last)
	}
}

// syncFixture is a service over a fake Hyprland 0.54 whose entry file includes the module.
type syncFixture struct {
	svc     *Service
	module  string
	reloads int
	// binds is what "hyprctl binds" answers.
	binds string
}

func newSyncFixture(t *testing.T) *syncFixture {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hyprland.conf"), []byte("monitor = , preferred, auto, 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	f := &syncFixture{module: filepath.Join(dir, "nevermind.conf"), binds: "[]"}
	run := func(_ context.Context, name string, args ...string) ([]byte, int, error) {
		switch call := name + " " + strings.Join(args, " "); call {
		case "hyprctl version -j":
			return []byte(`{"version":"0.54.0"}`), 0, nil
		case "Hyprland --help":
			return []byte("--verify-config"), 0, nil
		case "hyprctl reload":
			f.reloads++
		case "hyprctl binds -j":
			return []byte(f.binds), 0, nil
		}
		return []byte("[]"), 0, nil
	}
	env := hyprland.Env{
		Getenv: func(k string) string { return map[string]string{"HYPRLAND_INSTANCE_SIGNATURE": "sig"}[k] },
		Home:   home, Run: run, UndoFile: filepath.Join(home, "undo.json"),
	}
	f.svc = New(env, filepath.Join(home, "state.json"), time.Now)
	return f
}

func (f *syncFixture) desired(accelerator string) hyprland.Desired {
	return DesiredFor("Control+Alt+K", map[string]string{shortcuts.CommandQuickNotes: accelerator}, Identity{Executable: "/opt/nvm", Classes: []string{"nvm"}})
}

func (f *syncFixture) moduleText(t *testing.T) string {
	data, err := os.ReadFile(f.module)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// firstSync is the automatic first run: it adds the include, writes the module and reloads.
func (f *syncFixture) firstSync(t *testing.T, accelerator string) {
	if _, err := f.svc.Sync(context.Background(), f.desired(accelerator)); err != nil {
		t.Fatal(err)
	}
}

func TestFirstSyncAddsTheIncludeWritesTheModuleAndKeepsOwnershipWhenTheBindListLags(t *testing.T) {
	f := newSyncFixture(t) // its compositor lists no binds, so the load check answers "missing"
	outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+N"))
	if err != nil || !outcome.Wrote || !outcome.OwnershipChanged || f.reloads != 1 {
		t.Fatalf("outcome %+v err %v reloads %d", outcome, err, f.reloads)
	}
	if outcome.Result.Loaded != hyprland.LoadedMissing || !f.svc.State().OwnsShortcuts {
		t.Fatalf("loaded %v owns %v", outcome.Result.Loaded, f.svc.State().OwnsShortcuts)
	}
	entry, _ := os.ReadFile(filepath.Join(filepath.Dir(f.module), "hyprland.conf"))
	if !strings.Contains(string(entry), "nevermind.conf") || !strings.HasPrefix(string(entry), "monitor = , preferred, auto, 1\n") {
		t.Fatalf("entry:\n%s", entry)
	}
	// A later shortcut change still syncs, and the module carries the window rule for the border only.
	f.reloads = 0
	if outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+M")); err != nil || !outcome.Wrote || f.reloads != 1 {
		t.Fatalf("outcome %+v err %v reloads %d", outcome, err, f.reloads)
	}
	text := f.moduleText(t)
	if !strings.Contains(text, "border_size 0") || strings.Contains(strings.ToLower(text), "opacity") || strings.Contains(text, "alpha") {
		t.Fatalf("module:\n%s", text)
	}
}

func TestSyncRewritesAStaleModuleOnceAndReloads(t *testing.T) {
	f := newSyncFixture(t)
	f.firstSync(t, "Alt+N")
	f.reloads = 0
	outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+M"))
	if err != nil || !outcome.Wrote || f.reloads != 1 {
		t.Fatalf("outcome %+v err %v reloads %d", outcome, err, f.reloads)
	}
	if text := f.moduleText(t); !strings.Contains(text, "ALT, M") || strings.Contains(text, "ALT, N") {
		t.Fatalf("module still holds the old key:\n%s", text)
	}
	if outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+M")); err != nil || outcome.Wrote || f.reloads != 1 {
		t.Fatalf("second sync %+v err %v reloads %d", outcome, err, f.reloads)
	}
}

func TestSyncWritesNothingWhenTheModuleIsCurrent(t *testing.T) {
	f := newSyncFixture(t)
	f.firstSync(t, "Alt+N")
	f.reloads = 0
	before := f.moduleText(t)
	if outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+N")); err != nil || outcome.Wrote || f.reloads != 0 || f.moduleText(t) != before {
		t.Fatalf("outcome %+v err %v reloads %d", outcome, err, f.reloads)
	}
}

func TestRemovingAShortcutRemovesItsBind(t *testing.T) {
	f := newSyncFixture(t)
	f.firstSync(t, "Alt+N")
	d := f.desired("")
	if outcome, err := f.svc.Sync(context.Background(), d); err != nil || !outcome.Wrote {
		t.Fatalf("outcome %+v err %v", outcome, err)
	}
	if text := f.moduleText(t); strings.Contains(text, "quick-notes") || !strings.Contains(text, "palette-toggle") {
		t.Fatalf("module:\n%s", text)
	}
}

func TestChangingThePaletteShortcutUpdatesItsBind(t *testing.T) {
	f := newSyncFixture(t)
	f.firstSync(t, "Alt+N")
	d := DesiredFor("Control+space", nil, Identity{Executable: "/opt/nvm", Classes: []string{"nvm"}})
	if _, err := f.svc.Sync(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if text := f.moduleText(t); !strings.Contains(text, "CTRL, space") || strings.Contains(text, "CTRL ALT, K") {
		t.Fatalf("module:\n%s", text)
	}
}

func TestSyncSkipsAShortcutTheUsersOwnBindUses(t *testing.T) {
	f := newSyncFixture(t)
	entry := filepath.Join(filepath.Dir(f.module), "hyprland.conf")
	if err := os.WriteFile(entry, []byte("bind = ALT, N, exec, foot\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	f.binds = `[{"modmask":8,"key":"N","dispatcher":"exec","arg":"foot"}]`
	outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+N"))
	if err != nil || len(outcome.Skipped) != 1 || strings.Contains(f.moduleText(t), "quick-notes") {
		t.Fatalf("outcome %+v err %v\n%s", outcome, err, f.moduleText(t))
	}
	if data, _ := os.ReadFile(entry); !strings.HasPrefix(string(data), "bind = ALT, N, exec, foot\n") {
		t.Fatalf("the user's line changed:\n%s", data)
	}
}

func TestSyncDoesNothingAfterTheUserRemovedTheConfigUntilItIsEnabledAgain(t *testing.T) {
	f := newSyncFixture(t)
	f.firstSync(t, "Alt+N")
	if _, err := f.svc.Remove(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.reloads = 0
	if outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+N")); err != nil || outcome.Wrote || f.reloads != 0 {
		t.Fatalf("synced after removal: %+v %v", outcome, err)
	}
	if _, err := os.Stat(f.module); err == nil {
		t.Fatal("module came back")
	}
	if err := f.svc.Enable(); err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.svc.Sync(context.Background(), f.desired("Alt+N")); err != nil || !outcome.Wrote {
		t.Fatalf("not synced after enabling: %+v %v", outcome, err)
	}
}

// Syncs that overlap (the startup sync and one asked for by a change right after) must not fail against each other: a
// plan prepared before the other's write is refused as a concurrent edit, which would leave the module as the older
// sync wrote it with nothing to write it again.
func TestOverlappingSyncsApplyOneAfterTheOther(t *testing.T) {
	for round := 0; round < 30; round++ {
		f := newSyncFixture(t)
		f.firstSync(t, "Alt+A")
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for _, accelerator := range []string{"Alt+B", "Alt+C"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := f.svc.Sync(context.Background(), f.desired(accelerator))
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: overlapping sync failed: %v", round, err)
			}
		}
	}
}
