package hyprland

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeCompositor answers the programs the adapter runs, and records them.
type fakeCompositor struct {
	version      string
	help         string
	verifyExit   int
	errorsAfter  []string // configerrors after a reload
	reloads      int
	binds        string
	calls        []string
	errorsBefore []string
}

func (f *fakeCompositor) run(_ context.Context, name string, args ...string) ([]byte, int, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	switch {
	case call == "hyprctl version -j":
		return []byte(`{"version":"` + f.version + `"}`), 0, nil
	case call == "Hyprland --help":
		return []byte(f.help), 0, nil
	case strings.HasPrefix(call, "Hyprland --verify-config"):
		return []byte("config error: bad"), f.verifyExit, nil
	case call == "hyprctl reload":
		f.reloads++
		return []byte("ok"), 0, nil
	case call == "hyprctl configerrors":
		if f.reloads > 0 {
			return []byte(strings.Join(f.errorsAfter, "\n")), 0, nil
		}
		return []byte(strings.Join(f.errorsBefore, "\n")), 0, nil
	case call == "hyprctl binds -j":
		return []byte(f.binds), 0, nil
	}
	return nil, 0, errors.New("unexpected " + call)
}

type fixture struct {
	t    *testing.T
	home string
	dir  string
	fake *fakeCompositor
	env  Env
}

func newFixture(t *testing.T, version string, files map[string]string) *fixture {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	fake := &fakeCompositor{version: version, help: "--verify-config  check the config", binds: "[]"}
	env := Env{
		Getenv: func(k string) string {
			if k == "HYPRLAND_INSTANCE_SIGNATURE" {
				return "sig"
			}
			return ""
		},
		Home: home, Run: fake.run, UndoFile: filepath.Join(home, "undo.json"),
	}
	return &fixture{t: t, home: home, dir: dir, fake: fake, env: env}
}

func (f *fixture) read(name string) string {
	data, _ := os.ReadFile(filepath.Join(f.dir, name))
	return string(data)
}

var desired = Desired{Binds: []Bind{{Accelerator: "Control+Alt+K", Command: exe + " run palette-toggle"}}}

func TestDetect(t *testing.T) {
	off := Detect(context.Background(), Env{Getenv: func(string) string { return "" }})
	if off.Running || off.Ready() {
		t.Fatal("detected Hyprland without its instance signature")
	}
	f := newFixture(t, "0.56.0", map[string]string{"hyprland.lua": "-- mine\n"})
	s := Detect(context.Background(), f.env)
	if !s.Ready() || s.Format != FormatLua || s.ModulePath != filepath.Join(f.dir, "nevermind.lua") || !s.VerifyConfig {
		t.Fatalf("%+v", s)
	}
	both := newFixture(t, "0.56.0", map[string]string{"hyprland.lua": "", "hyprland.conf": ""})
	if s := Detect(context.Background(), both.env); s.Ready() || !s.Manual || !strings.Contains(s.Problem, "both") {
		t.Fatalf("ambiguous entry: %+v", s)
	}
	old := newFixture(t, "0.44.0", map[string]string{"hyprland.conf": ""})
	if s := Detect(context.Background(), old.env); s.Ready() {
		t.Fatalf("old version: %+v", s)
	}
	conf := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "", "hyprland.lua": ""})
	if s := Detect(context.Background(), conf.env); !s.Ready() || s.Format != FormatConf {
		t.Fatalf("0.54 reads hyprland.conf: %+v", s)
	}
	custom := newFixture(t, "0.56.0", map[string]string{"rice.conf": ""})
	custom.env.CompositorArgs = func() ([]string, error) {
		return []string{"Hyprland", "-c", filepath.Join(custom.dir, "rice.conf")}, nil
	}
	if s := Detect(context.Background(), custom.env); !s.Ready() || s.Entry != filepath.Join(custom.dir, "rice.conf") || s.Format != FormatConf {
		t.Fatalf("custom entry: %+v", s)
	}
}

func applyFresh(t *testing.T, f *fixture) Session {
	t.Helper()
	s := Detect(context.Background(), f.env)
	plan, err := Prepare(s, desired)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Preview(true), s.ModulePath) || !strings.Contains(plan.Preview(true), s.Entry) {
		t.Fatalf("the preview does not name the files:\n%s", plan.Preview(true))
	}
	if _, err := Apply(context.Background(), f.env, plan, true); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestApplyWritesModuleAndIncludeThenReloadsAndConfirms(t *testing.T) {
	user := "monitor = , preferred, auto, 1\n"
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": user})
	f.fake.binds = `[{"modmask":12,"key":"K","dispatcher":"exec","arg":"` + exe + ` run palette-toggle"},{"modmask":12,"key":"K","dispatcher":"exec","arg":"rofi"}]`
	s := Detect(context.Background(), f.env)
	plan, _ := Prepare(s, desired)
	result, err := Apply(context.Background(), f.env, plan, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || !result.Reloaded || result.Loaded != LoadedConfirmed || len(result.Conflicts) != 1 {
		t.Fatalf("%+v", result)
	}
	entry := f.read("hyprland.conf")
	if !strings.HasPrefix(entry, user) || !strings.Contains(entry, "source = "+s.ModulePath) {
		t.Fatalf("entry:\n%s", entry)
	}
	if info, _ := os.Stat(filepath.Join(f.dir, "hyprland.conf")); info.Mode().Perm() != 0o640 {
		t.Errorf("entry mode became %v", info.Mode().Perm())
	}
	if !slices.Contains(f.fake.calls, "Hyprland --verify-config --config "+s.Entry) {
		t.Errorf("not validated: %v", f.fake.calls)
	}
	again, _ := Prepare(Detect(context.Background(), f.env), desired)
	if !again.ModuleUnchanged() || !again.IncludePresent {
		t.Fatal("a second apply would change something")
	}
	if r, err := Apply(context.Background(), f.env, again, true); err != nil || r.Changed {
		t.Fatalf("second apply: %+v %v", r, err)
	}
}

func TestApplyWithoutApprovalLeavesTheEntryAlone(t *testing.T) {
	f := newFixture(t, "0.56.0", map[string]string{"hyprland.lua": "-- mine\n"})
	plan, _ := Prepare(Detect(context.Background(), f.env), desired)
	if !strings.Contains(plan.Preview(false), "yourself") {
		t.Fatalf("manual include not offered:\n%s", plan.Preview(false))
	}
	result, err := Apply(context.Background(), f.env, plan, false)
	if err != nil || result.Reloaded || f.fake.reloads != 0 {
		t.Fatalf("%+v %v", result, err)
	}
	if f.read("hyprland.lua") != "-- mine\n" || !strings.Contains(f.read("nevermind.lua"), "hl.bind(") {
		t.Fatal("wrong files written")
	}
}

func TestValidationFailureRestoresEverything(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "user\n"})
	f.fake.verifyExit = 1
	plan, _ := Prepare(Detect(context.Background(), f.env), desired)
	if _, err := Apply(context.Background(), f.env, plan, true); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err %v", err)
	}
	if f.read("hyprland.conf") != "user\n" || f.read("nevermind.conf") != "" || f.fake.reloads != 0 {
		t.Fatal("a rejected config was left in place or reloaded")
	}
}

func TestNewConfigErrorsAfterReloadRollBack(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "user\n"})
	f.fake.errorsBefore = []string{"old error"}
	f.fake.errorsAfter = []string{"old error", "nevermind.conf:5: invalid dispatcher"}
	plan, _ := Prepare(Detect(context.Background(), f.env), desired)
	_, err := Apply(context.Background(), f.env, plan, true)
	if err == nil || !strings.Contains(err.Error(), "invalid dispatcher") || strings.Contains(err.Error(), "old error") {
		t.Fatalf("err %v", err)
	}
	if f.read("hyprland.conf") != "user\n" || f.read("nevermind.conf") != "" || f.fake.reloads != 2 {
		t.Fatalf("not rolled back and reloaded: %d reloads", f.fake.reloads)
	}
}

func TestEditsAfterThePreviewAreNotOverwritten(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "user\n"})
	plan, _ := Prepare(Detect(context.Background(), f.env), desired)
	os.WriteFile(filepath.Join(f.dir, "hyprland.conf"), []byte("user edited\n"), 0o644)
	if _, err := Apply(context.Background(), f.env, plan, true); !errors.Is(err, ErrChangedSincePreview) {
		t.Fatalf("err %v", err)
	}
	if f.read("hyprland.conf") != "user edited\n" || f.read("nevermind.conf") != "" {
		t.Fatal("wrote after a concurrent edit")
	}
}

func TestUndoRestoresThePreviousStateOnce(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "user\n"})
	s := applyFresh(t, f)
	// The user edits their config after the apply; undo keeps that edit.
	os.WriteFile(s.Entry, []byte(f.read("hyprland.conf")+"bind = SUPER, Q, exec, kitty\n"), 0o644)
	if _, err := Undo(context.Background(), f.env, s); err != nil {
		t.Fatal(err)
	}
	if f.read("hyprland.conf") != "user\nbind = SUPER, Q, exec, kitty\n" {
		t.Fatalf("entry after undo:\n%q", f.read("hyprland.conf"))
	}
	if _, err := os.Stat(s.ModulePath); !os.IsNotExist(err) {
		t.Fatal("the module outlived the undo")
	}
	if _, err := Undo(context.Background(), f.env, s); !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("second undo: %v", err)
	}
}

func TestUndoRefusesAModuleEditedByHand(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "user\n"})
	s := applyFresh(t, f)
	os.WriteFile(s.ModulePath, []byte("# hand edit\n"), 0o644)
	if _, err := Undo(context.Background(), f.env, s); err == nil {
		t.Fatal("undo overwrote a hand edit")
	}
}

func TestRemoveTakesOnlyNevermindsLinesAndIsIdempotent(t *testing.T) {
	f := newFixture(t, "0.56.0", map[string]string{"hyprland.lua": "-- mine\n"})
	s := applyFresh(t, f)
	if _, err := Remove(context.Background(), f.env, s); err != nil {
		t.Fatal(err)
	}
	if f.read("hyprland.lua") != "-- mine\n" {
		t.Fatalf("entry after removal: %q", f.read("hyprland.lua"))
	}
	if r, err := Remove(context.Background(), f.env, s); err != nil || r.Changed {
		t.Fatalf("second removal: %+v %v", r, err)
	}
}

func TestAModuleNevermindDidNotWriteIsLeftAlone(t *testing.T) {
	f := newFixture(t, "0.54.0", map[string]string{"hyprland.conf": "", "nevermind.conf": "# someone else's\n"})
	s := Detect(context.Background(), f.env)
	if _, err := Prepare(s, desired); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("prepare: %v", err)
	}
	if _, err := Remove(context.Background(), f.env, s); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("remove: %v", err)
	}
	if f.read("nevermind.conf") != "# someone else's\n" {
		t.Fatal("deleted a file Nevermind did not write")
	}
}

func TestASymlinkedEntryStaysALink(t *testing.T) {
	f := newFixture(t, "0.54.0", nil)
	dotfiles := filepath.Join(f.home, "dotfiles")
	os.MkdirAll(dotfiles, 0o755)
	os.WriteFile(filepath.Join(dotfiles, "hyprland.conf"), []byte("user\n"), 0o644)
	if err := os.Symlink(filepath.Join(dotfiles, "hyprland.conf"), filepath.Join(f.dir, "hyprland.conf")); err != nil {
		t.Skip(err)
	}
	applyFresh(t, f)
	if info, _ := os.Lstat(filepath.Join(f.dir, "hyprland.conf")); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the entry link was replaced by a file")
	}
	data, _ := os.ReadFile(filepath.Join(dotfiles, "hyprland.conf"))
	if !strings.Contains(string(data), "source = ") {
		t.Fatal("the include did not reach the link's target")
	}
}
