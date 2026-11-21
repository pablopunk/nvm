package manager

import (
	"bytes"
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"os"
	"path/filepath"
	"testing"
)

func TestPictureGrantExpiresWithItsEnabledInstallation(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	contents[bundle.ManifestFile] = bytes.ReplaceAll(contents[bundle.ManifestFile], []byte(`"desktop.apps"`), []byte(`"desktop.apps","desktop.files"`))
	if err := h.m.Install("acme.good", contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, []byte("pixels"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity := h.m.SnapshotIdentity("acme.good")
	grant, err := h.m.OpenPictureSource(context.Background(), "acme.good", path, identity)
	if err != nil {
		t.Fatal(err)
	}
	defer grant.Source.Close()
	if !grant.Current() {
		t.Fatal("new grant is not current")
	}
	if _, err := h.m.OpenPictureSource(context.Background(), "acme.good", path, new(byte)); !errors.Is(err, api.ErrPermission) {
		t.Fatalf("foreign identity = %v", err)
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if grant.Current() {
		t.Fatal("disabled installation retained its read grant")
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	if grant.Current() {
		t.Fatal("old read grant revived on re-enable")
	}
	if _, err := h.m.OpenPictureSource(context.Background(), "acme.good", path, identity); !errors.Is(err, api.ErrPermission) {
		t.Fatalf("old identity = %v", err)
	}
	next, err := h.m.OpenPictureSource(context.Background(), "acme.good", path, h.m.SnapshotIdentity("acme.good"))
	if err != nil {
		t.Fatal(err)
	}
	defer next.Source.Close()
	if !next.Current() {
		t.Fatal("new enabled installation could not read")
	}
	host := h.m.pictureHost("acme.good", h.m.SnapshotIdentity("acme.good"))
	if err := host.Close(); err != nil {
		t.Fatal(err)
	}
	if next.Current() {
		t.Fatal("closed service host retained a deferred publication grant")
	}
}
