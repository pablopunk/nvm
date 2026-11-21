package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestBackupRotationKeepsExactlyTheNewestN(t *testing.T) {
	s, files, _ := newFixture(t)
	doc := settingsDocument(s)
	for version := 1; version <= 6; version++ {
		if err := doc.Update(func(v *settingsDoc) { v.Count = version }); err != nil {
			t.Fatalf("Update %d: %v", version, err)
		}
		if err := doc.Flush(); err != nil {
			t.Fatalf("Flush %d: %v", version, err)
		}
	}
	for backup, want := range map[int]int{1: 5, 2: 4, 3: 3} {
		assertBackupHoldsCount(t, files, backup, want)
	}
	if _, err := files.Stat(settingsPath + ".bak.4"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a fourth backup exists with the default of three: %v", err)
	}
	assertBackupHoldsCount(t, files, 0, 6)
}

// assertBackupHoldsCount checks the count stored in a backup, or in the live file when backup is 0.
func assertBackupHoldsCount(t *testing.T, files *fake.FS, backup, want int) {
	t.Helper()
	path := settingsPath
	if backup > 0 {
		path = fmt.Sprintf("%s.bak.%d", settingsPath, backup)
	}
	var env struct {
		Data settingsDoc `json:"data"`
	}
	if err := json.Unmarshal(mustRead(t, files, path), &env); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if env.Data.Count != want {
		t.Fatalf("%s holds count %d, want %d", path, env.Data.Count, want)
	}
}

func TestZeroKeepBackupsWritesNoBackups(t *testing.T) {
	s, files, _ := newFixture(t, WithKeepBackups(0))
	doc := settingsDocument(s)
	for version := 1; version <= 3; version++ {
		if err := doc.Update(func(v *settingsDoc) { v.Count = version }); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if err := doc.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
	}
	for _, path := range files.Paths() {
		if bytes.Contains([]byte(path), []byte(".bak.")) {
			t.Fatalf("backup written with WithKeepBackups(0): %s", path)
		}
	}
}

func TestBackupFailureIsReportedWithoutFailingTheSave(t *testing.T) {
	s, files, _ := newFixture(t, WithKeepBackups(1))
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("first Flush: %v", err)
	}
	// A folder where the backup must go makes the rotation fail while the main write still succeeds.
	if err := files.MkdirAll(settingsPath+".bak.1", 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Count = 2 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush returned %v; a backup failure must not fail the save", err)
	}
	assertBackupHoldsCount(t, files, 0, 2)
	if _, ok := findNotice(drainNotices(s.Notices()), BackupFailed); !ok {
		t.Fatal("no BackupFailed notice")
	}
}
