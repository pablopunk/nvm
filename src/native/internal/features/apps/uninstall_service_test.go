package apps

import (
	"errors"
	"path"
	"reflect"
	"strings"
	"testing"
)

// Ported from app-uninstall-service.test.ts (the darwin cases). Each test names the 1.x case it ports. The 1.x
// fixture used a fake lstat/realpath; this one uses memTree, which never follows links, so the "escaped by realpath"
// cases are ported as "a link on the path is refused", the rule that makes the escape impossible here.

const (
	testHome   = "/Users/tester"
	testUID    = 501
	testApp    = "/Applications/Example.app"
	testBundle = "com.example.App"
)

var (
	testCache       = path.Join(testHome, "Library", "Caches", testBundle)
	testPreferences = path.Join(testHome, "Library", "Preferences", testBundle+".plist")
	testAppSupport  = path.Join(testHome, "Library", "Application Support")
)

type otherApp struct {
	bundleID   string
	unreadable bool
}

type fixtureOpts struct {
	running           bool
	quitStuck         bool
	runningCheckFails bool
	symlink           string
	cacheOwner        int
	otherApps         map[string]otherApp
	entries           map[string][]string
	findApps          func(bundleID string) ([]string, error)
	bundleIDError     error
	nevermindBundle   string
	failTrashAt       string
}

type darwinFixture struct {
	u         *Uninstaller
	tree      *memTree
	running   bool
	quitCalls []string
	bundleID  string
	opts      fixtureOpts
}

func newDarwinFixture(t *testing.T, opts fixtureOpts) *darwinFixture {
	t.Helper()
	tree := newMemTree()
	tree.dir("/Applications", testUID)
	tree.dir(path.Join(testHome, "Library"), testUID)
	tree.dir(testApp, testUID)
	cacheOwner := testUID
	if opts.cacheOwner != 0 {
		cacheOwner = opts.cacheOwner
	}
	tree.dir(testCache, cacheOwner)
	tree.file(testPreferences, testUID, 0o644, nil)
	if opts.symlink != "" {
		tree.remove(opts.symlink)
		tree.link(opts.symlink, testUID)
	}
	for otherPath := range opts.otherApps {
		tree.dir(otherPath, testUID)
	}
	for folder, names := range opts.entries {
		for _, name := range names {
			tree.dir(path.Join(folder, name), testUID)
		}
	}
	f := &darwinFixture{tree: tree, running: opts.running, bundleID: testBundle, opts: opts}
	nevermindBundle := opts.nevermindBundle
	if nevermindBundle == "" {
		nevermindBundle = "com.nevermind.app"
	}
	f.u = NewUninstaller(UninstallDeps{
		Platform:          "darwin",
		Home:              testHome,
		UID:               testUID,
		Files:             tree,
		DirectoryAccess:   fakeDirectoryAccess{tree: tree},
		Trash:             tree.trash,
		ReadBundleID:      f.readBundleID,
		FindApps:          opts.findApps,
		IsRunning:         f.isRunning,
		RequestQuit:       f.requestQuit,
		NevermindAppPath:  "/Applications/Nevermind.app",
		NevermindBundleID: nevermindBundle,
	})
	tree.failTrashAt = opts.failTrashAt
	return f
}

func (f *darwinFixture) readBundleID(p string) (string, error) {
	if p == testApp || strings.HasSuffix(p, "/Example.app") {
		if f.opts.bundleIDError != nil {
			return "", f.opts.bundleIDError
		}
		return f.bundleID, nil
	}
	other, ok := f.opts.otherApps[p]
	if !ok {
		return "", errors.New("unknown app " + p)
	}
	if other.unreadable {
		return "", errors.New("unreadable")
	}
	return other.bundleID, nil
}

func (f *darwinFixture) isRunning(string) (bool, error) {
	if f.opts.runningCheckFails {
		return false, errors.New("process inspection failed")
	}
	return f.running, nil
}

func (f *darwinFixture) requestQuit(p string) error {
	f.quitCalls = append(f.quitCalls, "quit:"+p)
	if !f.opts.quitStuck {
		f.running = false
	}
	return nil
}

// discover runs discovery and fails the test unless the app is ready.
func (f *darwinFixture) discover(t *testing.T) Discovery {
	t.Helper()
	d := f.u.Discover(testApp)
	if !d.Ready {
		t.Fatalf("discovery unavailable: %s (%s)", d.Message, d.ReasonCode)
	}
	return d
}

func candidateAt(t *testing.T, d Discovery, p string) Candidate {
	t.Helper()
	for _, c := range d.Snapshot.Candidates {
		if c.Path == p {
			return c
		}
	}
	t.Fatalf("expected candidate %s; have %+v", p, d.Snapshot.Candidates)
	return Candidate{}
}

func hasCandidate(d Discovery, p string) bool {
	for _, c := range d.Snapshot.Candidates {
		if c.Path == p {
			return true
		}
	}
	return false
}

func selectIDs(list ...Candidate) map[string]bool {
	out := map[string]bool{}
	for _, c := range list {
		out[c.ID] = true
	}
	return out
}

// Ported: "production plist reader ... " is in uninstall_plist_test.go. "unsupported missing-top-level-plist layouts
// produce concise actionable text" and "redacts process failures when app metadata cannot be read" are below.

// Ported: "unsupported missing-top-level-plist layouts produce concise actionable text".
func TestDiscoverReportsUnreadableMetadataConcisely(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{bundleIDError: errors.New("unsupported layout")})
	d := f.u.Discover(testApp)
	if d.Ready || d.ReasonCode != "plist" {
		t.Fatalf("discovery = %+v, want unavailable plist", d)
	}
	if d.Message != "This app’s metadata could not be read" || strings.Contains(d.Message, "plutil") {
		t.Fatalf("message = %q", d.Message)
	}
}

// Ported: "redacts process failures when app metadata cannot be read".
func TestDiscoverRedactsProcessFailures(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{bundleIDError: errors.New("Command failed: /usr/bin/plutil -extract CFBundleIdentifier")})
	d := f.u.Discover(testApp)
	if d.Ready || d.ReasonCode != "plist" || d.Message != "This app’s metadata could not be read" {
		t.Fatalf("discovery = %+v", d)
	}
	if strings.Contains(d.Message, "plutil") || strings.Contains(d.Message, "/usr/bin") {
		t.Fatalf("message leaks the command: %q", d.Message)
	}
}

// Ported: "non-macOS discovery performs no host work and the trusted production ID rejects another Nevermind copy"
// (the Windows half: no host work at all). The Linux half is covered by the Linux discovery tests.
func TestDiscoverOnUnsupportedPlatformDoesNoHostWork(t *testing.T) {
	calls := 0
	u := NewUninstaller(UninstallDeps{
		Platform: "windows",
		Home:     testHome,
		UID:      testUID,
		Files:    countingFiles{calls: &calls},
		Trash:    func(string) error { calls++; return nil },
		IsRunning: func(string) (bool, error) {
			calls++
			return false, nil
		},
	})
	if d := u.Discover(testApp); d.Ready || d.ReasonCode != "unsupported-platform" {
		t.Fatalf("discovery = %+v", d)
	}
	if calls != 0 {
		t.Fatalf("host calls = %d, want 0", calls)
	}
}

// countingFiles fails the test's expectations by counting every call it receives.
type countingFiles struct{ calls *int }

func (c countingFiles) Lstat(string) (Entry, error) {
	*c.calls++
	return Entry{}, errors.New("must not run")
}
func (c countingFiles) ReadDir(string) ([]DirEntry, error) { *c.calls++; return nil, nil }
func (c countingFiles) Tree(string, int) (int, error)      { *c.calls++; return 0, nil }
func (c countingFiles) ReadFile(string) ([]byte, error)    { *c.calls++; return nil, nil }

// Ported: "non-macOS discovery ... the trusted production ID rejects another Nevermind copy" (the self half).
func TestDiscoverRefusesAnotherNevermindCopy(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{nevermindBundle: NeverMindBundleID})
	f.bundleID = NeverMindBundleID
	d := f.u.Discover(testApp)
	if d.Ready || d.ReasonCode != "self" {
		t.Fatalf("discovery = %+v, want self refusal", d)
	}
}

// Ported: "selects exact bundle-ID data when no conflicting app is found".
func TestDiscoverSelectsExactBundleIDData(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	d := f.discover(t)
	var paths []string
	for _, c := range d.Snapshot.Candidates {
		paths = append(paths, c.Path)
	}
	if want := []string{testApp, testCache, testPreferences}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("candidates = %v, want %v", paths, want)
	}
	if len(d.Notes) == 0 || d.Notes[0].Code != "missing-associated" {
		t.Fatalf("notes = %+v, want missing-associated first", d.Notes)
	}
	if d.AppRunning {
		t.Fatal("AppRunning = true for a stopped app")
	}
	var defaults []string
	for _, c := range d.Snapshot.Candidates {
		if c.SelectedByDefault {
			defaults = append(defaults, c.Path)
		}
	}
	if want := []string{testApp, testCache, testPreferences}; !reflect.DeepEqual(defaults, want) {
		t.Fatalf("selected by default = %v, want %v", defaults, want)
	}
	selected := Selected(d.Snapshot, map[string]bool{d.Snapshot.Candidates[0].ID: true})
	if len(selected) != 1 || selected[0].Path != testApp {
		t.Fatalf("Selected = %+v, want the app only", selected)
	}
}

// Ported: "discovery stays available while the app runs and reports the running state on the app item".
func TestDiscoverStaysAvailableWhileAppRuns(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{running: true})
	d := f.discover(t)
	if !d.AppRunning {
		t.Fatal("AppRunning = false for a running app")
	}
	app := candidateAt(t, d, testApp)
	if !app.SelectedByDefault || !strings.Contains(app.Reason, "quit gracefully") {
		t.Fatalf("app candidate = %+v", app)
	}
}

// Ported: "rejects symlinks and ownership mismatches without treating them as deletable candidates". The root-symlink
// and realpath-escape variants are covered by the same rule: a link anywhere on the path refuses the item.
func TestDiscoverRejectsLinksAndForeignOwnership(t *testing.T) {
	if d := newDarwinFixture(t, fixtureOpts{symlink: testCache}).discover(t); hasCandidate(d, testCache) {
		t.Fatal("symlinked cache data was offered")
	}
	if d := newDarwinFixture(t, fixtureOpts{cacheOwner: 502}).discover(t); hasCandidate(d, testCache) {
		t.Fatal("cache data owned by another user was offered")
	}
	rootLink := newDarwinFixture(t, fixtureOpts{})
	rootLink.tree.remove(path.Join(testHome, "Library", "Caches"))
	rootLink.tree.link(path.Join(testHome, "Library", "Caches"), testUID)
	if d := rootLink.discover(t); hasCandidate(d, testCache) {
		t.Fatal("cache data under a linked Caches folder was offered")
	}
	escaped := newDarwinFixture(t, fixtureOpts{})
	escaped.tree.remove(testCache)
	escaped.tree.link(testCache, testUID)
	if d := escaped.discover(t); hasCandidate(d, testCache) {
		t.Fatal("a cache link pointing elsewhere was offered")
	}
}
