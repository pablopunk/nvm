package extwindow

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"testing"
)

func TestWindowActionTypedHostBindingUsesFakeSurfaces(t *testing.T) {
	clock := fake.NewClock()
	opener := &fakeOpener{}
	windows := New(Deps{Clock: clock, Open: opener, Page: func(string, api.View, search.Host) *palette.View { return &palette.View{Title: "Window"} }})
	host := services.New(services.Deps{Clock: clock}, api.Manifest{Identity: api.Identity{ID: "acme.window"}})
	defer host.Close()
	view := api.ListView("Window")
	view.ID = "sample"
	create := api.Action{Kind: api.ActionWindow, Op: "create", Window: &api.WindowRef{View: &view}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := host.WindowAction(ctx, create, windows); !errors.Is(err, context.Canceled) || len(opener.surfaces) != 0 {
		t.Fatal("cancelled action opened a window")
	}
	if err := host.WindowAction(context.Background(), create, windows); err != nil {
		t.Fatal(err)
	}
	surface := opener.surfaces[0]
	action := api.Action{Kind: api.ActionWindow, Window: &api.WindowRef{ID: "sample"}}
	for _, op := range []string{"hide", "show", "toggle", "toggle"} {
		action.Op = op
		if err := host.WindowAction(context.Background(), action, windows); err != nil {
			t.Fatal(err)
		}
	}
	if len(opener.surfaces) != 1 || !surface.visible {
		t.Fatal("existing surface behavior changed")
	}
	if err := windows.Show("acme.other", api.WindowRef{ID: "sample"}); !errors.Is(err, ErrNotOpen) {
		t.Fatal("window crossed extension namespace")
	}
	action.Op = "close"
	if err := host.WindowAction(context.Background(), action, windows); err != nil || len(windows.windows) != 0 {
		t.Fatal("close did not remove fake window")
	}
	if err := host.WindowAction(context.Background(), create, windows); err != nil {
		t.Fatal(err)
	}
	surface = opener.surfaces[1]
	_ = host.Close()
	action.Op = "hide"
	if err := host.WindowAction(context.Background(), action, windows); !errors.Is(err, api.ErrNotAvailable) || !surface.visible {
		t.Fatal("closed host affected window")
	}
}
