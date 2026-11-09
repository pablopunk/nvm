package manager

import "testing"

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
