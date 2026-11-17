package recordings

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const folder = "/data/dictation-recordings"

// clock returns a time source the test moves forward one second per call to Save.
func clock(start time.Time) func() time.Time {
	now := start
	return func() time.Time {
		now = now.Add(time.Second)
		return now
	}
}

func newStore(t *testing.T) (*Store, *fake.FS) {
	t.Helper()
	files := fake.NewFS()
	return New(files, folder, clock(time.Unix(1_700_000_000, 0))), files
}

func TestSavedSegmentsComeBackInOrder(t *testing.T) {
	store, _ := newStore(t)
	segments := [][]byte{[]byte("first"), []byte("second")}
	if err := store.Save("dictation-abc", segments); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load("dictation-abc")
	if err != nil || len(got) != 2 || string(got[0]) != "first" || string(got[1]) != "second" {
		t.Fatalf("Load = %q, %v; want the two segments in order", got, err)
	}
}

func TestListIsNewestFirstAndCountsSegments(t *testing.T) {
	store, _ := newStore(t)
	for _, id := range []string{"dictation-old", "dictation-new"} {
		if err := store.Save(id, [][]byte{[]byte("a")}); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}
	list, err := store.List()
	if err != nil || len(list) != 2 || list[0].ID != "dictation-new" || list[1].SegmentCount != 1 {
		t.Fatalf("List = %+v, %v; want the newest first", list, err)
	}
}

func TestRemoveDeletesTheRecordAndItsSegments(t *testing.T) {
	store, files := newStore(t)
	if err := store.Save("dictation-gone", [][]byte{[]byte("a"), []byte("b")}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Remove("dictation-gone"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := store.Load("dictation-gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load after Remove = %v, want ErrNotFound", err)
	}
	entries, _ := files.ReadDir(folder)
	if len(entries) != 0 {
		t.Fatalf("files left after Remove: %v", entries)
	}
	if err := store.Remove("dictation-gone"); err != nil {
		t.Fatalf("Remove of a missing recording = %v, want nil", err)
	}
}

func TestAnIncompleteSaveIsNotListed(t *testing.T) {
	store, files := newStore(t)
	if err := files.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile(folder+"/dictation-half.0.webm", []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("List = %+v, %v; want a segment without its record ignored", list, err)
	}
}

func TestIdentifiersOutsideTheDictationPatternAreRefused(t *testing.T) {
	store, files := newStore(t)
	for _, id := range []string{"", "../escape", "dictation-../x", "notes-1", "dictation-"} {
		if err := store.Save(id, [][]byte{[]byte("a")}); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("Save(%q) = %v, want ErrInvalidID", id, err)
		}
	}
	if _, err := files.ReadDir(folder); err == nil {
		t.Fatalf("a refused save created the folder")
	}
}

func TestSegmentCountIsBounded(t *testing.T) {
	store, _ := newStore(t)
	if err := store.Save("dictation-none", nil); !errors.Is(err, ErrSegmentCount) {
		t.Fatalf("Save with no segments = %v, want ErrSegmentCount", err)
	}
	tooMany := slices.Repeat([][]byte{[]byte("a")}, maxSegments+1)
	if err := store.Save("dictation-many", tooMany); !errors.Is(err, ErrSegmentCount) {
		t.Fatalf("Save with %d segments = %v, want ErrSegmentCount", len(tooMany), err)
	}
}

func TestAFailureReasonSurvivesARestart(t *testing.T) {
	store, files := newStore(t)
	if err := store.Save("dictation-failed", [][]byte{[]byte("a")}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkFailed("dictation-failed", "Sign in to use API dictation."); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	restarted := New(files, folder, clock(time.Unix(1_700_000_100, 0)))
	list, err := restarted.List()
	if err != nil || len(list) != 1 || list[0].Failure != "Sign in to use API dictation." || list[0].SegmentCount != 1 {
		t.Fatalf("List after restart = %+v, %v; want the failed entry with its reason", list, err)
	}
	if segments, err := restarted.Load("dictation-failed"); err != nil || len(segments) != 1 {
		t.Fatalf("Load after restart = %q, %v; want the audio", segments, err)
	}
}

func TestADictationIsListedFromItsFirstSegmentAndGrowsInPlace(t *testing.T) {
	store, _ := newStore(t)
	if err := store.SaveSegment("dictation-live", 0, []byte("first sec")); err != nil {
		t.Fatalf("SaveSegment: %v", err)
	}
	list, err := store.List()
	if err != nil || len(list) != 1 || list[0].SegmentCount != 1 || list[0].Failure != "" {
		t.Fatalf("List = %+v, %v; want the dictation listed as unfinished from its first segment", list, err)
	}
	created := list[0].CreatedAt
	if err := store.SaveSegment("dictation-live", 0, []byte("first minute")); err != nil {
		t.Fatalf("SaveSegment again: %v", err)
	}
	if err := store.SaveSegment("dictation-live", 1, []byte("second")); err != nil {
		t.Fatalf("SaveSegment 1: %v", err)
	}
	got, err := store.Load("dictation-live")
	if err != nil || len(got) != 2 || string(got[0]) != "first minute" || string(got[1]) != "second" {
		t.Fatalf("Load = %q, %v; want the replaced first segment and the second", got, err)
	}
	if list, _ := store.List(); list[0].CreatedAt != created {
		t.Fatalf("CreatedAt moved from %d to %d; a dictation keeps the time it started", created, list[0].CreatedAt)
	}
}

func TestRecordingsAreNeverPruned(t *testing.T) {
	store, _ := newStore(t)
	for index := range 150 {
		if err := store.Save(fmt.Sprintf("dictation-%03d", index), [][]byte{make([]byte, 1<<10)}); err != nil {
			t.Fatalf("Save %d: %v", index, err)
		}
	}
	if list, _ := store.List(); len(list) != 150 {
		t.Fatalf("kept %d recordings, want all 150: unfinished audio is never dropped", len(list))
	}
}

func TestARecordWithoutAudioIsNotListed(t *testing.T) {
	store, files := newStore(t)
	if err := files.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile(folder+"/dictation-empty.json", []byte(`{"id":"dictation-empty","createdAt":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if list, err := store.List(); err != nil || len(list) != 0 {
		t.Fatalf("List = %+v, %v; want a record with no segment skipped", list, err)
	}
	if _, err := store.Load("dictation-empty"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load = %v, want ErrNotFound", err)
	}
}

func TestALeftoverTemporaryFileIsNotASegment(t *testing.T) {
	store, files := newStore(t)
	if err := store.SaveSegment("dictation-cut", 0, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile(folder+"/dictation-cut.1.webm.tmp", []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Load("dictation-cut"); err != nil || len(got) != 1 {
		t.Fatalf("Load = %q, %v; want only the complete segment", got, err)
	}
}
