package install

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type cancelStagingFS struct {
	*fake.FS
	cancel  context.CancelFunc
	renames int
}

func (f *cancelStagingFS) Sync(name string) error {
	err := f.FS.Sync(name)
	if err == nil && strings.Contains(name, stagingPrefix) {
		f.cancel()
	}
	return err
}

func (f *cancelStagingFS) Rename(from, to string) error {
	f.renames++
	return f.FS.Rename(from, to)
}

func TestCancellationDuringStagingNeverPublishes(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "replacement"}[existing], func(t *testing.T) {
			files := fake.NewFS()
			clock := fake.NewClock()
			old := contents("acme.good", "package main\n// original\n")
			if existing {
				if err := Install(files, clock, root, "acme.good", old); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			port := &cancelStagingFS{FS: files, cancel: cancel}
			err := InstallContext(ctx, port, clock, root, "acme.good", contents("acme.good", "package main\n// candidate\n"))
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("install error = %v", err)
			}
			if port.renames != 0 {
				t.Fatalf("publication attempted %d renames", port.renames)
			}
			folders := visibleFolders(t, files)
			if existing {
				if strings.Join(folders, ",") != "acme.good" {
					t.Fatalf("folders = %v", folders)
				}
				for name, want := range old {
					got, err := files.ReadFile(root + "/acme.good/" + name)
					if err != nil || string(got) != string(want) {
						t.Fatalf("original %s changed: %q, %v", name, got, err)
					}
				}
			} else if len(folders) != 0 {
				t.Fatalf("cancelled creation left folders: %v", folders)
			}
			if got := bundle.Load(files, root+"/acme.good"); !existing && got.Runnable() {
				t.Fatal("cancelled candidate became runnable")
			}
		})
	}
}
