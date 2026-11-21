package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

type testMenuPort struct{ calls int }

func (p *testMenuPort) Snapshot(ctx context.Context) (api.MenuSnapshot, error) {
	p.calls++
	if err := ctx.Err(); err != nil {
		return api.MenuSnapshot{}, err
	}
	return api.MenuSnapshot{Items: []api.MenuItem{{Handle: "opaque", Title: "Copy", Path: []string{"Edit"}, Shortcut: "⌘C", Enabled: true}}}, nil
}
func (p *testMenuPort) Invoke(ctx context.Context, handle string) error {
	p.calls++
	if err := ctx.Err(); err != nil {
		return err
	}
	if handle != "opaque" {
		return notFound("stale menu handle")
	}
	return nil
}
func TestMenusHandlerContract(t *testing.T) {
	_, deps := newDesktopHost(t)
	port := &testMenuPort{}
	deps.Menus = port
	requireCode(t, serve(t, desktopHandler(t, deps), protocol.ServiceDesktopMenus, protocol.OpSnapshot, nil), api.CodePermission, "desktop.menus")
	if port.calls != 0 {
		t.Fatal("denied call reached port")
	}
	h := desktopHandler(t, deps, api.CapabilityDesktopMenus)
	reply := serve(t, h, protocol.ServiceDesktopMenus, protocol.OpSnapshot, nil)
	data, err := json.Marshal(mustOK(t, reply))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"items":[{"handle":"opaque","title":"Copy","path":["Edit"],"shortcut":"⌘C","enabled":true,"checked":false}],"loading":false}` {
		t.Fatalf("snapshot: %s", data)
	}
	mustOK(t, serve(t, h, protocol.ServiceDesktopMenus, protocol.OpInvoke, protocol.HandleArgs{Handle: "opaque"}))
	requireCode(t, serve(t, h, protocol.ServiceDesktopMenus, protocol.OpInvoke, protocol.HandleArgs{Handle: "stale"}), api.CodeNotFound, "stale")
	requireCode(t, serve(t, h, protocol.ServiceDesktopMenus, protocol.OpInvoke, protocol.HandleArgs{}), api.CodeInvalid, "handle")
	requireCode(t, serve(t, h, protocol.ServiceDesktopMenus, protocol.OpInvoke, map[string]any{"handle": "opaque", "pid": 42}), api.CodeInvalid, "arguments")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.Serve(ctx, protocol.ServiceCall{Service: protocol.ServiceDesktopMenus, Op: protocol.OpSnapshot})
	if port.calls != 4 {
		t.Fatalf("canceled context did not reach port: %d", port.calls)
	}
	deps.Menus = nil
	requireCode(t, serve(t, desktopHandler(t, deps, api.CapabilityDesktopMenus), protocol.ServiceDesktopMenus, protocol.OpSnapshot, nil), api.CodeUnavailable, "not available")
}
