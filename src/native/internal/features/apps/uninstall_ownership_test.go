package apps

import (
	"errors"
	"path"
	"strings"
	"testing"
)

// Ownership and pattern cases of the darwin discovery, ported from app-uninstall-service.test.ts. The fixture is
// the one in uninstall_service_test.go.

// Ported: "demotes exact data when another installed app shares or overlaps the bundle ID".
func TestDiscoverDemotesSharedBundleIDData(t *testing.T) {
	shared := newDarwinFixture(t, fixtureOpts{otherApps: map[string]otherApp{
		"/Applications/Helper.app": {bundleID: "com.example.App.Helper"},
	}})
	d := shared.discover(t)
	cache := candidateAt(t, d, testCache)
	if cache.SelectedByDefault || !strings.Contains(cache.Reason, "May be shared with Helper") {
		t.Fatalf("shared cache = %+v", cache)
	}
	if !candidateAt(t, d, testApp).SelectedByDefault {
		t.Fatal("the app itself must stay selected by default")
	}

	unrelated := newDarwinFixture(t, fixtureOpts{otherApps: map[string]otherApp{
		"/Applications/Appetite.app": {bundleID: "com.example.Appetite"},
		"/Applications/Tool.app":     {bundleID: "com.other.Tool"},
	}})
	if !candidateAt(t, unrelated.discover(t), testCache).SelectedByDefault {
		t.Fatal("unrelated neighbours demoted the cache data")
	}
}

// Ported: "demotes exact data when another installed app cannot be read to rule out ownership".
func TestDiscoverDemotesDataWhenAnotherAppIsUnreadable(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{otherApps: map[string]otherApp{"/Applications/Broken.app": {unreadable: true}}})
	cache := candidateAt(t, f.discover(t), testCache)
	if cache.SelectedByDefault || cache.Reason != "Other installed apps could not be ruled out; left unchecked" {
		t.Fatalf("cache = %+v", cache)
	}
}

// Ported: "uses the application index to find nested conflicting apps and handles incomplete results".
func TestDiscoverUsesSpotlightIndexForOwnership(t *testing.T) {
	nested := newDarwinFixture(t, fixtureOpts{findApps: func(string) ([]string, error) {
		return []string{testApp, "/Applications/Vendor/Helper.app"}, nil
	}})
	if candidateAt(t, nested.discover(t), testCache).SelectedByDefault {
		t.Fatal("nested conflicting app did not demote the cache data")
	}
	incomplete := newDarwinFixture(t, fixtureOpts{findApps: func(string) ([]string, error) { return nil, nil }})
	if candidateAt(t, incomplete.discover(t), testCache).SelectedByDefault {
		t.Fatal("an incomplete index selected the cache data")
	}
	failing := newDarwinFixture(t, fixtureOpts{findApps: func(string) ([]string, error) { return nil, errors.New("index unavailable") }})
	if candidateAt(t, failing.discover(t), testCache).SelectedByDefault {
		t.Fatal("a failing index selected the cache data")
	}
	unique := newDarwinFixture(t, fixtureOpts{findApps: func(string) ([]string, error) { return []string{testApp}, nil }})
	if !candidateAt(t, unique.discover(t), testCache).SelectedByDefault {
		t.Fatal("a verified unique owner did not select the cache data")
	}
}

// Ported: "lists bounded prefix and name matches unchecked with reasons and never includes group containers".
func TestDiscoverListsPatternMatchesUncheckedAndNeverGroupContainers(t *testing.T) {
	lib := path.Join(testHome, "Library")
	f := newDarwinFixture(t, fixtureOpts{entries: map[string][]string{
		path.Join(lib, "Application Support"): {"com.example.App.Helper", "Example", "com.example.Other", "com.example.AppOther"},
		path.Join(lib, "Caches"):              {"com.example.App.ShipIt"},
		path.Join(lib, "Group Containers"):    {"group." + testBundle},
	}})
	d := f.discover(t)
	helper := path.Join(testAppSupport, "com.example.App.Helper")
	name := path.Join(testAppSupport, "Example")
	shipIt := path.Join(lib, "Caches", "com.example.App.ShipIt")
	for _, p := range []string{helper, name, shipIt} {
		c := candidateAt(t, d, p)
		if c.Kind != KindAssociated || c.SelectedByDefault {
			t.Fatalf("pattern candidate %s = %+v, want associated and unchecked", p, c)
		}
	}
	if !strings.Contains(candidateAt(t, d, helper).Reason, "prefix match") || !strings.Contains(candidateAt(t, d, name).Reason, "Name match") {
		t.Fatal("pattern reasons are not named")
	}
	for _, c := range d.Snapshot.Candidates {
		if strings.Contains(c.Path, "Group Containers") || strings.HasSuffix(c.Path, "com.example.Other") || strings.HasSuffix(c.Path, "com.example.AppOther") {
			t.Fatalf("unexpected candidate %s", c.Path)
		}
	}
}

// Ported: "caps pattern matches and reports the limit".
func TestDiscoverCapsPatternMatches(t *testing.T) {
	var names []string
	for i := 0; i < 25; i++ {
		names = append(names, "com.example.App.Helper"+strings.Repeat("x", i))
	}
	f := newDarwinFixture(t, fixtureOpts{entries: map[string][]string{testAppSupport: names}})
	d := f.discover(t)
	count := 0
	for _, c := range d.Snapshot.Candidates {
		if strings.HasPrefix(c.Path, testAppSupport) {
			count++
		}
	}
	if count != MaxPatternMatches {
		t.Fatalf("pattern candidates = %d, want %d", count, MaxPatternMatches)
	}
	found := false
	for _, n := range d.Notes {
		if n.Code == "pattern-limit" {
			found = true
		}
	}
	if !found {
		t.Fatal("no pattern-limit note")
	}
}
