package store

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// corruptInputs are files that must be quarantined: each one is a crash artifact or a damaged envelope.
func corruptInputs(t *testing.T) map[string][]byte {
	t.Helper()
	valid := readTestdata(t, "v1_settings.json")
	return map[string][]byte{
		"truncated":        readTestdata(t, "truncated_settings.json"),
		"empty":            readTestdata(t, "empty_settings.json"),
		"whitespace only":  []byte(" \n\t "),
		"not JSON":         []byte("{not json"),
		"no envelope":      readTestdata(t, "wrong_envelope.json"),
		"missing data":     []byte(`{"schemaVersion":1}`),
		"schema version 0": []byte(`{"schemaVersion":0,"data":{}}`),
		"null data":        []byte(`{"schemaVersion":1,"data":null}`),
		"array document":   []byte(`[1,2,3]`),
		"trailing junk":    append(bytes.Clone(valid), []byte("garbage")...),
		"wrong field type": []byte(`{"schemaVersion":1,"data":{"count":"many"}}`),
	}
}

func TestCorruptFilesAreQuarantinedNotDeleted(t *testing.T) {
	for name, content := range corruptInputs(t) {
		t.Run(name, func(t *testing.T) {
			s, files, _ := newFixture(t)
			files.Seed(settingsPath, content)
			got, err := settingsDocument(s).Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.Title != "default" {
				t.Fatalf("Load = %+v, want the defaults", got)
			}
			quarantined := quarantinedFiles(files)
			if len(quarantined) != 1 {
				t.Fatalf("quarantined files = %v, want exactly one", quarantined)
			}
			if !bytes.Equal(mustRead(t, files, quarantined[0]), content) {
				t.Fatal("the quarantined copy does not hold the original bytes")
			}
			if _, err := files.Stat(settingsPath); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("the corrupt file is still in place: %v", err)
			}
			notice, ok := findNotice(drainNotices(s.Notices()), Quarantined)
			if !ok || notice.Path != quarantined[0] || notice.Reason == "" {
				t.Fatalf("Quarantined notice = %+v, %v", notice, ok)
			}
		})
	}
}

func TestQuarantinedDocumentAcceptsNewWrites(t *testing.T) {
	s, files, _ := newFixture(t)
	files.Seed(settingsPath, []byte("{broken"))
	doc := settingsDocument(s)
	if _, err := doc.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Title = "fresh" }); err != nil {
		t.Fatalf("Update after quarantine: %v", err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	version, data := envelopeOf(t, mustRead(t, files, settingsPath))
	if version != 1 || !bytes.Contains(data, []byte(`"fresh"`)) {
		t.Fatalf("new file = version %d, %s", version, data)
	}
}

func TestQuarantineNamesDoNotCollideWithinOneSecond(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	first := []byte("{first")
	second := []byte("{second")
	for _, content := range [][]byte{first, second} {
		files.Seed(settingsPath, content)
		s := openStoreOn(t, files, clock)
		if _, err := settingsDocument(s).Load(); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
	quarantined := quarantinedFiles(files)
	if len(quarantined) != 2 {
		t.Fatalf("quarantined files = %v, want two distinct names", quarantined)
	}
	if !bytes.Equal(mustRead(t, files, quarantined[0]), first) || !bytes.Equal(mustRead(t, files, quarantined[1]), second) {
		t.Fatal("a quarantined copy lost its original bytes")
	}
}

func TestFailedQuarantineOpensReadOnlyWithoutOverwriting(t *testing.T) {
	s, files, _ := newFixture(t)
	corrupt := []byte("{corrupt")
	files.Seed(settingsPath, corrupt)
	files.FailNext(fake.OpRename, errDisk)
	doc := settingsDocument(s)
	got, err := doc.Load()
	if !errors.Is(err, errDisk) || got.Title != "default" {
		t.Fatalf("Load = %+v, %v; want the injected error and defaults", got, err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Title = "x" }); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Update = %v, want ErrReadOnly", err)
	}
	if _, ok := findNotice(drainNotices(s.Notices()), LoadFailed); !ok {
		t.Fatal("no LoadFailed notice")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath), corrupt) {
		t.Fatal("a read-only document overwrote the corrupt file")
	}
}

func TestUnreadableFileOpensReadOnlyWithoutOverwriting(t *testing.T) {
	s, files, _ := newFixture(t)
	original := readTestdata(t, "v1_settings.json")
	files.Seed(settingsPath, original)
	files.FailNext(fake.OpReadFile, errDisk)
	doc := settingsDocument(s)
	if _, err := doc.Load(); !errors.Is(err, errDisk) {
		t.Fatalf("Load = %v, want the injected read error", err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Count = 9 }); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Update = %v, want ErrReadOnly", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath), original) {
		t.Fatal("an unreadable document was overwritten")
	}
}

func TestNewerVersionOpensReadOnlyAndKeepsItsFile(t *testing.T) {
	s, files, _ := newFixture(t)
	newer := readTestdata(t, "newer_settings.json")
	files.Seed(settingsPath, newer)
	doc := settingsDocument(s)
	got, err := doc.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Title != "from the future" {
		t.Fatalf("Load = %+v, want the newer file's fields", got)
	}
	notice, ok := findNotice(drainNotices(s.Notices()), NewerVersion)
	if !ok || notice.Path != settingsPath {
		t.Fatalf("NewerVersion notice = %+v, %v", notice, ok)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Update = %v, want ErrReadOnly", err)
	}
	if err := doc.Save(); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Save = %v, want ErrReadOnly", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath), newer) {
		t.Fatal("a newer file was rewritten by an older build")
	}
}

func TestNewerVersionWithDataThisBuildCannotReadStillStaysOnDisk(t *testing.T) {
	s, files, _ := newFixture(t)
	newer := []byte(`{"schemaVersion":4,"data":"not an object"}`)
	files.Seed(settingsPath, newer)
	doc := settingsDocument(s)
	got, err := doc.Load()
	if err != nil || got.Title != "default" {
		t.Fatalf("Load = %+v, %v; want defaults without an error", got, err)
	}
	notice, ok := findNotice(drainNotices(s.Notices()), NewerVersion)
	if !ok || !strings.Contains(notice.Reason, "could not be read") {
		t.Fatalf("NewerVersion notice = %+v, %v", notice, ok)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath), newer) {
		t.Fatal("unreadable newer data was overwritten")
	}
}
