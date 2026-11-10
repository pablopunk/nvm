package install

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type failedTrashCleanup struct{ *fake.FS }

func (f failedTrashCleanup) Remove(name string) error {
	if strings.Contains(name, trashPrefix) {
		return errors.New("injected cleanup failure")
	}
	return f.FS.Remove(name)
}

func TestPublicationReportedWhenOldVersionCleanupFails(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	if err := Install(files, clock, root, "acme.good", contents("acme.good", "package main\n// old\n")); err != nil {
		t.Fatal(err)
	}
	published, err := InstallContextResult(context.Background(), failedTrashCleanup{files}, clock, root, "acme.good", contents("acme.good", "package main\n// new\n"))
	if !published || err == nil {
		t.Fatalf("published = %v, error = %v", published, err)
	}
	data, err := files.ReadFile(root + "/acme.good/main.go")
	if err != nil || string(data) != "package main\n// new\n" {
		t.Fatalf("published source = %q, %v", data, err)
	}
}
