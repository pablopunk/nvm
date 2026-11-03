package apps

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Ported from app-index-service.test.ts. The 1.x tests passed callbacks for each dependency; the Go tests
// assert the same sequence through IndexHooks. Scans are synchronous unless the test says otherwise.

func lowerIdentity(path string) string { return strings.ToLower(path) }

// mustLoad loads the persisted state the tests seed: an initialized baseline, so no baseline scan runs unless the
// test is about the baseline.
func mustLoad(t *testing.T, index *Index) {
	t.Helper()
	if err := index.LoadCache(); err != nil {
		t.Fatal(err)
	}
}

// scriptedScanner returns the scripted results in order; a result with err set fails that scan. Scans past
// the script repeat the last result.
type scriptedScanner struct {
	mu      sync.Mutex
	results []scriptedResult
	calls   int
}

type scriptedResult struct {
	apps     []App
	complete bool
	err      error
}

func (s *scriptedScanner) scan() (ScanResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.calls
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	s.calls++
	r := s.results[i]
	if r.err != nil {
		return ScanResult{}, r.err
	}
	return ScanResult{Apps: append([]App(nil), r.apps...), Complete: r.complete}, nil
}

func (s *scriptedScanner) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// Ported: "app index service indexes apps and notifies dependent running-app status".
func TestIndexIndexesAppsAndNotifiesRunningStatus(t *testing.T) {
	var notes []string
	scanner := &scriptedScanner{results: []scriptedResult{{apps: []App{
		{Name: "B"},
		{Name: "A"},
		{Name: "a", Path: "/latest/A.app"},
	}, complete: true}}}
	cache := &MemoryCache{}
	_ = cache.Save(Snapshot{Version: SnapshotVersion, FirstSeen: FirstSeen{ByID: map[string]int64{}, Initialized: true}})
	index := NewIndex(IndexOptions{Scan: scanner.scan, Cache: cache, Clock: fake.NewClock(), Hooks: IndexHooks{
		InvalidateRunning:      func() { notes = append(notes, "invalidate-running") },
		Indexed:                func(count int) { notes = append(notes, "indexed:"+strconv.Itoa(count)) },
		ScheduleRunningRefresh: func(reason string) { notes = append(notes, "refresh:"+reason) },
	}})
	mustLoad(t, index)
	if err := index.Scan(); err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, app := range index.Apps() {
		names = append(names, app.Name)
	}
	if !reflect.DeepEqual(names, []string{"a", "B"}) {
		t.Fatalf("names = %v, want [a B]", names)
	}
	want := []string{"invalidate-running", "indexed:2", "refresh:apps-indexed"}
	if !reflect.DeepEqual(notes, want) {
		t.Fatalf("notifications = %v, want %v", notes, want)
	}
}

// Ported: "app index service waits for a complete baseline scan". Scan 1 and 2 are incomplete, 3 and 4 complete.
func TestIndexWaitsForCompleteBaselineScan(t *testing.T) {
	existing := App{Name: "Existing", Path: "/Applications/Existing.app"}
	recovered := App{Name: "Recovered", Path: "/Applications/Recovered.app"}
	scanner := &scriptedScanner{results: []scriptedResult{
		{apps: []App{existing}, complete: false},
		{apps: []App{existing}, complete: false},
		{apps: []App{existing, recovered}, complete: true},
		{apps: []App{existing, recovered}, complete: true},
	}}
	index := NewIndex(IndexOptions{Scan: scanner.scan, Clock: fake.NewClock(), Identity: lowerIdentity})

	if err := index.Scan(); err != nil {
		t.Fatal(err)
	}
	if scanner.callCount() != 2 {
		t.Fatalf("after first Scan calls = %d, want 2", scanner.callCount())
	}
	if index.firstSeen.Initialized {
		t.Fatal("baseline confirmed by incomplete scans")
	}

	if err := index.Scan(); err != nil {
		t.Fatal(err)
	}
	if scanner.callCount() != 4 {
		t.Fatalf("after second Scan calls = %d, want 4", scanner.callCount())
	}
	if !index.firstSeen.Initialized {
		t.Fatal("baseline not confirmed by complete scans")
	}
	for _, app := range index.Apps() {
		if app.FirstSeenAt != 0 {
			t.Fatalf("%s FirstSeenAt = %d, want 0 (baseline)", app.Name, app.FirstSeenAt)
		}
	}
}

// Ported: "app index service keeps previous index when scanning fails".
func TestIndexKeepsPreviousIndexWhenScanFails(t *testing.T) {
	var errs []error
	scanner := &scriptedScanner{results: []scriptedResult{
		{apps: []App{{Name: "Notes"}}, complete: true},
		{err: errors.New("boom")},
	}}
	cache := &MemoryCache{}
	_ = cache.Save(Snapshot{Version: SnapshotVersion, FirstSeen: FirstSeen{ByID: map[string]int64{}, Initialized: true}})
	index := NewIndex(IndexOptions{Scan: scanner.scan, Cache: cache, Clock: fake.NewClock(), Hooks: IndexHooks{
		Error: func(err error) { errs = append(errs, err) },
	}})
	mustLoad(t, index)
	if err := index.Scan(); err != nil {
		t.Fatal(err)
	}
	if err := index.Scan(); err == nil {
		t.Fatal("second Scan returned nil, want the scan error")
	}
	apps := index.Apps()
	if len(apps) != 1 || apps[0].Name != "Notes" {
		t.Fatalf("apps = %+v, want [Notes]", apps)
	}
	if len(errs) != 1 || errs[0].Error() != "boom" {
		t.Fatalf("errors = %v, want [boom]", errs)
	}
}

// Ported in spirit from "app index service replaces watchers and emits debounced host change events": the 1.x
// test counted watcher replacements; the Go index has no watchers, so its contract is the debounce. Repeated
// requests inside the window are dropped; a request after it scans again.
func TestIndexRefreshRequestsAreDebounced(t *testing.T) {
	clock := fake.NewClock()
	scanner := &scriptedScanner{results: []scriptedResult{{apps: []App{{Name: "Notes", Path: "/Applications/Notes.app"}}, complete: true}}}
	cache := &MemoryCache{}
	_ = cache.Save(Snapshot{Version: SnapshotVersion, FirstSeen: FirstSeen{ByID: map[string]int64{}, Initialized: true}})
	index := NewIndex(IndexOptions{Scan: scanner.scan, Cache: cache, Clock: clock})
	mustLoad(t, index)

	index.RequestRefresh()
	index.WaitIdle()
	index.RequestRefresh()
	index.RequestRefresh()
	index.WaitIdle()
	if scanner.callCount() != 1 {
		t.Fatalf("scans inside the debounce window = %d, want 1", scanner.callCount())
	}
	clock.Advance(RefreshDebounce + time.Millisecond)
	index.RequestRefresh()
	index.WaitIdle()
	if scanner.callCount() != 2 {
		t.Fatalf("scans after the window = %d, want 2", scanner.callCount())
	}
}

// Ported: "dedupeAndSortApps keeps one app per normalized name and sorts by display name".
func TestDedupeAndSortKeepsOneAppPerNameAndSortsByName(t *testing.T) {
	apps := DedupeAndSort([]App{
		{Name: "Safari", Path: "/A/Safari.app"},
		{Name: "Notes", Path: "/A/Notes.app"},
		{Name: "safari", Path: "/B/Safari.app"},
	})
	var got []string
	for _, app := range apps {
		got = append(got, app.Name+":"+app.Path)
	}
	want := []string{"Notes:/A/Notes.app", "safari:/B/Safari.app"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DedupeAndSort = %v, want %v", got, want)
	}
}

// Hostile input beyond 1.x: apps with identical names from two install roots collapse to one row, and the
// row keeps the path of the copy listed last, so an uninstall acts on exactly the path that was shown.
func TestDedupeAndSortIdenticalNamesKeepOneRowWithItsPath(t *testing.T) {
	apps := DedupeAndSort([]App{
		{Name: "Example", Path: "/Applications/Example.app"},
		{Name: "Example ", Path: "/Users/tester/Applications/Example.app"},
	})
	if len(apps) != 1 || apps[0].Path != "/Users/tester/Applications/Example.app" {
		t.Fatalf("apps = %+v, want one row for the last copy", apps)
	}
}
