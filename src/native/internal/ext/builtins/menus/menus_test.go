package menus

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type fakeMenus struct {
	snapshot api.MenuSnapshot
	err      error
	handles  []string
}

func (m *fakeMenus) Snapshot() (api.MenuSnapshot, error) { return m.snapshot, m.err }
func (m *fakeMenus) Invoke(handle string) error          { m.handles = append(m.handles, handle); return m.err }

type fakeDesktop struct {
	api.Desktop
	menus *fakeMenus
}

func (d fakeDesktop) Menus() api.Menus { return d.menus }

type fakeNavigation struct{ api.Navigation }

func (fakeNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

type fakeContext struct {
	api.Context
	menus *fakeMenus
}

func (c fakeContext) Desktop() api.Desktop     { return fakeDesktop{menus: c.menus} }
func (fakeContext) Navigation() api.Navigation { return fakeNavigation{} }

func TestCommandAvailability(t *testing.T) {
	for _, available := range []bool{false, true} {
		commands, err := New(available).Commands(fakeContext{})
		if err != nil || len(commands) != map[bool]int{false: 0, true: 1}[available] {
			t.Fatalf("commands: %+v %v", commands, err)
		}
		if available && (commands[0].ID != CommandID || !commands[0].Customizable) {
			t.Fatalf("command: %+v", commands[0])
		}
	}
}
func TestCachedViewAndRefresh(t *testing.T) {
	m := &fakeMenus{snapshot: api.MenuSnapshot{Loading: true, Items: []api.MenuItem{{Handle: "opaque", Title: "Copy", Path: []string{"Edit"}, Shortcut: "⌘C", Enabled: true, Checked: true}, {Handle: "disabled", Title: "Paste"}}}}
	ctx := fakeContext{menus: m}
	result, err := New(true).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: CommandID}})
	if err != nil || result.View == nil {
		t.Fatalf("open: %+v %v", result, err)
	}
	v := result.View
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	if !v.IsLoading || len(v.Items) != 2 || v.Items[0].Subtitle != "Edit" || len(v.Items[0].Accessories) != 2 || v.Items[0].Primary.DismissAfterRun || v.Items[0].Primary.Shortcut != "" || !v.Items[1].Disabled || v.Items[1].Primary != nil {
		t.Fatalf("view: %+v", v)
	}
	if v.Refresh == nil || !v.Refresh.Immediate || v.Refresh.IntervalMs != 0 || v.Refresh.Ref.ID != RefreshID {
		t.Fatalf("refresh: %+v", v.Refresh)
	}
	m.snapshot = api.MenuSnapshot{}
	result, err = New(true).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: RefreshID}})
	if err != nil || result.Patch == nil || result.Patch.Mode != api.PatchReplace || result.Patch.IsLoading == nil || *result.Patch.IsLoading || result.Navigation != "" || result.Patch.EmptyView == nil {
		t.Fatalf("patch: %+v %v", result, err)
	}
}
func TestInvokeAndFailures(t *testing.T) {
	m := &fakeMenus{}
	ctx := fakeContext{menus: m}
	invocation := api.Invocation{Ref: api.ActionRef{ID: invokeID, Args: map[string]string{"handle": "opaque"}}}
	result, err := New(true).RunAction(ctx, invocation)
	if err != nil || len(m.handles) != 1 || m.handles[0] != "opaque" || result.View != nil {
		t.Fatalf("invoke: %+v %v %+v", result, err, m.handles)
	}
	m.err = api.ErrPermission
	_, err = New(true).RunAction(ctx, invocation)
	if !errors.Is(err, api.ErrPermission) || len(m.handles) != 2 {
		t.Fatalf("failure: %v %+v", err, m.handles)
	}
	view := menuView(api.MenuSnapshot{}, api.ErrPermission)
	if view.EmptyView.Title != "Accessibility access needed" {
		t.Fatalf("permission: %+v", view.EmptyView)
	}
}
