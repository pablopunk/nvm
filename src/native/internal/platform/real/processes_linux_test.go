package real

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListProcessesReadsOwnedProcessEntries(t *testing.T) {
	root := t.TempDir()
	writeProcEntry(t, root, "101", "nvm\n", "100 25 3 1 0 9 0\n")
	writeProcEntry(t, root, "self", "self\n", "100 25 3 1 0 9 0\n")
	if err := os.WriteFile(filepath.Join(root, "202"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	list, err := listProcessesUnder(root, os.Getuid())
	if err != nil {
		t.Fatalf("listProcessesUnder: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d processes, want 1: %+v", len(list), list)
	}
	got := list[0]
	wantKB := int64(25 * os.Getpagesize() / 1024)
	if got.PID != 101 || got.Name != "nvm" || got.ResidentKB != wantKB {
		t.Fatalf("got %+v, want PID 101, name nvm, %d KB", got, wantKB)
	}
}

func TestListProcessesSkipsProcessesOfOtherUsers(t *testing.T) {
	root := t.TempDir()
	writeProcEntry(t, root, "303", "other\n", "100 10 3 1 0 9 0\n")

	list, err := listProcessesUnder(root, os.Getuid()+1)
	if err != nil {
		t.Fatalf("listProcessesUnder: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("got %+v, want no processes of another uid", list)
	}
}

// writeProcEntry creates a procfs-style directory with comm and statm files.
func writeProcEntry(t *testing.T, root, name, comm, statm string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "statm"), []byte(statm), 0o644); err != nil {
		t.Fatal(err)
	}
}
