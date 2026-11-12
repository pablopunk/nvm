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

func TestTheOldestRecordingsGoPastTheLimit(t *testing.T) {
	store, _ := newStore(t)
	for index := range MaxRecordings + 2 {
		if err := store.Save(fmt.Sprintf("dictation-%03d", index), [][]byte{[]byte("a")}); err != nil {
			t.Fatalf("Save %d: %v", index, err)
		}
	}
	list, _ := store.List()
	if len(list) != MaxRecordings {
		t.Fatalf("kept %d recordings, want %d", len(list), MaxRecordings)
	}
	if list[len(list)-1].ID != "dictation-002" {
		t.Fatalf("oldest kept = %s, want dictation-002", list[len(list)-1].ID)
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

func TestTheNewestRecordingsStayWithinTheTotalSize(t *testing.T) {
	store, files := newStore(t)
	store.maxBytes = 30
	chunk := make([]byte, 11)
	for _, id := range []string{"dictation-a", "dictation-b", "dictation-c", "dictation-d"} {
		if err := store.Save(id, [][]byte{chunk}); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}
	list, _ := store.List()
	if len(list) != 2 || list[0].ID != "dictation-d" || list[1].ID != "dictation-c" {
		t.Fatalf("kept %+v, want only the two newest within the size bound", list)
	}
	entries, _ := files.ReadDir(folder)
	if len(entries) != 4 {
		t.Fatalf("%d files left, want two records and two segments", len(entries))
	}
}

func TestTheNewestRecordingIsKeptEvenWhenItAloneIsOverTheSize(t *testing.T) {
	store, _ := newStore(t)
	store.maxBytes = 30
	if err := store.Save("dictation-huge", [][]byte{make([]byte, 31)}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if list, _ := store.List(); len(list) != 1 {
		t.Fatalf("kept %+v, want the one recording the user can retry", list)
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

func TestSegmentsOfAnIncompleteSaveAreSweptOnTheNextSave(t *testing.T) {
	store, files := newStore(t)
	if err := files.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile(folder+"/dictation-half.0.webm", []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("dictation-next", [][]byte{[]byte("a")}); err != nil {
		t.Fatal(err)
	}
	if _, err := files.Stat(folder + "/dictation-half.0.webm"); err == nil {
		t.Fatal("the orphan segment is still on disk")
	}
}
