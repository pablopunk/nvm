package store

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCommitFailureDoesNotPublishOrRetry(t *testing.T) {
	for _, operation := range []string{fake.OpWriteFile, fake.OpChmod, fake.OpSync, fake.OpRename} {
		t.Run(operation, func(t *testing.T) {
			s, files, _ := newFixture(t, WithKeepBackups(0))
			doc := settingsDocument(s)
			if err := doc.Commit(func(v *settingsDoc) error { v.Title = "before"; return nil }); err != nil {
				t.Fatal(err)
			}
			before := string(mustRead(t, files, doc.Path()))
			files.FailNext(operation, fs.ErrPermission)
			if err := doc.Commit(func(v *settingsDoc) error { v.Title = "rejected"; v.Items[0] = "rejected"; return nil }); err == nil {
				t.Fatal("commit succeeded")
			}
			if err := doc.Flush(); err != nil {
				t.Fatal(err)
			}
			got, err := doc.Load()
			if err != nil || got.Title != "before" || got.Items[0] != "alpha" {
				t.Fatalf("rejected state published: %+v %v", got, err)
			}
			if after := string(mustRead(t, files, doc.Path())); after != before {
				t.Fatal("failed commit or later flush changed persisted state")
			}
		})
	}
}

func TestCommitIsolatesRetainedReferencesAndRejectedMutations(t *testing.T) {
	s, files, _ := newFixture(t, WithKeepBackups(0))
	doc := settingsDocument(s)
	var retained *settingsDoc
	if err := doc.Commit(func(v *settingsDoc) error {
		v.Title = "committed"
		v.Items = []string{"original"}
		retained = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before := string(mustRead(t, files, doc.Path()))
	retained.Title = "mutated outside lock"
	retained.Items[0] = "mutated retained slice"
	refusal := errors.New("reject nested change")
	if err := doc.Commit(func(v *settingsDoc) error { v.Items[0] = "rejected"; return refusal }); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	got, err := doc.Load()
	if err != nil || got.Title != "committed" || got.Items[0] != "original" {
		t.Fatalf("retained reference changed committed state: %+v %v", got, err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatal(err)
	}
	if after := string(mustRead(t, files, doc.Path())); after != before {
		t.Fatal("retained reference or rejected callback changed disk")
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
