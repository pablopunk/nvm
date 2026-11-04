package fake

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// TestWatchFilteredDropsSkippedFoldersAndTheirEvents checks that a skipped folder's entries are neither reported nor
// counted: a change inside it stays silent while a change beside it is reported.
func TestWatchFilteredDropsSkippedFoldersAndTheirEvents(t *testing.T) {
	fsys := NewFS()
	fsys.Seed("/proj/a.txt", []byte("a"))
	fsys.Seed("/proj/skip/b.txt", []byte("b"))
	fsys.Seed("/proj/keep/c.txt", []byte("c"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	skip := func(rel string, isDir bool) bool { return rel == "skip" || rel == "skip/b.txt" }
	events, err := fsys.WatchFiltered(ctx, "/proj", skip)
	if err != nil {
		t.Fatalf("WatchFiltered: %v", err)
	}
	if err := fsys.WriteFile("/proj/skip/b.txt", []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile("/proj/keep/c.txt", []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-events:
		if ev.Path != "/proj/keep/c.txt" {
			t.Fatalf("first event = %q, want the kept file only", ev.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("no event for the kept file")
	}
	select {
	case ev := <-events:
		t.Fatalf("unexpected event %q from a skipped folder", ev.Path)
	default:
	}
}

// TestWatchFilteredFailsPastTheEntryLimit checks that a watch over more entries than the limit fails with
// platform.ErrWatchLimit, and that the same tree fits once skip drops enough of it.
func TestWatchFilteredFailsPastTheEntryLimit(t *testing.T) {
	fsys := NewFS()
	for _, name := range []string{"a", "b", "c"} {
		fsys.Seed("/proj/"+name+".txt", []byte(name))
	}
	fsys.Seed("/proj/node_modules/x.js", []byte("x"))
	fsys.SetWatchEntryLimit(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := fsys.WatchFiltered(ctx, "/proj", nil); !errors.Is(err, platform.ErrWatchLimit) {
		t.Fatalf("unfiltered watch past the limit: %v, want ErrWatchLimit", err)
	}
	skipModules := func(rel string, isDir bool) bool { return rel == "node_modules" }
	if _, err := fsys.WatchFiltered(ctx, "/proj", skipModules); !errors.Is(err, platform.ErrWatchLimit) {
		t.Fatalf("filtered watch still past the limit: %v, want ErrWatchLimit", err)
	}
	skipFiles := func(rel string, isDir bool) bool { return !isDir && rel != "a.txt" }
	if _, err := fsys.WatchFiltered(ctx, "/proj", skipFiles); err != nil {
		t.Fatalf("watch within the limit: %v", err)
	}
}
