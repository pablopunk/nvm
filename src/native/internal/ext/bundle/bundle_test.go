package bundle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

var root = testpath.Abs("ext")

const validManifest = `{"apiVersion":"1.0","id":"acme.good","title":"Good","capabilities":["desktop.apps","future.thing"]}`

func seedExtension(files *fake.FS, folder, manifest, source string) {
	files.Seed(filepath.Join(root, folder, ManifestFile), []byte(manifest))
	files.Seed(filepath.Join(root, folder, SourceFile), []byte(source))
}

func TestLoadReadsAValidExtensionWithoutRunningIt(t *testing.T) {
	files := fake.NewFS()
	seedExtension(files, "acme.good", validManifest, "package main\n")
	bundle := Load(files, filepath.Join(root, "acme.good"))
	if !bundle.Runnable() {
		t.Fatalf("Problems = %v; want none", bundle.Problems)
	}
	if bundle.Manifest.ID != "acme.good" || len(bundle.Source) == 0 || bundle.Hash == "" {
		t.Fatalf("bundle = %+v; want manifest, source and hash", bundle)
	}
	if got := bundle.Manifest.UnknownCapabilities(); len(got) != 1 || got[0] != "future.thing" {
		t.Fatalf("UnknownCapabilities = %v; want future.thing", got)
	}
}

func TestLoadReportsProblemsPerFolder(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		source   string
		want     string
	}{
		{"bad json", `{"id":`, "package main\n", "not valid JSON"},
		{"reserved id", `{"apiVersion":"1.0","id":"nevermind.clipboard","title":"X"}`, "package main\n", "reserved"},
		{"newer minor", `{"apiVersion":"1.9","id":"acme.good","title":"X"}`, "package main\n", "update Nevermind"},
		{"missing source", validManifest, "", "main.go is missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := fake.NewFS()
			if tt.source == "" {
				files.Seed(filepath.Join(root, "acme.good", ManifestFile), []byte(tt.manifest))
			} else {
				seedExtension(files, "acme.good", tt.manifest, tt.source)
			}
			bundle := Load(files, filepath.Join(root, "acme.good"))
			if bundle.Runnable() || !strings.Contains(strings.Join(bundle.Problems, "\n"), tt.want) {
				t.Fatalf("Problems = %v; want one containing %q", bundle.Problems, tt.want)
			}
		})
	}
}

func TestLoadRejectsSourceOverTheBound(t *testing.T) {
	files := fake.NewFS()
	seedExtension(files, "acme.good", validManifest, strings.Repeat("x", MaxSourceBytes+1))
	if bundle := Load(files, filepath.Join(root, "acme.good")); bundle.Runnable() {
		t.Fatal("Load accepted source over MaxSourceBytes")
	}
}

func TestScanSkipsHiddenFoldersAndFlagsNameMismatch(t *testing.T) {
	files := fake.NewFS()
	seedExtension(files, "acme.good", validManifest, "package main\n")
	seedExtension(files, "acme.renamed", validManifest, "package main\n")
	seedExtension(files, ".staging-acme.good-1", validManifest, "package main\n")
	bundles, err := Scan(files, root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("Scan returned %d bundles; want 2 (hidden folder skipped)", len(bundles))
	}
	if good := bundleIn(t, bundles, filepath.Join(root, "acme.good")); !good.Runnable() {
		t.Fatalf("acme.good problems = %v", good.Problems)
	}
	if renamed := bundleIn(t, bundles, filepath.Join(root, "acme.renamed")); renamed.Runnable() {
		t.Fatal("folder acme.renamed holding id acme.good loaded as runnable")
	}
}

func TestScanOfMissingRootIsEmpty(t *testing.T) {
	bundles, err := Scan(fake.NewFS(), testpath.Abs("absent"))
	if err != nil || len(bundles) != 0 {
		t.Fatalf("Scan = %v, %v; want empty and no error", bundles, err)
	}
}

func TestHashChangesWhenEitherFileChanges(t *testing.T) {
	base := Hash([]byte("m"), []byte("s"))
	if Hash([]byte("m2"), []byte("s")) == base || Hash([]byte("m"), []byte("s2")) == base {
		t.Fatal("Hash did not change with the content")
	}
	if Hash([]byte("ms"), []byte("")) == base {
		t.Fatal("Hash must separate the two files")
	}
}

// bundleIn returns the bundle Scan made for dir, and fails the test when there is none, so a missing folder cannot
// pass as a runnable zero value.
func bundleIn(t *testing.T, bundles []Bundle, dir string) Bundle {
	t.Helper()
	for _, bundle := range bundles {
		if bundle.Dir == dir {
			return bundle
		}
	}
	t.Fatalf("Scan returned no bundle for %q", dir)
	return Bundle{}
}
