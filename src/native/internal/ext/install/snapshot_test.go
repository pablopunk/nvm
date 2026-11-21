package install

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestSourceSnapshotIncludesAllSourcesAndRefusesUnrelatedFiles(t *testing.T) {
	files := fake.NewFS()
	for name, data := range contents("acme.good", "package main") {
		files.Seed(root+"/acme.good/"+name, data)
	}
	files.Seed(root+"/acme.good/helper.go", []byte("package main"))
	snapshot, err := SourceSnapshot(files, root, "acme.good")
	if err != nil || len(snapshot) != 3 || string(snapshot["helper.go"]) != "package main" {
		t.Fatalf("snapshot = %v, %v", snapshot, err)
	}
	files.Seed(root+"/acme.good/notes.txt", []byte("keep me"))
	if snapshot, err := SourceSnapshot(files, root, "acme.good"); err == nil || snapshot != nil {
		t.Fatalf("unrelated file accepted: %v, %v", snapshot, err)
	}
	data, err := files.ReadFile(root + "/acme.good/notes.txt")
	if err != nil || string(data) != "keep me" {
		t.Fatalf("unrelated file changed: %q, %v", data, err)
	}
}

func TestSourceSnapshotRejectsUnsafeAndIncompleteInputs(t *testing.T) {
	files := fake.NewFS()
	for name, data := range contents("acme.good", "package main") {
		files.Seed(root+"/acme.good/"+name, data)
	}
	for _, id := range []string{"../outside", "", ".", "..", `a\b`} {
		if _, err := SourceSnapshot(files, root, id); err == nil {
			t.Errorf("accepted %q", id)
		}
	}
	files.Seed(root+"/acme.good/large.go", []byte(strings.Repeat("x", int(maxSnapshotBytes)+1)))
	if _, err := SourceSnapshot(files, root, "acme.good"); err == nil {
		t.Fatal("accepted oversized source")
	}
	if err := files.Remove(root + "/acme.good/large.go"); err != nil {
		t.Fatal(err)
	}
	files.FailNext(fake.OpOpenReadNoFollow, fs.ErrPermission)
	if snapshot, err := SourceSnapshot(files, root, "acme.good"); err == nil || snapshot != nil {
		t.Fatalf("accepted unreadable source: %v, %v", snapshot, err)
	}
}

func TestSourceSnapshotRejectsDirectoriesAndMissingRequiredFiles(t *testing.T) {
	files := fake.NewFS()
	for name, data := range contents("acme.good", "package main") {
		files.Seed(root+"/acme.good/"+name, data)
	}
	files.Seed(root+"/acme.good/extra.go/nested", []byte("keep"))
	if _, err := SourceSnapshot(files, root, "acme.good"); err == nil {
		t.Fatal("accepted a directory named like source")
	}
	files2 := fake.NewFS()
	files2.Seed(root+"/acme.good/extension.json", []byte("{}"))
	if _, err := SourceSnapshot(files2, root, "acme.good"); err == nil {
		t.Fatal("accepted a missing main.go")
	}
}
