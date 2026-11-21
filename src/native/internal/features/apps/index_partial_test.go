package apps

import (
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func appNames(list []App) []string {
	names := []string{}
	for _, app := range list {
		names = append(names, app.Name)
	}
	return names
}

// An incomplete scan only adds what it found: an app it could not see is not proof of an uninstall, so a usable list
// is never erased by one unreadable location. A complete scan is authoritative and removes what it no longer lists.
func TestIncompleteScanNeverReplacesAGoodIndex(t *testing.T) {
	loom := App{Name: "Loom", Path: "/Applications/Loom.app"}
	notes := App{Name: "Notes", Path: "/Applications/Notes.app"}
	added := App{Name: "Zed", Path: "/Applications/Zed.app"}
	scanner := &scriptedScanner{results: []scriptedResult{
		{apps: []App{loom, notes}, complete: true},
		{apps: []App{added}, complete: false},
		{apps: []App{notes, added}, complete: true},
	}}
	cache := &MemoryCache{}
	_ = cache.Save(Snapshot{Version: SnapshotVersion, FirstSeen: FirstSeen{ByID: map[string]int64{}, Initialized: true}})
	index := NewIndex(IndexOptions{Scan: scanner.scan, Cache: cache, Clock: fake.NewClock()})
	mustLoad(t, index)
	for i, want := range [][]string{{"Loom", "Notes"}, {"Loom", "Notes", "Zed"}, {"Notes", "Zed"}} {
		if err := index.Scan(); err != nil {
			t.Fatal(err)
		}
		if got := appNames(index.Apps()); !reflect.DeepEqual(got, want) {
			t.Fatalf("after scan %d apps = %v, want %v", i+1, got, want)
		}
	}
	saved, _ := cache.Load()
	if got := appNames(saved.Apps); !reflect.DeepEqual(got, []string{"Notes", "Zed"}) {
		t.Fatalf("saved apps = %v", got)
	}
}

// A partial scan that lands before the saved list loads keeps its discoveries and gains the saved list; a complete scan
// that lands first is not overwritten by the older saved list.
func TestSavedListMergesUnderPartialScanButNotOverCompleteScan(t *testing.T) {
	saved := Snapshot{Version: SnapshotVersion, Apps: []App{{Name: "Loom", Path: "/Applications/Loom.app"}}, FirstSeen: FirstSeen{ByID: map[string]int64{}, Initialized: true}}
	for _, complete := range []bool{false, true} {
		cache := &MemoryCache{}
		_ = cache.Save(saved)
		scanner := &scriptedScanner{results: []scriptedResult{{apps: []App{{Name: "Zed", Path: "/Applications/Zed.app"}}, complete: complete}}}
		index := NewIndex(IndexOptions{Scan: scanner.scan, Cache: cache, Clock: fake.NewClock()})
		if err := index.Scan(); err != nil {
			t.Fatal(err)
		}
		_ = cache.Save(saved)
		mustLoad(t, index)
		want := []string{"Loom", "Zed"}
		if complete {
			want = []string{"Zed"}
		}
		if got := appNames(index.Apps()); !reflect.DeepEqual(got, want) {
			t.Fatalf("complete=%v apps = %v, want %v", complete, got, want)
		}
	}
}
