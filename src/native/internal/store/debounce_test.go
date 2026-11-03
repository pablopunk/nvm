package store

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// waitTimeout bounds how long a test waits for a background write. It is a safety net for a broken
// implementation, not a sleep: a correct store answers at once.
const waitTimeout = 5 * time.Second

// waitForSave receives the path of the next write from saved, or fails the test.
func waitForSave(t *testing.T, saved <-chan string) string {
	t.Helper()
	select {
	case path := <-saved:
		return path
	case <-time.After(waitTimeout):
		t.Fatal("no background write happened")
		return ""
	}
}

// waitForNotice blocks until a notice of kind arrives on the store, or fails the test.
func waitForNotice(t *testing.T, s *Store, kind Kind) Notice {
	t.Helper()
	timeout := time.After(waitTimeout)
	for {
		select {
		case notice := <-s.Notices():
			if notice.Kind == kind {
				return notice
			}
		case <-timeout:
			t.Fatalf("no %s notice arrived", kind)
		}
	}
}

// mustUpdate applies a change and fails the test on error.
func mustUpdate(t *testing.T, doc *Collection[settingsDoc], change func(*settingsDoc)) {
	t.Helper()
	if err := doc.Update(change); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

func TestHundredUpdatesCoalesceIntoOneWrite(t *testing.T) {
	saved := make(chan string, 16)
	s, _, clock := newFixture(t, WithDebounce(500*time.Millisecond), WithOnSaved(func(path string) { saved <- path }))
	doc := settingsDocument(s)
	for range 100 {
		mustUpdate(t, doc, func(v *settingsDoc) { v.Count++ })
	}
	if len(saved) != 0 {
		t.Fatal("a write happened before the debounce elapsed")
	}
	clock.Advance(500 * time.Millisecond)
	waitForSave(t, saved)
	if len(saved) != 0 {
		t.Fatalf("%d writes for 100 Updates, want one", len(saved)+1)
	}
	got, err := doc.Load()
	if err != nil || got.Count != 100 {
		t.Fatalf("Load = %+v, %v; want Count 100", got, err)
	}
}

func TestUpdateRestartsTheDebounceWindow(t *testing.T) {
	saved := make(chan string, 4)
	s, _, clock := newFixture(t, WithDebounce(500*time.Millisecond), WithOnSaved(func(path string) { saved <- path }))
	doc := settingsDocument(s)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Count = 1 })
	clock.Advance(400 * time.Millisecond)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Count = 2 })
	clock.Advance(100 * time.Millisecond)
	clock.AwaitTimers(1)
	if len(saved) != 0 {
		t.Fatal("the first window ended the debounce although a later Update moved it")
	}
	clock.Advance(399 * time.Millisecond)
	if len(saved) != 0 {
		t.Fatal("a write happened before the restarted window ended")
	}
	clock.Advance(time.Millisecond)
	waitForSave(t, saved)
}

func TestFlushWritesWithoutWaitingForTheDebounce(t *testing.T) {
	s, files, _ := newFixture(t)
	doc := settingsDocument(s)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "now" })
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	version, _ := envelopeOf(t, mustRead(t, files, settingsPath))
	if version != 1 {
		t.Fatalf("file version = %d after Flush", version)
	}
}

func TestCloseFlushesPendingChanges(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	s := openStoreOn(t, files, clock)
	doc := settingsDocument(s)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "saved on close" })
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	restarted := openStoreOn(t, files, clock)
	got, err := settingsDocument(restarted).Load()
	if err != nil || got.Title != "saved on close" {
		t.Fatalf("after Close, Load = %+v, %v", got, err)
	}
}

func TestFailedDebouncedWriteIsRetriedByTheNextFlush(t *testing.T) {
	s, files, clock := newFixture(t, WithDebounce(time.Second))
	doc := settingsDocument(s)
	files.FailNext(fake.OpRename, errDisk)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "retry me" })
	clock.Advance(time.Second)
	notice := waitForNotice(t, s, SaveFailed)
	if notice.Path != settingsPath {
		t.Fatalf("SaveFailed notice = %+v", notice)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("retry Flush: %v", err)
	}
	version, _ := envelopeOf(t, mustRead(t, files, settingsPath))
	if version != 1 {
		t.Fatalf("retry did not write, version %d", version)
	}
}
