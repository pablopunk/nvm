package install

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const root = "/ext"

func contents(id, source string) map[string][]byte {
	return map[string][]byte{
		bundle.ManifestFile: []byte(`{"apiVersion":"1.0","id":"` + id + `","title":"T"}`),
		bundle.SourceFile:   []byte(source),
	}
}

func visibleFolders(t *testing.T, files *fake.FS) []string {
	t.Helper()
	entries, err := files.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestInstallPublishesAValidExtensionAndLeavesNoStaging(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	if err := Install(files, clock, root, "acme.good", contents("acme.good", "package main\n")); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := visibleFolders(t, files); strings.Join(got, ",") != "acme.good" {
		t.Fatalf("folders = %v; want only acme.good", got)
	}
	if loaded := bundle.Load(files, root+"/acme.good"); !loaded.Runnable() {
		t.Fatalf("installed folder problems = %v", loaded.Problems)
	}
}

func TestReinstallReplacesTheVersionInPlace(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	if err := Install(files, clock, root, "acme.good", contents("acme.good", "package main\n// v1\n")); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	if err := Install(files, clock, root, "acme.good", contents("acme.good", "package main\n// v2\n")); err != nil {
		t.Fatalf("second Install: %v", err)
	}
	source, _ := files.ReadFile(root + "/acme.good/" + bundle.SourceFile)
	if !strings.Contains(string(source), "v2") {
		t.Fatalf("source = %q; want v2", source)
	}
	if got := visibleFolders(t, files); strings.Join(got, ",") != "acme.good" {
		t.Fatalf("folders = %v; want only acme.good", got)
	}
}

func TestInstallRejectsAFolderWithoutSource(t *testing.T) {
	files := fake.NewFS()
	incomplete := map[string][]byte{bundle.ManifestFile: []byte(`{"apiVersion":"1.0","id":"acme.good","title":"T"}`)}
	if err := Install(files, fake.NewClock(), root, "acme.good", incomplete); err == nil {
		t.Fatal("Install accepted a folder without main.go")
	}
	if got := visibleFolders(t, files); len(got) != 0 {
		t.Fatalf("folders = %v; want none", got)
	}
}

func TestInstallRejectsManifestNamingAnotherID(t *testing.T) {
	files := fake.NewFS()
	err := Install(files, fake.NewClock(), root, "acme.good", contents("acme.other", "package main\n"))
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Install = %v; want an id mismatch", err)
	}
	if got := visibleFolders(t, files); len(got) != 0 {
		t.Fatalf("folders = %v; a failed install must leave nothing visible", got)
	}
}

func TestInstallFailureBeforePublishLeavesNoVisibleFolder(t *testing.T) {
	files := fake.NewFS()
	files.FailNext(fake.OpRename, fs.ErrPermission)
	err := Install(files, fake.NewClock(), root, "acme.good", contents("acme.good", "package main\n"))
	if err == nil {
		t.Fatal("Install succeeded although the rename failed")
	}
	if got := visibleFolders(t, files); len(got) != 0 {
		t.Fatalf("folders = %v; want none after a failed rename", got)
	}
}

func TestUninstallRemovesTheFolderAndToleratesAMissingOne(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	if err := Install(files, clock, root, "acme.good", contents("acme.good", "package main\n")); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := Uninstall(files, clock, root, "acme.good"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if got := visibleFolders(t, files); len(got) != 0 {
		t.Fatalf("folders = %v after uninstall", got)
	}
	if err := Uninstall(files, clock, root, "acme.good"); err != nil {
		t.Fatalf("second Uninstall = %v; want nil for a missing folder", err)
	}
}

func TestSweepRemovesInterruptedStagingAndTrash(t *testing.T) {
	files := fake.NewFS()
	files.Seed(root+"/.staging-acme.good-1/"+bundle.ManifestFile, []byte("{}"))
	files.Seed(root+"/.trash-acme.good-2/"+bundle.SourceFile, []byte("x"))
	if err := Sweep(files, root); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if got := visibleFolders(t, files); len(got) != 0 {
		t.Fatalf("leftovers after Sweep: %v", got)
	}
}

func TestInstallWritesEveryGoFileOfAMultiFileExtension(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	folder := contents("acme.multi", "package main\n")
	folder["helpers.go"] = []byte("package main\n// helpers\n")
	folder["data.go"] = []byte("package main\n// data\n")
	if err := Install(files, clock, root, "acme.multi", folder); err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, name := range []string{bundle.ManifestFile, bundle.SourceFile, "helpers.go", "data.go"} {
		if _, err := files.ReadFile(root + "/acme.multi/" + name); err != nil {
			t.Fatalf("%s was not installed: %v", name, err)
		}
	}
	if loaded := bundle.Load(files, root+"/acme.multi"); !loaded.Runnable() {
		t.Fatalf("installed folder problems = %v", loaded.Problems)
	}
}
