package store

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCommitFailureDoesNotPublishOrRetry(t *testing.T) {
	s, files, _ := newFixture(t, WithKeepBackups(0))
	doc := settingsDocument(s)
	if err := doc.Commit(func(v *settingsDoc) error { v.Title = "before"; return nil }); err != nil {
		t.Fatal(err)
	}
	files.FailNext(fake.OpRename, fs.ErrPermission)
	if err := doc.Commit(func(v *settingsDoc) error { v.Title = "rejected"; return nil }); err == nil {
		t.Fatal("commit succeeded")
	}
	got, err := doc.Load()
	if err != nil || got.Title != "before" {
		t.Fatalf("load: %+v %v", got, err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatal(err)
	}
	got, err = doc.Load()
	if err != nil || got.Title != "before" {
		t.Fatalf("flush published rejected value: %+v %v", got, err)
	}
}

func TestCommitRejectsConflictPendingAndSecrets(t *testing.T) {
	s, _, _ := newFixture(t)
	doc := settingsDocument(s)
	conflict := errors.New("conflict")
	if err := doc.Commit(func(v *settingsDoc) error { v.Title = "bad"; return conflict }); !errors.Is(err, conflict) {
		t.Fatalf("conflict: %v", err)
	}
	if err := doc.Commit(func(v *settingsDoc) error { v.Token = "secret"; return nil }); !errors.Is(err, ErrSecretInStore) {
		t.Fatalf("secret: %v", err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Title = "pending" }); err != nil {
		t.Fatal(err)
	}
	if err := doc.Commit(func(v *settingsDoc) error { v.Title = "other"; return nil }); !errors.Is(err, ErrPendingChanges) {
		t.Fatalf("pending: %v", err)
	}
}
