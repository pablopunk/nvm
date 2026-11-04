package fake

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestWriteThenReadReturnsCopies(t *testing.T) {
	files := NewFS()
	if err := files.MkdirAll("/cfg", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data := []byte("first")
	if err := files.WriteFile("/cfg/settings.json", data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data[0] = 'X'
	got, err := files.ReadFile("/cfg/settings.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("ReadFile = %q, want the bytes at write time", got)
	}
}

func TestWriteRequiresExistingFolder(t *testing.T) {
	files := NewFS()
	err := files.WriteFile("/missing/settings.json", []byte("x"), 0o600)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("WriteFile in missing folder: %v, want ErrNotExist", err)
	}
}

func TestFailNextLeavesTreeUnchangedAndIsConsumed(t *testing.T) {
	files := NewFS()
	boom := errors.New("disk full")
	files.FailNext(OpWriteFile, boom)

	if err := files.WriteFile("/a.txt", []byte("x"), 0o600); !errors.Is(err, boom) {
		t.Fatalf("first WriteFile: %v, want the injected error", err)
	}
	if _, err := files.ReadFile("/a.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file exists after failed write: %v", err)
	}
	if err := files.WriteFile("/a.txt", []byte("x"), 0o600); err != nil {
		t.Fatalf("second WriteFile should succeed once the fault is consumed: %v", err)
	}
}

func TestAfterWriteHookErrorKeepsTheData(t *testing.T) {
	files := NewFS()
	files.Seed("/notes/.keep", nil)
	crash := errors.New("power lost after write")
	files.AfterWrite(func(string) error { return crash })

	if err := files.WriteFile("/notes/a.txt", []byte("saved"), 0o600); !errors.Is(err, crash) {
		t.Fatalf("WriteFile: %v, want the hook error", err)
	}
	files.AfterWrite(nil)
	got, err := files.ReadFile("/notes/a.txt")
	if err != nil || string(got) != "saved" {
		t.Fatalf("ReadFile after crash = %q, %v; the data must have been applied", got, err)
	}
}

func TestRenameMovesFolderSubtree(t *testing.T) {
	files := NewFS()
	files.Seed("/old/inner/file.txt", []byte("x"))
	if err := files.Rename("/old", "/new"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := files.ReadFile("/new/inner/file.txt"); err != nil {
		t.Fatalf("file under moved folder: %v", err)
	}
	if _, err := files.Stat("/old"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old folder still present: %v", err)
	}
}

func TestRemoveRefusesNonEmptyFolder(t *testing.T) {
	files := NewFS()
	files.Seed("/dir/file.txt", nil)
	if err := files.Remove("/dir"); err == nil {
		t.Fatal("Remove of a non-empty folder succeeded")
	}
	if err := files.Remove("/dir/file.txt"); err != nil {
		t.Fatalf("Remove file: %v", err)
	}
	if err := files.Remove("/dir"); err != nil {
		t.Fatalf("Remove empty folder: %v", err)
	}
}

func TestWalkVisitsInLexicalOrderAndHonorsSkipDir(t *testing.T) {
	tests := []struct {
		name  string
		skip  string
		wants []string
	}{
		{
			name:  "full walk",
			wants: []string{"/", "/a", "/a/b.txt", "/a/c", "/a/c/d.txt", "/e.txt"},
		},
		{
			name:  "SkipDir on a folder skips its contents only",
			skip:  "/a",
			wants: []string{"/", "/a", "/e.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := NewFS()
			files.Seed("/a/b.txt", nil)
			files.Seed("/a/c/d.txt", nil)
			files.Seed("/e.txt", nil)
			var visited []string
			err := files.Walk("/", func(name string, _ fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				visited = append(visited, name)
				if name == tt.skip {
					return fs.SkipDir
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Walk: %v", err)
			}
			if !reflect.DeepEqual(visited, tt.wants) {
				t.Fatalf("visited %v, want %v", visited, tt.wants)
			}
		})
	}
}

func TestReadDirIsSortedAndRejectsFiles(t *testing.T) {
	files := NewFS()
	files.Seed("/d/b.txt", nil)
	files.Seed("/d/a.txt", nil)
	files.Seed("/d/sub/c.txt", nil)
	entries, err := files.ReadDir("/d")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := []string{"a.txt", "b.txt", "sub"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("ReadDir names = %v, want %v", names, want)
	}
	if _, err := files.ReadDir("/d/a.txt"); err == nil {
		t.Fatal("ReadDir of a file succeeded")
	}
}

func TestStatReportsFolderAndFileShapes(t *testing.T) {
	files := NewFS()
	files.Seed("/d/a.txt", []byte("abc"))
	info, err := files.Stat("/d/a.txt")
	if err != nil || info.Size() != 3 || info.IsDir() || info.Name() != "a.txt" {
		t.Fatalf("Stat file = %v, %v", info, err)
	}
	dir, err := files.Stat("/d")
	if err != nil || !dir.IsDir() || !dir.Mode().IsDir() {
		t.Fatalf("Stat folder = %v, %v", dir, err)
	}
}

func TestWatchReportsChangesUnderFolderUntilCancelled(t *testing.T) {
	files := NewFS()
	files.MkdirAll("/proj", 0o755)
	files.MkdirAll("/other", 0o755)
	ctx, cancel := context.WithCancel(context.Background())
	events, err := files.Watch(ctx, "/proj")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	if err := files.WriteFile("/other/x.txt", nil, 0o600); err != nil {
		t.Fatalf("WriteFile outside: %v", err)
	}
	if err := files.WriteFile("/proj/a.txt", nil, 0o600); err != nil {
		t.Fatalf("WriteFile inside: %v", err)
	}
	got := <-events
	want := platform.Event{Op: platform.EventCreate, Path: "/proj/a.txt"}
	if got != want {
		t.Fatalf("first event = %+v, want %+v (events outside the folder are not reported)", got, want)
	}

	cancel()
	for range events {
	}
}

func TestWatchMissingFolderFails(t *testing.T) {
	_, err := NewFS().Watch(context.Background(), "/nowhere")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Watch of missing folder: %v, want ErrNotExist", err)
	}
}

// TestVolumeQualifiedNamesShareTheRootedTree checks that a Windows name with a drive letter reaches the node its
// rooted spelling names, so a path the OS resolves against the process volume finds what the fixture seeded.
func TestVolumeQualifiedNamesShareTheRootedTree(t *testing.T) {
	files := NewFS()
	files.Seed("/home/u/docs/notes.txt", []byte("notes"))
	for _, name := range []string{`C:\home\u\docs\notes.txt`, "d:/home/u/docs/notes.txt", "/home/u/docs/notes.txt"} {
		data, err := files.ReadFile(name)
		if err != nil || string(data) != "notes" {
			t.Fatalf("ReadFile(%q) = %q, %v; want the seeded file", name, data, err)
		}
	}
}
