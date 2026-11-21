package manager

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPreviewActionRequiresItsOriginalEnabledInstallation(t *testing.T) {
	h := newHarness(t)
	shell := fake.NewShell()
	h.m.serviceDeps.Shell = shell
	h.m.serviceDeps.Capabilities = platform.NewCapabilities(platform.CapabilityShellQuickLook)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	identity := h.m.SnapshotIdentity("acme.good")
	path := filepath.Join(t.TempDir(), "a.pdf")
	run := func(actor any) error { return h.m.PreviewFileAction(context.Background(), "acme.good", path, actor) }
	if err := run(identity); err != nil {
		t.Fatal(err)
	}
	if err := run(new(byte)); err == nil {
		t.Fatal("another installation supplied authority")
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if err := run(identity); err == nil {
		t.Fatal("disabled action reached OS")
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	if err := run(identity); err == nil {
		t.Fatal("old action revived after enable")
	}
	if err := run(h.m.SnapshotIdentity("acme.good")); err != nil {
		t.Fatal("new installation retained closed services:", err)
	}
	if len(shell.Calls()) != 2 {
		t.Fatalf("effects=%v", shell.Calls())
	}
	if !h.m.ActionSupported(api.QuickLook("Preview", path)) {
		t.Fatal("wired operation is not discoverable")
	}
}
