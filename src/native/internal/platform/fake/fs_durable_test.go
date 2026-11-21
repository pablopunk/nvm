package fake

import (
	"errors"
	"io/fs"
	"testing"
)

func TestCreateExclusiveClaimsOnlyFreeNames(t *testing.T) {
	files := NewFS()
	files.Seed("/cfg/.keep", nil)
	if err := files.CreateExclusive("/cfg/store.lock", []byte("first"), 0o600); err != nil {
		t.Fatalf("first CreateExclusive: %v", err)
	}
	err := files.CreateExclusive("/cfg/store.lock", []byte("second"), 0o600)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("second CreateExclusive: %v, want ErrExist", err)
	}
	got, _ := files.ReadFile("/cfg/store.lock")
	if string(got) != "first" {
		t.Fatalf("content after refused create = %q, want the first writer's", got)
	}
}

func TestCreateExclusiveNeedsExistingFolder(t *testing.T) {
	files := NewFS()
	err := files.CreateExclusive("/missing/store.lock", []byte("x"), 0o600)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("CreateExclusive in missing folder: %v, want ErrNotExist", err)
	}
}

func TestSyncReportsMissingFilesAndHonoursFaults(t *testing.T) {
	files := NewFS()
	files.Seed("/cfg/settings.json", []byte("{}"))
	if err := files.Sync("/cfg/settings.json"); err != nil {
		t.Fatalf("Sync existing file: %v", err)
	}
	if err := files.Sync("/cfg/absent.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Sync missing file: %v, want ErrNotExist", err)
	}
	boom := errors.New("io error")
	files.FailNext(OpSync, boom)
	if err := files.Sync("/cfg/settings.json"); !errors.Is(err, boom) {
		t.Fatalf("Sync with fault: %v, want the injected error", err)
	}
	files.FailNext(OpCreateExclusive, boom)
	if err := files.CreateExclusive("/cfg/new", nil, 0o600); !errors.Is(err, boom) {
		t.Fatalf("CreateExclusive with fault: %v, want the injected error", err)
	}
}
