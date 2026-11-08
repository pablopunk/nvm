package manager

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	clipbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

func TestCopyImageRequiresCurrentEnabledFilesAuthority(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	contents[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"acme.good","title":"Good","capabilities":["desktop.files"]}`)
	if err := h.m.Install("acme.good", contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	identity := h.m.SnapshotIdentity("acme.good")
	if identity == nil || !h.m.MayCopyImage("acme.good", identity, false) {
		t.Fatal("enabled files extension cannot copy")
	}
	if h.m.MayCopyImage("acme.good", nil, true) || h.m.MayCopyImage("acme.missing", identity, true) {
		t.Fatal("missing identity or extension granted copy")
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if h.m.MayCopyImage("acme.good", identity, false) {
		t.Fatal("disabled extension retained authority")
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if h.m.MayCopyImage("acme.good", identity, false) {
		t.Fatal("old view retained authority after enable")
	}
}

func TestCopyImageRejectsUndeclaredFilesCapability(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	contents[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"acme.good","title":"Good","capabilities":["desktop.clipboard"]}`)
	if err := h.m.Install("acme.good", contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	if h.m.MayCopyImage("acme.good", h.m.SnapshotIdentity("acme.good"), true) {
		t.Fatal("clipboard capability must not grant file reads")
	}
}

func TestClipboardBuiltinOnlyCopiesOwnedPictures(t *testing.T) {
	h := newHarness(t, func(opts *Options) { opts.BuiltIns = []api.Extension{clipbuiltin.New(nil)} })
	if !h.m.MayCopyImage(clipbuiltin.ID, nil, true) || h.m.MayCopyImage(clipbuiltin.ID, nil, false) {
		t.Fatal("clipboard builtin must be limited to owned pictures")
	}
}
