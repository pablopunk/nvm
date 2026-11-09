package manager

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

func TestSnapshotIdentityChangesOnDisableAndReinstall(t *testing.T) {
	h := newHarness(t)
	const id = "acme.menubar"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	first := h.m.SnapshotIdentity(id)
	if first == nil {
		t.Fatal("enabled extension has no identity")
	}
	if err := h.m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if h.m.SnapshotIdentity(id) != nil {
		t.Fatal("disabled extension exposed identity")
	}
	if err := h.m.Enable(id); err != nil {
		t.Fatal(err)
	}
	second := h.m.SnapshotIdentity(id)
	if second == nil || second == first {
		t.Fatal("enable reused disabled installation identity")
	}
	if err := h.m.Uninstall(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	third := h.m.SnapshotIdentity(id)
	if third == nil || third == second || third == first {
		t.Fatal("reinstall reused prior identity")
	}
}

func TestSnapshotRevisionRequiresCurrentFilesAuthority(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.good")
	contents[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"acme.good","title":"Good","capabilities":["desktop.files"]}`)
	if err := h.m.Install("acme.good", contents, true); err != nil {
		t.Fatal(err)
	}
	identity := h.m.SnapshotIdentity("acme.good")
	revision, ok := h.m.SnapshotRevision("acme.good", identity)
	if !ok || revision == "" {
		t.Fatal("enabled installation has no revision")
	}
	if _, ok := h.m.SnapshotRevision("acme.good", []string{"not comparable"}); ok {
		t.Fatal("invalid identity was accepted")
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.m.SnapshotRevision("acme.good", identity); ok {
		t.Fatal("disabled installation retained authority")
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.m.SnapshotRevision("acme.good", identity); ok {
		t.Fatal("old identity survived re-enablement")
	}
}
