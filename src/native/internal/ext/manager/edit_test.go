package manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEditAdmissionRejectsManagedChangesAndForeignTokens(t *testing.T) {
	for _, change := range []string{"reinstall", "disable-enable", "foreign"} {
		t.Run(change, func(t *testing.T) {
			h := newHarness(t)
			contents := fixtureContents(t, "acme.good")
			if err := h.m.Install("acme.good", contents, true); err != nil {
				t.Fatal(err)
			}
			files, rev, err := h.m.EditSnapshot("acme.good")
			if err != nil || len(files) == 0 {
				t.Fatalf("snapshot: %v", err)
			}
			switch change {
			case "reinstall":
				if err := h.m.Install("acme.good", contents, true); err != nil {
					t.Fatal(err)
				}
			case "disable-enable":
				if err := h.m.Disable("acme.good"); err != nil {
					t.Fatal(err)
				}
				if err := h.m.Enable("acme.good"); err != nil {
					t.Fatal(err)
				}
			case "foreign":
				h2 := newHarness(t)
				_, rev, err = h2.m.EditSnapshot("acme.good")
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := h.m.InstallEdit(context.Background(), "acme.good", contents, true, rev); !errors.Is(err, ErrEditConflict) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestEditAdmissionDetectsCollisionAndExternalChange(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	_, absent, err := h.m.EditSnapshot("acme.good")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install("acme.good", contents, false); err != nil {
		t.Fatal(err)
	}
	if _, err := h.m.InstallEdit(context.Background(), "acme.good", contents, true, absent); !errors.Is(err, ErrEditConflict) {
		t.Fatalf("collision: %v", err)
	}
	_, rev, err := h.m.EditSnapshot("acme.good")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, "acme.good", "main.go"), []byte("changed externally"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := h.m.InstallEdit(context.Background(), "acme.good", contents, true, rev); !errors.Is(err, ErrEditConflict) {
		t.Fatalf("external change: %v", err)
	}
}

func TestEditAdmissionCancellationAndUnrelatedFiles(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	if err := h.m.Install("acme.good", contents, false); err != nil {
		t.Fatal(err)
	}
	_, rev, err := h.m.EditSnapshot("acme.good")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.m.InstallEdit(ctx, "acme.good", contents, true, rev); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	path := filepath.Join(h.root, "acme.good", "notes.txt")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := h.m.InstallEdit(context.Background(), "acme.good", contents, true, rev); err == nil {
		t.Fatal("accepted unrelated file")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatalf("unrelated file: %q %v", data, err)
	}
	if _, _, err := h.m.EditSnapshot("acme.good"); err == nil {
		t.Fatal("snapshot accepted unrelated file")
	}
}

func TestEditAdmissionPublishesAndConsumesRevision(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	files, rev, err := h.m.EditSnapshot("acme.good")
	if err != nil || files != nil {
		t.Fatalf("absence: %v %v", files, err)
	}
	if _, err := h.m.InstallEdit(context.Background(), "acme.good", contents, false, rev); err != nil {
		t.Fatal(err)
	}
	if _, err := h.m.InstallEdit(context.Background(), "acme.good", contents, false, rev); !errors.Is(err, ErrEditConflict) {
		t.Fatalf("reused revision: %v", err)
	}
}

func TestEditFailedStageLeavesExistingFolderAndNoStaging(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	if err := h.m.Install("acme.good", contents, false); err != nil {
		t.Fatal(err)
	}
	before, rev, err := h.m.EditSnapshot("acme.good")
	if err != nil {
		t.Fatal(err)
	}
	invalid := map[string][]byte{"main.go": []byte("package main")}
	if _, err := h.m.InstallEdit(context.Background(), "acme.good", invalid, false, rev); err == nil {
		t.Fatal("invalid stage accepted")
	}
	after, _, err := h.m.EditSnapshot("acme.good")
	if err != nil || !equalEditFiles(before, after) {
		t.Fatalf("existing source changed: %v", err)
	}
	entries, err := os.ReadDir(h.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "acme.good" {
			t.Fatalf("leftover %q", entry.Name())
		}
	}
}
