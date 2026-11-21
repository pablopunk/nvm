package services

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestOpenExternalFollowsTheElectronLinkRule(t *testing.T) {
	_, deps := newDesktopHost(t)
	deps.Shell = fake.NewShell()
	h := desktopHandler(t, deps, api.CapabilityDesktopFiles)
	for _, link := range []string{"https://example.com", "mailto:someone@example.com"} {
		mustOK(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpOpenExternal, protocol.URLArgs{URL: link}))
	}
	for _, link := range []string{"javascript:alert(1)", "file:///etc/passwd", "https://", "nevermind://x"} {
		requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpOpenExternal, protocol.URLArgs{URL: link}), api.CodeInvalid, "mailto")
	}
}
