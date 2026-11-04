package apps

import (
	"path"
	"reflect"
	"strings"
	"testing"
)

// Linux uninstall: user-level desktop applications only. These tests have no 1.x counterpart (the 1.x uninstall was
// macOS-only); they pin the rules the design adds: exact identifier data, no name matching, no system packages.

const (
	linuxHome    = "/home/tester"
	linuxUID     = 1000
	linuxDesktop = "/home/tester/.local/share/applications/fakeapp.desktop"
	linuxData    = "/home/tester/.local/share/fakeapp"
)

type linuxFixture struct {
	u         *Uninstaller
	tree      *memTree
	running   bool
	quitCalls []string
}

func newLinuxFixture(t *testing.T, mutate func(tree *memTree)) *linuxFixture {
	t.Helper()
	tree := newMemTree()
	tree.dir(linuxHome, linuxUID)
	tree.file(linuxDesktop, linuxUID, 0o644, []byte("[Desktop Entry]\nName=Fake App\nExec=/home/tester/.local/share/fakeapp/bin/fakeapp %U\n"))
	tree.file(path.Join(linuxData, "config.toml"), linuxUID, 0o644, []byte("x"))
	tree.dir(path.Join(linuxHome, ".local", "share", "otherapp"), linuxUID)
	if mutate != nil {
		mutate(tree)
	}
	f := &linuxFixture{tree: tree}
	f.u = NewUninstaller(UninstallDeps{
		Platform:  "linux",
		Home:      linuxHome,
		UID:       linuxUID,
		Files:     tree,
		Trash:     tree.trash,
		IsRunning: func(string) (bool, error) { return f.running, nil },
		RequestQuit: func(p string) error {
			f.quitCalls = append(f.quitCalls, p)
			f.running = false
			return nil
		},
		NevermindAppPath:  "/usr/bin/nevermind",
		NevermindBundleID: NeverMindBundleID,
	})
	return f
}

// Discovery lists the desktop entry and the data folder named by its identifier, and nothing else under the user's
// data folder: "otherapp" is not matched by name.
func TestLinuxDiscoveryListsOnlyTheEntryAndItsOwnDataFolder(t *testing.T) {
	f := newLinuxFixture(t, nil)
	d := f.u.Discover(linuxDesktop)
	if !d.Ready {
		t.Fatalf("discovery unavailable: %+v", d)
	}
	var paths []string
	for _, c := range d.Snapshot.Candidates {
		paths = append(paths, c.Path)
	}
	if want := []string{linuxDesktop, linuxData}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("candidates = %v, want %v", paths, want)
	}
	if !d.Snapshot.Candidates[1].SelectedByDefault {
		t.Fatal("the app's own data folder should be selected by default when nothing else claims it")
	}
}

// A system-wide desktop entry is refused with advice to use the package manager, and nothing is probed.
func TestLinuxRefusesSystemPackagesWithAdvice(t *testing.T) {
	f := newLinuxFixture(t, func(tree *memTree) {
		tree.dir("/usr/share/applications", 0)
		tree.file("/usr/share/applications/sysapp.desktop", 0, 0o644, []byte("[Desktop Entry]\nName=Sys\nExec=sys\n"))
	})
	d := f.u.Discover("/usr/share/applications/sysapp.desktop")
	if d.Ready || d.ReasonCode != codeSystemPkg {
		t.Fatalf("discovery = %+v, want system-package refusal", d)
	}
	for _, word := range []string{"package manager", "apt", "flatpak uninstall --system"} {
		if !strings.Contains(d.Message, word) {
			t.Fatalf("message %q does not mention %q", d.Message, word)
		}
	}
}

// Trash moves exactly the reviewed items, the data folder before the entry.
func TestLinuxTrashMovesOnlyTheReviewedItems(t *testing.T) {
	f := newLinuxFixture(t, nil)
	d := f.u.Discover(linuxDesktop)
	result := f.u.Trash(d.Snapshot, selectIDs(d.Snapshot.Candidates...))
	if result.Status != StatusComplete {
		t.Fatalf("status = %s, untouched %+v", result.Status, result.Untouched)
	}
	if want := []string{linuxData, linuxDesktop}; !reflect.DeepEqual(f.tree.trashed, want) {
		t.Fatalf("trashed = %v, want %v", f.tree.trashed, want)
	}
	if !f.tree.has(path.Join(linuxHome, ".local", "share", "otherapp")) {
		t.Fatal("an unrelated data folder was moved")
	}
}

// A running app is asked to quit first; its entry is moved only after it has exited.
func TestLinuxQuitsRunningAppBeforeMovingIt(t *testing.T) {
	f := newLinuxFixture(t, nil)
	f.running = true
	d := f.u.Discover(linuxDesktop)
	result := f.u.Trash(d.Snapshot, selectIDs(d.Snapshot.Candidates...))
	if result.Status != StatusComplete || len(f.quitCalls) != 1 {
		t.Fatalf("result %+v, quit calls %v", result, f.quitCalls)
	}
}

// Another entry with the same identifier in a system folder means the data may belong to it: the data is not
// selected by default.
func TestLinuxDemotesDataSharedWithASystemEntry(t *testing.T) {
	f := newLinuxFixture(t, func(tree *memTree) {
		tree.dir("/usr/share/applications", 0)
		tree.file("/usr/share/applications/fakeapp.desktop", 0, 0o644, []byte("[Desktop Entry]\nName=Other\nExec=other\n"))
	})
	d := f.u.Discover(linuxDesktop)
	if !d.Ready {
		t.Fatalf("discovery unavailable: %+v", d)
	}
	data := candidateAt(t, d, linuxData)
	if data.SelectedByDefault {
		t.Fatal("data shared with a system entry was selected by default")
	}
}

// A user flatpak adds its install folder and its app data, both keyed by the exact identifier.
func TestLinuxFlatpakUserInstallAddsInstallAndAppData(t *testing.T) {
	const id = "org.example.Flat"
	entry := path.Join(linuxHome, ".local", "share", "flatpak", "exports", "share", "applications", id+".desktop")
	install := path.Join(linuxHome, ".local", "share", "flatpak", "app", id)
	appData := path.Join(linuxHome, ".var", "app", id)
	f := newLinuxFixture(t, func(tree *memTree) {
		tree.file(entry, linuxUID, 0o644, []byte("[Desktop Entry]\nName=Flat\nExec=flat\n"))
		tree.dir(install, linuxUID)
		tree.dir(appData, linuxUID)
	})
	d := f.u.Discover(entry)
	if !d.Ready {
		t.Fatalf("discovery unavailable: %+v", d)
	}
	for _, p := range []string{install, appData} {
		if !hasCandidate(d, p) {
			t.Fatalf("flatpak item %s not listed; have %+v", p, d.Snapshot.Candidates)
		}
	}
}

// Hostile input: an identifier with spaces or slashes cannot name a folder, so the entry is refused.
func TestLinuxRefusesUnsafeIdentifiers(t *testing.T) {
	bad := path.Join(linuxHome, ".local", "share", "applications", "bad id.desktop")
	f := newLinuxFixture(t, func(tree *memTree) {
		tree.file(bad, linuxUID, 0o644, []byte("[Desktop Entry]\nName=Bad\nExec=bad\n"))
	})
	d := f.u.Discover(bad)
	if d.Ready || d.ReasonCode != "bundle-id" {
		t.Fatalf("discovery = %+v, want bundle-id refusal", d)
	}
}

// Hostile input: a desktop entry that is a link is refused before anything under it is listed.
func TestLinuxRefusesLinkedDesktopEntry(t *testing.T) {
	link := path.Join(linuxHome, ".local", "share", "applications", "linked.desktop")
	f := newLinuxFixture(t, func(tree *memTree) { tree.link(link, linuxUID) })
	if d := f.u.Discover(link); d.Ready {
		t.Fatalf("linked entry discovered: %+v", d)
	}
}

// Hostile input: between review and Trash the data folder is replaced by a link. The move is refused and nothing
// under the link is touched.
func TestLinuxRefusesDataReplacedByALinkAfterReview(t *testing.T) {
	f := newLinuxFixture(t, nil)
	d := f.u.Discover(linuxDesktop)
	f.tree.remove(linuxData)
	f.tree.link(linuxData, linuxUID)
	result := f.u.Trash(d.Snapshot, selectIDs(d.Snapshot.Candidates...))
	for _, moved := range f.tree.trashed {
		if moved == linuxData {
			t.Fatal("a link was trashed in place of the reviewed folder")
		}
	}
	if len(result.Untouched) == 0 || result.Untouched[0].Code != codeSymlink {
		t.Fatalf("untouched = %+v, want a symlink refusal", result.Untouched)
	}
}

// The discover error for a missing entry is the ordinary missing refusal, with no listing performed.
func TestLinuxMissingEntryIsUnavailable(t *testing.T) {
	f := newLinuxFixture(t, nil)
	d := f.u.Discover(path.Join(linuxHome, ".local", "share", "applications", "gone.desktop"))
	if d.Ready || d.ReasonCode != codeMissing {
		t.Fatalf("discovery = %+v", d)
	}
}
