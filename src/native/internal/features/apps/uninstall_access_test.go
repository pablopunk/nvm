package apps

import (
	"path"
	"testing"
)

type accessDecision struct{ allowed bool }

func (a *accessDecision) CanWrite(string) bool { return a.allowed }

func TestUninstallUsesEffectiveParentAccess(t *testing.T) {
	for _, tc := range []struct {
		name    string
		parent  string
		allowed bool
	}{
		{"shared root owned", "/Applications", true},
		{"user applications", path.Join(testHome, "Applications"), true},
		{"denied or nonsearchable or read only", "/Applications", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDarwinFixture(t, fixtureOpts{})
			f.tree.dir(tc.parent, 0)
			app := path.Join(tc.parent, "Example.app")
			f.tree.dir(app, 0)
			f.u.deps.DirectoryAccess = &accessDecision{allowed: tc.allowed}
			d := f.u.Discover(app)
			if d.Ready != tc.allowed {
				t.Fatalf("ready = %v, reason = %s", d.Ready, d.ReasonCode)
			}
		})
	}
}

func TestUninstallRefusesAccessRevokedAfterReview(t *testing.T) {
	f := newDarwinFixture(t, fixtureOpts{})
	access := &accessDecision{allowed: true}
	f.u.deps.DirectoryAccess = access
	d := f.u.Discover(testApp)
	if !d.Ready {
		t.Fatalf("review refused: %+v", d)
	}
	access.allowed = false
	values := map[string]bool{}
	for _, c := range d.Snapshot.Candidates {
		values[c.ID] = true
	}
	result := f.u.Trash(d.Snapshot, values)
	if len(result.Moved) != 0 || len(f.tree.trashed) != 0 || len(result.Untouched) == 0 || result.Untouched[0].Refusal.Code != "app-parent-not-writable" {
		t.Fatalf("result = %+v, trashed = %v", result, f.tree.trashed)
	}
}
