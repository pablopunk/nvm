package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type changingDuringStageFS struct {
	platform.FS
	platform.SourceDirectories
	change func() error
}

func (f *changingDuringStageFS) Sync(name string) error {
	if err := f.FS.Sync(name); err != nil {
		return err
	}
	if f.change != nil && strings.HasPrefix(filepath.Base(filepath.Dir(name)), ".staging-") {
		change := f.change
		f.change = nil
		return change()
	}
	return nil
}

func TestEditRevalidatesChangesAndCreationDuringStaging(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "creation", true: "edit"}[existing], func(t *testing.T) {
			h := newHarness(t)
			contents := fixtureContents(t, "acme.good")
			if existing {
				if err := h.m.Install("acme.good", contents, false); err != nil {
					t.Fatal(err)
				}
			}
			_, revision, err := h.m.EditSnapshot("acme.good")
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(h.root, "acme.good")
			external := []byte("package main\n// external edit during staging\n")
			files := h.m.ports.Files
			h.m.ports.Files = &changingDuringStageFS{
				FS: files, SourceDirectories: files.(platform.SourceDirectories),
				change: func() error {
					if err := os.MkdirAll(target, 0700); err != nil {
						return err
					}
					if !existing {
						if err := os.WriteFile(filepath.Join(target, "extension.json"), contents["extension.json"], 0600); err != nil {
							return err
						}
					}
					return os.WriteFile(filepath.Join(target, "main.go"), external, 0600)
				},
			}
			published, err := h.m.InstallEdit(context.Background(), "acme.good", contents, false, revision)
			if published || err == nil {
				t.Fatalf("published = %v, error = %v", published, err)
			}
			got, err := os.ReadFile(filepath.Join(target, "main.go"))
			if err != nil || string(got) != string(external) {
				t.Fatalf("external source lost: %q, %v", got, err)
			}
			entries, err := os.ReadDir(h.root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "acme.good" {
				t.Fatalf("staging not cleaned: %v, %v", entries, err)
			}
		})
	}
}
