package services

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func notifyCall(t *testing.T, h *Handler, title, body string) protocol.ServiceReply {
	t.Helper()
	return serve(t, h, protocol.ServiceSystem, protocol.OpNotify, protocol.NotifyArgs{Title: title, Body: body})
}

func notifyHost(t *testing.T) (*fake.Notifier, *Handler, Deps) {
	t.Helper()
	_, deps := newDesktopHost(t, platform.CapabilityNotifications)
	notifier := fake.NewNotifier()
	deps.Notifier = notifier
	return notifier, desktopHandler(t, deps, api.CapabilitySystem), deps
}

func TestNotifyShowsTheTitleAndBodyWhenDeclared(t *testing.T) {
	notifier, h, _ := notifyHost(t)
	mustOK(t, notifyCall(t, h, "Timer", "Tea is ready"))
	shown := notifier.Shown()
	if len(shown) != 1 || shown[0].Title != "Timer" || shown[0].Body != "Tea is ready" {
		t.Fatalf("shown = %+v, want one notification with the title and body", shown)
	}
}

func TestNotifyNeedsTheSystemDeclaration(t *testing.T) {
	notifier, _, deps := notifyHost(t)
	undeclared := desktopHandler(t, deps)
	requireCode(t, notifyCall(t, undeclared, "Timer", "Done"), api.CodePermission, "system")
	if len(notifier.Shown()) != 0 {
		t.Fatal("an undeclared notification reached the notifier")
	}
}

func TestNotifyIsUnavailableWithoutTheCapability(t *testing.T) {
	_, deps := newDesktopHost(t)
	deps.Notifier = fake.NewNotifier()
	h := desktopHandler(t, deps, api.CapabilitySystem)
	requireCode(t, notifyCall(t, h, "Timer", "Done"), api.CodeUnavailable, "not available")
}

func TestNotifyReportsARefusalAsPermission(t *testing.T) {
	notifier, h, _ := notifyHost(t)
	notifier.FailNext(fake.OpNotifierShow, platform.ErrDenied)
	requireCode(t, notifyCall(t, h, "Timer", "Done"), api.CodePermission, "turned off")
}

func TestNotifyRejectsAnEmptyTitleAndOversizedText(t *testing.T) {
	_, h, _ := notifyHost(t)
	requireCode(t, notifyCall(t, h, "", "Done"), api.CodeInvalid, "title")
	requireCode(t, notifyCall(t, h, strings.Repeat("t", maxNotificationTitle+1), "Done"), api.CodeInvalid, "title")
	requireCode(t, notifyCall(t, h, "Timer", strings.Repeat("b", maxNotificationBody+1)), api.CodeInvalid, "body")
}

func TestNotifyIsRateLimited(t *testing.T) {
	_, h, _ := notifyHost(t)
	for range notificationsPerMinute {
		mustOK(t, notifyCall(t, h, "Timer", "Done"))
	}
	requireCode(t, notifyCall(t, h, "Timer", "Done"), api.CodeQuota, "rate limited")
}
