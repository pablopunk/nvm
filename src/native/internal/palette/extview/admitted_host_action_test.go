package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type revokeOnHideHost struct {
	fakeHost
	revoke func()
}

func (h *revokeOnHideHost) Hide() { h.revoke(); h.fakeHost.Hide() }

func TestDirectOpenRechecksAfterHide(t *testing.T) {
	identity := new(int)
	valid := true
	opened := 0
	deps := Deps{
		CommandSnapshotIdentity: func(string) any { return identity },
		AdmitAction: func(_ string, got any) (func() bool, error) {
			if got != identity {
				t.Fatalf("identity=%v", got)
			}
			return func() bool { return valid }, nil
		},
		OpenPath: func(string) { opened++ },
	}
	session := NewSession(deps, "test", api.View{})
	host := &revokeOnHideHost{revoke: func() { valid = false }}
	session.action(api.OpenPath("Open", "/fixture"), "").Run(host)
	if opened != 0 || len(host.toasts) == 0 {
		t.Fatalf("opened=%d toasts=%v", opened, host.toasts)
	}
}

func TestTrashStopsAtRevocationBetweenEffects(t *testing.T) {
	valid := true
	var paths []string
	deps := Deps{
		AdmitAction: func(string, any) (func() bool, error) { return func() bool { return valid }, nil },
		Trash:       func(path string) error { paths = append(paths, path); valid = false; return nil },
	}
	session := NewSession(deps, "test", api.View{})
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionTrash, Paths: []string{"/first", "/second"}}
	session.action(action, "").Run(host)
	if len(paths) != 1 || paths[0] != "/first" || len(host.toasts) == 0 {
		t.Fatalf("paths=%v toasts=%v", paths, host.toasts)
	}
}

func TestUnboundCrossExtensionFollowUpCannotReachHost(t *testing.T) {
	opened := 0
	deps := Deps{OpenPath: func(string) { opened++ }}
	session := NewSession(deps, "source", api.View{})
	host := &fakeHost{}
	action := api.OpenPath("Open", "/fixture")
	session.finish(host, "target", "", api.Result{Next: &action}, nil)
	if opened != 0 || len(host.toasts) == 0 {
		t.Fatalf("opened=%d toasts=%v", opened, host.toasts)
	}
}
