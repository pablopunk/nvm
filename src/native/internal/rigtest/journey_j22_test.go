//go:build rig

package rigtest

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// J22 (PLAN.md 3.1): quit an app, then uninstall it through a reviewed list. The app is a fake desktop entry in the
// isolated HOME whose program is feh, linked under the name fakeapp-sleeper, so the process is named like the entry
// and owns a real X window with its own PID, as a GUI app does. feh exits on SIGTERM, so Quit is observed as the
// process ending. The uninstall moves only the entry and its own data folder, into a temp Trash that XDG_DATA_HOME
// points at, so the real Trash is never touched.

const (
	fakeAppName     = "Fake Rig App"
	fakeAppID       = "fakeapp"
	fakeAppSleeper  = "fakeapp-sleeper" // 15 bytes: the kernel's process-name limit, so the name is not truncated
	fakeAppImage    = "fakeapp.png"
	fakeAppTitle    = "fakeapp-window"
	sleeperExitWait = 15 * time.Second
	actionMenuKey   = "ctrl+k"
	uninstallIndex  = 3 // Open, Show in Folder, Copy Path, then Uninstall: the app is not running by then
)

// seedFakeApp writes the desktop entry, its program and its own data folder, and one unrelated folder that the
// uninstall must not touch. It returns the program path.
func seedFakeApp(t *testing.T, home string) string {
	t.Helper()
	program := filepath.Join(home, "bin", fakeAppSleeper)
	must(t, os.MkdirAll(filepath.Dir(program), 0o755))
	feh, err := exec.LookPath("feh")
	must(t, err)
	must(t, os.Symlink(feh, program))
	image := filepath.Join(home, "bin", fakeAppImage)
	must(t, exec.Command("convert", "-size", "320x200", "xc:steelblue", image).Run())
	entry := filepath.Join(home, ".local", "share", "applications", fakeAppID+".desktop")
	writeFile(t, entry, "[Desktop Entry]\nType=Application\nName="+fakeAppName+"\nExec="+program+" %U\n", 0o644)
	writeFile(t, filepath.Join(home, ".local", "share", fakeAppID, "config.toml"), "rig = true\n", 0o644)
	writeFile(t, filepath.Join(home, ".local", "share", "otherapp", "keep.txt"), "not ours\n", 0o644)
	return program
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// startSleeper runs the fake program and returns a channel that receives when it has exited and been reaped.
func startSleeper(t *testing.T, program string) <-chan error {
	t.Helper()
	image := filepath.Join(filepath.Dir(program), fakeAppImage)
	cmd := exec.Command(program, "--title", fakeAppTitle, "-g", "320x200+400+300", image)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		exited <- cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			_ = cmd.Process.Kill()
		}
	})
	return exited
}

// treeSnapshot maps every path under root to its kind and content digest. Three places are left out: the app's own
// settings folder (the app may write it while it runs), and the Trash, which is where the uninstall is meant to put
// things and is asserted on its own.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	excluded := []string{filepath.Join(".config", "nvm-native"), filepath.Join(".local", "share", "Trash")}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		if isUnder(rel, excluded) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// isUnder reports whether rel is one of the folders or inside one of them.
func isUnder(rel string, folders []string) bool {
	for _, folder := range folders {
		if rel == folder || strings.HasPrefix(rel, folder+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// diffTrees returns the paths removed, added and changed between two snapshots, sorted.
func diffTrees(before, after map[string]string) (removed, added, changed []string) {
	for p, v := range before {
		w, ok := after[p]
		switch {
		case !ok:
			removed = append(removed, p)
		case w != v:
			changed = append(changed, p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			added = append(added, p)
		}
	}
	sort.Strings(removed)
	sort.Strings(added)
	sort.Strings(changed)
	return removed, added, changed
}

// TestJourneyJ22QuitAndUninstallFakeApp is J22: the running fake app shows as running, Force Quit Apps quits it
// (SIGTERM, the process exits), and the uninstall review lists only its entry and its own data folder; confirming
// moves exactly those into the temp Trash and nothing else under HOME changes.
// Not covered by the rig: a real GUI application, Quit All Apps, the keyboard-only pass, macOS and Windows
// discovery (those are untested), and the escalation to SIGKILL after the grace period.
func TestJourneyJ22QuitAndUninstallFakeApp(t *testing.T) {
	app := Launch(t, Options{})
	home := app.Home()
	program := seedFakeApp(t, home)
	exited := startSleeper(t, program)

	must(t, app.SummonPalette())
	must(t, app.Type("fake rig"))
	_, err := app.WaitFor(fakeAppName, elementWait)
	must(t, err)
	_, err = app.WaitFor("Running", elementWait)
	must(t, err)
	shoot(t, app, "j22-1-running")

	// Quit through Force Quit Apps: the first row's primary action is Quit, which sends SIGTERM only.
	must(t, app.Key("Escape"))
	must(t, app.Type("force quit"))
	_, err = app.WaitFor("Force Quit Apps", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Search running apps", elementWait)
	must(t, err)
	_, err = app.WaitFor(fakeAppName, elementWait)
	must(t, err)
	shoot(t, app, "j22-2-force-quit-view")
	must(t, app.Key("Return"))
	select {
	case <-exited:
	case <-time.After(sleeperExitWait):
		t.Fatalf("%s is still running %s after Quit was chosen", fakeAppName, sleeperExitWait)
	}
	t.Logf("%s exited after Quit", fakeAppName)

	// Uninstall through the action menu of the (now idle) app's row.
	must(t, app.Key("Escape"))
	must(t, app.DismissWithEscape())
	must(t, app.SummonPalette())
	must(t, app.Type("fake rig"))
	_, err = app.WaitFor(fakeAppName, elementWait)
	must(t, err)
	must(t, app.Key(actionMenuKey))
	for range uninstallIndex {
		must(t, app.Key("Down"))
	}
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Filter items to move to Trash", elementWait)
	must(t, err)
	_, err = app.WaitFor("~/.local/share/"+fakeAppID, elementWait)
	must(t, err)
	shoot(t, app, "j22-3-uninstall-review")

	before := treeSnapshot(t, home)
	must(t, app.Key("Return")) // Review selected items
	_, err = app.WaitFor("Move 2 items to Trash?", elementWait)
	must(t, err)
	shoot(t, app, "j22-4-confirm")
	must(t, app.Key("Return")) // Confirm: moves the two items to the Trash
	_, err = app.WaitFor("2 items moved to Trash", elementWait)
	must(t, err)
	shoot(t, app, "j22-5-result")

	after := treeSnapshot(t, home)
	removed, added, changed := diffTrees(before, after)
	wantRemoved := []string{
		filepath.Join(".local", "share", "applications", fakeAppID+".desktop"),
		filepath.Join(".local", "share", fakeAppID),
		filepath.Join(".local", "share", fakeAppID, "config.toml"),
	}
	sort.Strings(wantRemoved)
	if !reflect.DeepEqual(removed, wantRemoved) {
		t.Fatalf("removed under HOME = %v, want %v", removed, wantRemoved)
	}
	if len(added) != 0 || len(changed) != 0 {
		t.Fatalf("HOME changed beyond the uninstall: added %v, changed %v", added, changed)
	}
	trashFiles := filepath.Join(home, ".local", "share", "Trash", "files")
	for _, name := range []string{fakeAppID, fakeAppID + ".desktop"} {
		if _, err := os.Stat(filepath.Join(trashFiles, name)); err != nil {
			t.Fatalf("%s is not in the isolated Trash: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "otherapp", "keep.txt")); err != nil {
		t.Fatalf("an unrelated folder was touched: %v", err)
	}
}
