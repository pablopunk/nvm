package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// appWithCapabilities is an app whose one port set reports exactly the capabilities named.
func appWithCapabilities(t *testing.T, names ...platform.Capability) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := goldenPalette(t)
	p.ports.Capabilities = platform.NewCapabilities(names...)
	p.systemSvc = newSystemServices(p.ports, p.goos)
	m, err := p.newExtensionManager()
	if err != nil {
		t.Fatalf("newExtensionManager: %v", err)
	}
	t.Cleanup(m.Close)
	p.exts.mgr.Store(m)
	return p
}

// TestBuiltInCommandsFollowTheAppsCapabilities is the one-source check: a system command appears at the root only when
// the app's own port set reports its capability, so the extensions never read a different set (the rig's capabilities
// reach the built-in through the same ports).
func TestBuiltInCommandsFollowTheAppsCapabilities(t *testing.T) {
	without := appWithCapabilities(t, platform.CapabilitySystemSleep)
	for _, row := range commandRows(without, &rootInput{}, "restart") {
		if row.Title == "Restart Computer" {
			t.Fatal("Restart shown without its capability")
		}
	}
	with := appWithCapabilities(t, platform.CapabilitySystemSleep, platform.CapabilitySystemRestart)
	findRowTitled(t, commandRows(with, &rootInput{}, "restart"), "Restart Computer")
}
