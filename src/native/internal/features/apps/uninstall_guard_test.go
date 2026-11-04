package apps

import (
	"math/rand"
	"path"
	"strings"
	"testing"
)

// Hostile-path tests for the uninstall guards. The 1.x suite had none of these; the cases below are the ones the
// plan names (symlinks, "..", trailing separators, case variations, prefix identifiers, read-only items, the item cap,
// and the time-of-check to time-of-use gap), plus a property test over random paths.

// A symlink named like an app, pointing at the root of the file system, is refused as a link before anything is read.
func TestGuardRefusesSymlinkToRootAsApp(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	f.tree.link("/Applications/Evil.app", testUID)
	d := f.u.Discover("/Applications/Evil.app")
	if d.Ready || d.ReasonCode != codeSymlink {
		t.Fatalf("discovery = %+v, want symlink refusal", d)
	}
}

// A ".." segment is refused before any path is built, even when it would resolve inside an allowed root.
func TestGuardRefusesDotDotSegments(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	for _, p := range []string{
		"/Applications/../Applications/Example.app",
		"/Applications/Example.app/../Example.app",
		"/Applications/..",
	} {
		if d := f.u.Discover(p); d.Ready || d.ReasonCode != codeInvalidPath {
			t.Errorf("Discover(%q) = %+v, want invalid-path", p, d)
		}
	}
}

// A trailing separator names the same item: the cleaned path is what the review shows and what Trash moves.
func TestGuardCleansTrailingSeparators(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	d := f.discover(t)
	again := f.u.Discover(testApp + "/")
	if !again.Ready || again.Snapshot.AppPath != testApp {
		t.Fatalf("trailing separator discovery = %+v", again)
	}
	if d.Snapshot.AppPath != testApp {
		t.Fatalf("app path = %q", d.Snapshot.AppPath)
	}
}

// Case is significant to the guards: a differently-cased spelling names a different path, which does not exist on
// this tree, so the request is refused rather than matched to the real bundle.
func TestGuardDoesNotMatchCaseVariants(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	if d := f.u.Discover("/applications/example.app"); d.Ready {
		t.Fatalf("case variant discovered: %+v", d)
	}
}

// A candidate whose path contains ".." (a snapshot that did not come from discovery) is refused at revalidation.
func TestGuardRefusesDotDotInCandidatePaths(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	d := f.discover(t)
	cache := candidateAt(t, d, testCache)
	// Built by concatenation: path.Join would clean the ".." away and test nothing.
	cache.Path = testHome + "/Library/Caches/../Caches/" + testBundle
	forged := d.Snapshot
	forged.Candidates = []Candidate{cache}
	result := f.u.Trash(forged, selectIDs(cache))
	if result.Status != StatusFailed || len(f.tree.trashed) != 0 {
		t.Fatalf("forged path moved: %+v trashed %v", result, f.tree.trashed)
	}
}

// A read-only item inside a writable folder can still be moved: the move renames it out of the folder, and the
// folder's permissions govern that. Trash therefore completes.
func TestGuardMovesReadOnlyItemsInsideWritableFolders(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	f.tree.file(path.Join(testCache, "locked.db"), testUID, 0o400, []byte("x"))
	d := f.discover(t)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testCache)))
	if result.Status != StatusComplete {
		t.Fatalf("read-only item: %+v", result)
	}
}

// An app inside a folder the user cannot write to cannot be moved, so it is refused with that reason.
func TestGuardRefusesAppInReadOnlyFolder(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	f.tree.remove("/Applications")
	f.tree.dir("/Applications", 0)
	f.tree.dir(testApp, testUID)
	d := f.u.Discover(testApp)
	if d.Ready || d.ReasonCode != "app-parent-not-writable" {
		t.Fatalf("discovery = %+v, want the parent refusal", d)
	}
}

// The item cap: a folder with more than MaxItems entries is refused with a note, and is not offered.
func TestGuardRefusesFoldersOverTheItemCap(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	for i := 0; i < MaxItems+1; i++ {
		f.tree.file(path.Join(testCache, "f"+itoa(i)), testUID, 0o644, nil)
	}
	d := f.discover(t)
	if hasCandidate(d, testCache) {
		t.Fatal("an oversized folder was offered")
	}
	found := false
	for _, n := range d.Notes {
		if strings.HasSuffix(n.Code, codeTooLarge) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no too-large note: %+v", d.Notes)
	}
}

// Time of check to time of use: a data folder replaced by a link after review is refused at its own move, and the
// link's target is never touched.
func TestGuardRefusesLinkSwappedInAfterReview(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	d := f.discover(t)
	f.tree.remove(testCache)
	f.tree.link(testCache, testUID)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testCache)))
	for _, moved := range f.tree.trashed {
		if moved == testCache {
			t.Fatal("the swapped-in link was moved")
		}
	}
	if len(result.Untouched) != 1 || result.Untouched[0].Code != codeSymlink {
		t.Fatalf("untouched = %+v, want symlink", result.Untouched)
	}
}

// Identifiers that are prefixes of each other: "com.example.App" is not related to "com.example.AppTwo", but is
// related to "com.example.App.Helper". Only the related one demotes the data.
func TestGuardPrefixIdentifiersOnlyOverlapOnDotBoundaries(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{otherApps: map[string]otherApp{
		"/Applications/AppTwo.app": {bundleID: "com.example.AppTwo"},
	}})
	if !candidateAt(t, f.discover(t), testCache).SelectedByDefault {
		t.Fatal("com.example.AppTwo demoted com.example.App")
	}
	g := newDarwinFixture(t, fixtureOpts{otherApps: map[string]otherApp{
		"/Applications/Helper.app": {bundleID: "com.example.App.Helper"},
	}})
	if candidateAt(t, g.discover(t), testCache).SelectedByDefault {
		t.Fatal("com.example.App.Helper did not demote com.example.App")
	}
}

// Identical names from two installs: the index keeps one row; discovery of either path works on its own bundle and
// lists only its own data.
func TestGuardIdenticalNamesStayIndependent(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	userCopy := path.Join(testHome, "Applications", "Example.app")
	f.tree.dir(userCopy, testUID)
	d := f.u.Discover(userCopy)
	if !d.Ready || d.Snapshot.AppPath != userCopy {
		t.Fatalf("user copy discovery = %+v", d)
	}
	for _, c := range d.Snapshot.Candidates {
		if c.Path == testApp {
			t.Fatal("the other install's app was listed")
		}
	}
}

// Property: random hostile paths never produce an available discovery unless the path is a direct child of an
// application folder that is not a link; and every listed item stays inside its root, is not a link, and belongs to
// the user. The generator mixes real and hostile segments.
func TestGuardPropertyRandomPathsNeverEscape(t *testing.T) {
	segments := []string{"Applications", "Library", "Caches", "..", ".", "", "Example.app", testBundle, "Preferences", "x\x00y", "Evil.app", "Users", testHome[1:]}
	rng := rand.New(rand.NewSource(20260501))
	f := newDarwinFixture(t, fixtureOpts{})
	f.tree.link("/Applications/Evil.app", testUID)
	f.tree.dir("/Users/evil", 0)
	for i := 0; i < 2000; i++ {
		var parts []string
		for n := rng.Intn(6); n >= 0; n-- {
			parts = append(parts, segments[rng.Intn(len(segments))])
		}
		candidate := "/" + strings.Join(parts, "/")
		if rng.Intn(4) == 0 {
			candidate = testApp
		}
		d := f.u.Discover(candidate)
		if !d.Ready {
			continue
		}
		app := d.Snapshot.AppPath
		if path.Dir(app) != "/Applications" && path.Dir(app) != testHome+"/Applications" {
			t.Fatalf("path %q discovered outside the application folders as %q", candidate, app)
		}
		for _, c := range d.Snapshot.Candidates {
			if c.Kind == KindApp {
				continue
			}
			entry, ref := f.u.inspectAllowed(c.Path, c.Root)
			if ref != nil {
				t.Fatalf("candidate %q of %q is not inside its root: %+v", c.Path, candidate, ref)
			}
			if entry.Identity.UID != testUID || entry.Symlink {
				t.Fatalf("candidate %q is foreign or a link: %+v", c.Path, entry)
			}
		}
	}
}

// itoa formats a small non-negative integer for fixture names.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
