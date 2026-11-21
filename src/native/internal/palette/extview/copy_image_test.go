package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestCopyImageActionCarriesOriginAndInstallation(t *testing.T) {
	identity := new(byte)
	called := false
	deps := Deps{
		CommandSnapshotIdentity: func(id string) any {
			if id != "local.screenshots" {
				t.Fatalf("identity for %q", id)
			}
			return identity
		},
		CopyImage: func(_ search.Host, id, path string, got any) {
			called = true
			if id != "local.screenshots" || path != "/home/test/Desktop/shot.png" || got != identity {
				t.Fatalf("copy = %q %q %v", id, path, got)
			}
		},
	}
	s := NewSession(deps, "local.screenshots", api.View{})
	action := s.action(api.Action{Kind: api.ActionCopyImage, Path: "/home/test/Desktop/shot.png"}, "shot")
	action.Run(&fakeHost{})
	if !called {
		t.Fatal("copy was not dispatched")
	}
}
