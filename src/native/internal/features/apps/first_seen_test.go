package apps

import "testing"

// Ported from app-index-service.test.ts: first-seen tracking. The 1.x tests passed an identity function
// (value => value); the Go tests pass the same.

const (
	firstScanTime = 100
	newAppTime    = 200
	updateTime    = 300
)

func identity(value string) string { return value }

// Ported: "first-seen tracking confirms a neutral baseline".
func TestFirstSeenConfirmsNeutralBaseline(t *testing.T) {
	baseline, state, changed := TrackFirstSeen(
		[]App{{Name: "Existing", Path: "/Applications/Existing.app"}},
		FirstSeen{ByID: map[string]int64{}},
		false, firstScanTime, identity,
	)
	if baseline[0].FirstSeenAt != 0 || state.Initialized || !changed {
		t.Fatalf("neutral scan: firstSeenAt=%d initialized=%v changed=%v", baseline[0].FirstSeenAt, state.Initialized, changed)
	}

	confirmed, state, _ := TrackFirstSeen(
		[]App{
			{Name: "Existing", Path: "/Applications/Existing.app"},
			{Name: "Recovered", Path: "/Applications/Recovered.app"},
		},
		state, true, firstScanTime, identity,
	)
	if got := []int64{confirmed[0].FirstSeenAt, confirmed[1].FirstSeenAt}; got[0] != 0 || got[1] != 0 {
		t.Fatalf("confirmed firstSeenAt = %v, want [0 0]", got)
	}
	if !state.Initialized {
		t.Fatal("baseline not initialized by the confirming scan")
	}

	_, state, _ = TrackFirstSeen(nil, FirstSeen{ByID: map[string]int64{}}, true, firstScanTime, identity)
	if !state.Initialized {
		t.Fatal("an empty confirming scan must still initialize the baseline")
	}
}

// Ported: "first-seen tracking dates new paths but retains updates".
func TestFirstSeenDatesNewPathsButRetainsUpdates(t *testing.T) {
	next, state, _ := TrackFirstSeen(
		[]App{
			{Name: "Existing", Path: "/Applications/Existing.app"},
			{Name: "Recovered", Path: "/Applications/Recovered.app"},
			{Name: "New", Path: "/Applications/New.app"},
		},
		FirstSeen{ByID: map[string]int64{"/Applications/Existing.app": 0, "/Applications/Recovered.app": 0}, Initialized: true},
		false, newAppTime, identity,
	)
	got := []int64{next[0].FirstSeenAt, next[1].FirstSeenAt, next[2].FirstSeenAt}
	if got[0] != 0 || got[1] != 0 || got[2] != newAppTime {
		t.Fatalf("firstSeenAt = %v, want [0 0 %d]", got, newAppTime)
	}

	after, _, changed := TrackFirstSeen(
		[]App{{Name: "New", Path: "/Applications/New.app"}},
		state, false, updateTime, identity,
	)
	if after[0].FirstSeenAt != newAppTime {
		t.Fatalf("updated app firstSeenAt = %d, want %d", after[0].FirstSeenAt, newAppTime)
	}
	if changed {
		t.Fatal("a rescan of known apps reported a change")
	}
}

// Hostile input beyond 1.x: a path that differs only by case on a case-insensitive OS is one app when the
// identity function lowercases it, so the same app is not dated twice.
func TestFirstSeenIdentityCollapsesCaseVariants(t *testing.T) {
	lower := func(value string) string {
		out := []rune(value)
		for i, r := range out {
			if r >= 'A' && r <= 'Z' {
				out[i] = r + 32
			}
		}
		return string(out)
	}
	_, state, _ := TrackFirstSeen(
		[]App{{Name: "Notes", Path: "/Applications/Notes.app"}},
		FirstSeen{ByID: map[string]int64{}, Initialized: true}, false, newAppTime, lower,
	)
	if len(state.ByID) != 1 {
		t.Fatalf("keys = %v, want one", state.ByID)
	}
	if _, ok := state.ByID["/applications/notes.app"]; !ok {
		t.Fatalf("key not lowercased: %v", state.ByID)
	}
}
