package services

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// stubFrontmost reports one app in front, or none.
type stubFrontmost struct {
	app   platform.WindowApp
	found bool
}

func (s stubFrontmost) Frontmost(context.Context) (platform.WindowApp, bool, error) {
	return s.app, s.found, nil
}

func frontmostOf(t *testing.T, h *Handler) (api.App, protocol.ServiceReply) {
	t.Helper()
	reply := serve(t, h, protocol.ServiceDesktopApps, protocol.OpFrontmost, nil)
	var app api.App
	if reply.Failure == nil {
		if err := jsonInto(mustOK(t, reply), &app); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return app, reply
}

func TestFrontmostNamesTheIndexedAppWithItsPath(t *testing.T) {
	host, deps := newDesktopHost(t, platform.CapabilityFrontmostApp)
	notes := testpath.Abs("Applications", "Notes.app")
	host.shell.SetApps([]platform.App{{Name: "Notes", ID: "com.example.notes", Path: notes}}, nil)
	deps.Frontmost = stubFrontmost{app: platform.WindowApp{PID: 42, BundleID: "COM.example.notes"}, found: true}
	h := desktopHandler(t, deps, api.CapabilityDesktopApps)
	app, _ := frontmostOf(t, h)
	if app.ID != "com.example.notes" || app.Name != "Notes" || app.Path != notes {
		t.Fatalf("frontmost = %+v, want the indexed Notes app with its path", app)
	}
}

func TestFrontmostOutsideTheIndexKeepsItsBundleIDWithoutPath(t *testing.T) {
	_, deps := newDesktopHost(t, platform.CapabilityFrontmostApp)
	deps.Frontmost = stubFrontmost{app: platform.WindowApp{PID: 7, BundleID: "com.example.unlisted"}, found: true}
	h := desktopHandler(t, deps, api.CapabilityDesktopApps)
	app, _ := frontmostOf(t, h)
	if app.ID != "com.example.unlisted" || app.Name != "com.example.unlisted" || app.Path != "" {
		t.Fatalf("frontmost = %+v, want the bundle id as name and no path", app)
	}
}

func TestFrontmostNeedsTheDeclarationTheCapabilityAndAnAppInFront(t *testing.T) {
	_, deps := newDesktopHost(t, platform.CapabilityFrontmostApp)
	deps.Frontmost = stubFrontmost{app: platform.WindowApp{BundleID: "com.example.notes"}, found: true}
	requireCode(t, serve(t, desktopHandler(t, deps), protocol.ServiceDesktopApps, protocol.OpFrontmost, nil), api.CodePermission, "desktop.apps")

	_, unsupported := newDesktopHost(t)
	unsupported.Frontmost = deps.Frontmost
	requireCode(t, serve(t, desktopHandler(t, unsupported, api.CapabilityDesktopApps), protocol.ServiceDesktopApps, protocol.OpFrontmost, nil), api.CodeUnavailable, "not available")

	_, empty := newDesktopHost(t, platform.CapabilityFrontmostApp)
	empty.Frontmost = stubFrontmost{}
	requireCode(t, serve(t, desktopHandler(t, empty, api.CapabilityDesktopApps), protocol.ServiceDesktopApps, protocol.OpFrontmost, nil), api.CodeNotFound, "in front")
}
