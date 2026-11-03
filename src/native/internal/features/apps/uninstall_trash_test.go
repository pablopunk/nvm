package apps

import (
	"reflect"
	"testing"
)

// Trash runs of the darwin uninstall, ported from app-uninstall-service.test.ts. The fixture is the one in
// uninstall_service_test.go.

// Ported: "binds Trash to snapshot IDs, ignores injected input, revalidates identities, and moves app last".
func TestTrashBindsToSnapshotIDsAndMovesAppLast(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	d := f.discover(t)
	app, cache, prefs := d.Snapshot.Candidates[0], d.Snapshot.Candidates[1], d.Snapshot.Candidates[2]
	values := selectIDs(app, cache)
	values["injected-path"] = true
	complete := f.u.Trash(d.Snapshot, values)
	if complete.Status != StatusComplete {
		t.Fatalf("status = %s", complete.Status)
	}
	if want := []string{cache.Path, app.Path}; !reflect.DeepEqual(f.tree.trashed, want) {
		t.Fatalf("trashed = %v, want %v", f.tree.trashed, want)
	}
	if !f.tree.has(prefs.Path) {
		t.Fatal("unselected preferences were moved")
	}

	second := newDarwinFixture(t, fixtureOpts{})
	stale := second.discover(t)
	staleCache := candidateAt(t, stale, testCache)
	second.tree.replace(testCache)
	partial := second.u.Trash(stale.Snapshot, selectIDs(staleCache))
	if partial.Status != StatusFailed || len(second.tree.trashed) != 0 {
		t.Fatalf("stale candidate: status %s, trashed %v", partial.Status, second.tree.trashed)
	}
	if partial.Untouched[0].Code != codeChanged {
		t.Fatalf("untouched = %+v, want changed", partial.Untouched)
	}
}

// Ported: "does not probe or trash on zero selection and keeps everything when a selected running app will not quit".
func TestTrashKeepsEverythingWhenRunningAppWillNotQuit(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{running: true, quitStuck: true})
	d := f.discover(t)
	empty := f.u.Trash(d.Snapshot, map[string]bool{})
	if empty.Status != StatusFailed || len(f.tree.trashed) != 0 {
		t.Fatalf("zero selection: %+v trashed %v", empty, f.tree.trashed)
	}
	failed := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testApp), candidateAt(t, d, testCache)))
	if failed.Status != StatusFailed {
		t.Fatalf("status = %s", failed.Status)
	}
	var codes []string
	for _, u := range failed.Untouched {
		codes = append(codes, u.Code)
	}
	if want := []string{codeQuitTimeout, codeQuitTimeout}; !reflect.DeepEqual(codes, want) {
		t.Fatalf("codes = %v, want %v", codes, want)
	}
	if len(f.tree.trashed) != 0 {
		t.Fatalf("trashed = %v", f.tree.trashed)
	}
	if len(f.quitCalls) == 0 {
		t.Fatal("quit was not requested")
	}
}

// Ported: "does not move items when running process inspection fails".
func TestTrashDoesNotMoveWhenProcessInspectionFails(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{runningCheckFails: true})
	d := f.discover(t)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testApp), candidateAt(t, d, testCache)))
	if result.Status != StatusFailed || len(f.tree.trashed) != 0 {
		t.Fatalf("result %+v trashed %v", result, f.tree.trashed)
	}
}

// Ported: "quits a selected running app gracefully before moving associated data and the app itself".
func TestTrashQuitsRunningAppBeforeMoving(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{running: true})
	d := f.discover(t)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testApp), candidateAt(t, d, testCache)))
	if result.Status != StatusComplete {
		t.Fatalf("status = %s, untouched %+v", result.Status, result.Untouched)
	}
	if want := []string{testCache, testApp}; !reflect.DeepEqual(f.tree.trashed, want) {
		t.Fatalf("trashed = %v, want %v", f.tree.trashed, want)
	}
	if len(f.quitCalls) != 1 || f.quitCalls[0] != "quit:"+testApp {
		t.Fatalf("quit calls = %v", f.quitCalls)
	}
}

// Ported: "refuses to move associated data of a running app unless the app itself is selected".
func TestTrashRefusesDataOfRunningAppWithoutTheApp(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{running: true})
	d := f.discover(t)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testCache)))
	if result.Status != StatusFailed || result.Untouched[0].Code != codeRunning {
		t.Fatalf("result = %+v", result)
	}
	if len(f.tree.trashed) != 0 || len(f.quitCalls) != 0 {
		t.Fatalf("trashed %v, quit %v", f.tree.trashed, f.quitCalls)
	}
}

// Ported: "moves other selected items and the app when an associated item fails".
func TestTrashContinuesPastAFailedItem(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{failTrashAt: testCache})
	d := f.discover(t)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testApp), candidateAt(t, d, testCache), candidateAt(t, d, testPreferences)))
	if result.Status != StatusPartial {
		t.Fatalf("status = %s", result.Status)
	}
	if want := []string{testPreferences, testApp}; !reflect.DeepEqual(f.tree.trashed, want) {
		t.Fatalf("trashed = %v, want %v", f.tree.trashed, want)
	}
	if len(result.Untouched) != 1 || result.Untouched[0].Path != testCache || result.Untouched[0].Code != codeTrashFailed {
		t.Fatalf("untouched = %+v", result.Untouched)
	}
	if f.tree.has(testApp) {
		t.Fatal("the app is still present")
	}
}

// Ported: "moves the app and other data if one selected item changed after review".
func TestTrashMovesTheRestWhenOneItemChanged(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	d := f.discover(t)
	f.tree.replace(testCache)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testApp), candidateAt(t, d, testCache), candidateAt(t, d, testPreferences)))
	if result.Status != StatusPartial {
		t.Fatalf("status = %s", result.Status)
	}
	if want := []string{testPreferences, testApp}; !reflect.DeepEqual(f.tree.trashed, want) {
		t.Fatalf("trashed = %v, want %v", f.tree.trashed, want)
	}
	if result.Untouched[0].Code != codeChanged || result.Untouched[0].Path != testCache {
		t.Fatalf("untouched = %+v", result.Untouched)
	}
}

// Ported: "reports moved data and retains the app when the app cannot be moved".
func TestTrashReportsMovedDataWhenAppCannotMove(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{failTrashAt: testApp})
	d := f.discover(t)
	result := f.u.Trash(d.Snapshot, selectIDs(candidateAt(t, d, testApp), candidateAt(t, d, testCache)))
	if result.Status != StatusPartial || !reflect.DeepEqual(result.Moved, []string{testCache}) {
		t.Fatalf("result = %+v", result)
	}
	if result.Untouched[0].Path != testApp || !f.tree.has(testApp) {
		t.Fatalf("the app was not retained: %+v", result.Untouched)
	}
}

// Ported: "rejects app bundle changes and a running transition after candidate revalidation before Trash".
func TestTrashRefusesChangedAppAndLateRunningTransition(t *testing.T) {
	replaced := newDarwinFixture(t, fixtureOpts{})
	stale := replaced.discover(t)
	replaced.bundleID = "com.example.Replaced"
	changed := replaced.u.Trash(stale.Snapshot, selectIDs(stale.Snapshot.Candidates[0]))
	if changed.Status != StatusFailed || changed.Untouched[0].Code != codeAppChanged || len(replaced.tree.trashed) != 0 {
		t.Fatalf("changed app: %+v trashed %v", changed, replaced.tree.trashed)
	}

	late := newDarwinFixture(t, fixtureOpts{})
	discovered := late.discover(t)
	armed := true
	late.tree.onLstat = func(p string) {
		if armed && p == testCache {
			late.running = true
		}
	}
	stopped := late.u.Trash(discovered.Snapshot, selectIDs(candidateAt(t, discovered, testCache)))
	if stopped.Status != StatusFailed || stopped.Untouched[0].Code != codeRunning {
		t.Fatalf("late running: %+v", stopped)
	}
	if !late.running || len(late.tree.trashed) != 0 {
		t.Fatalf("late running: running=%v trashed=%v", late.running, late.tree.trashed)
	}
}
