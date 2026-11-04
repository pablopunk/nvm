package store

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// canary is a secret value that must never appear in any file, backup, notice or error text.
const canary = "canary-7f3a9c-do-not-persist"

func TestSecretFieldNeverReachesDiskBackupsOrNotices(t *testing.T) {
	s, files, _ := newFixture(t)
	doc := settingsDocument(s)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "visible"; v.Token = canary })
	err := doc.Flush()
	if !errors.Is(err, ErrSecretInStore) {
		t.Fatalf("Flush = %v, want ErrSecretInStore", err)
	}
	raw := mustRead(t, files, settingsPath)
	if bytes.Contains(raw, []byte(canary)) {
		t.Fatalf("the secret reached the file:\n%s", raw)
	}
	if !bytes.Contains(raw, []byte("visible")) {
		t.Fatal("omitting the secret also dropped the rest of the document")
	}

	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "second" })
	if err := doc.Flush(); !errors.Is(err, ErrSecretInStore) {
		t.Fatalf("second Flush with the secret still held = %v, want ErrSecretInStore on every save", err)
	}
	if !bytes.Contains(mustRead(t, files, settingsPath), []byte("second")) {
		t.Fatal("the second save did not write the document")
	}
	for _, path := range files.Paths() {
		if strings.HasPrefix(treePath(path), testDir+string(filepath.Separator)) && !isDirectory(files, path) {
			if bytes.Contains(mustRead(t, files, path), []byte(canary)) {
				t.Fatalf("the secret reached %s", path)
			}
		}
	}

	notices := drainNotices(s.Notices())
	if _, ok := findNotice(notices, SecretOmitted); !ok {
		t.Fatal("no SecretOmitted notice")
	}
	for _, notice := range notices {
		if strings.Contains(notice.Reason, canary) || strings.Contains(notice.Path, canary) {
			t.Fatalf("a notice carries the secret: %+v", notice)
		}
	}
}

func TestSecretStaysUsableInMemory(t *testing.T) {
	s, _, _ := newFixture(t)
	doc := settingsDocument(s)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Token = canary })
	_ = doc.Flush()
	got, err := doc.Load()
	if err != nil || got.Token != canary {
		t.Fatalf("Load after omission = %q, %v; the app must still see its secret", got.Token, err)
	}
}

func TestSaveReportsSecretOmissionAndCleanDocumentsDoNot(t *testing.T) {
	s, _, _ := newFixture(t)
	doc := settingsDocument(s)
	mustUpdate(t, doc, func(v *settingsDoc) { v.Title = "clean" })
	if err := doc.Save(); err != nil {
		t.Fatalf("Save of a document without secrets: %v", err)
	}
	mustUpdate(t, doc, func(v *settingsDoc) { v.Token = canary })
	if err := doc.Save(); !errors.Is(err, ErrSecretInStore) {
		t.Fatalf("Save with a secret = %v, want ErrSecretInStore", err)
	}
}

func TestRedactionLeavesTheOriginalValueAlone(t *testing.T) {
	value := settingsDoc{Title: "kept", Token: canary}
	redacted, omitted := redactSecrets(value, secretFieldIndexes[settingsDoc]())
	if value.Token != canary {
		t.Fatal("redaction changed the original value")
	}
	if redacted.Token != "" || redacted.Title != "kept" {
		t.Fatalf("redacted = %+v", redacted)
	}
	if !reflect.DeepEqual(omitted, []string{"Token"}) {
		t.Fatalf("omitted = %v, want [Token]", omitted)
	}
}

// isDirectory reports whether a path in the fake tree is a folder.
func isDirectory(files *fake.FS, path string) bool {
	info, err := files.Stat(path)
	return err == nil && info.IsDir()
}
