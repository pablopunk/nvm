package files

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The cases below port find-files-missing-roots.test.ts and the partitionRootsByExistence cases of file-utils.

// TestPartitionRootsSeparatesExistingFromMissing ports the first partition case.
func TestPartitionRootsSeparatesExistingFromMissing(t *testing.T) {
	fsys := fake.NewFS()
	dir := "/tmp/work"
	fsys.MkdirAll(dir, 0o755)
	missing := filepath.Join(dir, "nonexistent")
	existing, gone := PartitionRoots(fsys, []string{dir, missing})
	if !slices.Equal(existing, []string{dir}) || !slices.Equal(gone, []string{missing}) {
		t.Fatalf("PartitionRoots = %v / %v, want [%s] / [%s]", existing, gone, dir, missing)
	}
}

// TestPartitionRootsReturnsEmptyListsForEmptyInput ports the empty-input case.
func TestPartitionRootsReturnsEmptyListsForEmptyInput(t *testing.T) {
	existing, gone := PartitionRoots(fake.NewFS(), nil)
	if len(existing) != 0 || len(gone) != 0 {
		t.Fatalf("PartitionRoots(nil) = %v / %v, want empty", existing, gone)
	}
}

// TestPartitionRootsHandlesAllExistingRoots ports the all-existing case.
func TestPartitionRootsHandlesAllExistingRoots(t *testing.T) {
	fsys := fake.NewFS()
	a, b := "/tmp/work/a", "/tmp/work/b"
	fsys.MkdirAll(a, 0o755)
	fsys.MkdirAll(b, 0o755)
	existing, gone := PartitionRoots(fsys, []string{a, b})
	if !slices.Equal(existing, []string{a, b}) || len(gone) != 0 {
		t.Fatalf("PartitionRoots = %v / %v, want both existing", existing, gone)
	}
}

// TestPartitionRootsHandlesAllMissingRoots ports the all-missing case.
func TestPartitionRootsHandlesAllMissingRoots(t *testing.T) {
	fsys := fake.NewFS()
	a, b := "/tmp/work/no-a", "/tmp/work/no-b"
	existing, gone := PartitionRoots(fsys, []string{a, b})
	if len(existing) != 0 || !slices.Equal(gone, []string{a, b}) {
		t.Fatalf("PartitionRoots = %v / %v, want both missing", existing, gone)
	}
}

// TestPartitionRootsSeparatesMixedRoots ports the mixed case.
func TestPartitionRootsSeparatesMixedRoots(t *testing.T) {
	fsys := fake.NewFS()
	real1, real2 := "/tmp/work/real-a", "/tmp/work/real-b"
	fsys.MkdirAll(real1, 0o755)
	fsys.MkdirAll(real2, 0o755)
	ghost1, ghost2 := "/tmp/work/ghost-1", "/tmp/work/ghost-2"
	existing, gone := PartitionRoots(fsys, []string{real1, ghost1, real2, ghost2})
	if !slices.Equal(existing, []string{real1, real2}) || !slices.Equal(gone, []string{ghost1, ghost2}) {
		t.Fatalf("PartitionRoots = %v / %v", existing, gone)
	}
}

// TestPartitionRootsTreatsAFileInTheWayAsMissing checks that a parent that is a file reads as ENOTDIR, so the root
// is missing rather than unreadable.
func TestPartitionRootsTreatsAFileInTheWayAsMissing(t *testing.T) {
	if !isMissingPath(&fs.PathError{Op: "stat", Path: "x", Err: syscall.ENOTDIR}) {
		t.Fatal("ENOTDIR should count as a missing path")
	}
	if isMissingPath(&fs.PathError{Op: "stat", Path: "x", Err: errors.New("i/o error")}) {
		t.Fatal("an I/O error should keep the root in the existing list")
	}
}
