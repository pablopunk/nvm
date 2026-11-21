//go:build darwin || linux

package real

import (
	"path/filepath"
	"testing"
)

func TestDirectoryAccess(t *testing.T) {
	dir := t.TempDir()
	access := directoryAccess{}
	if !access.CanWrite(dir) {
		t.Fatal("temporary directory should be writable and searchable")
	}
	if access.CanWrite(filepath.Join(dir, "missing")) {
		t.Fatal("missing directory should be denied")
	}
}
