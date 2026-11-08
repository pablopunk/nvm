package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The fixture is an unchanged copy of the installed Downloads Big Files extension.
func TestDownloadsQuickLookFollowsHostCapability(t *testing.T) {
	requireUnixProcesses(t)
	for _, supported := range []bool{false, true} {
		t.Run(map[bool]string{false: "unsupported", true: "supported"}[supported], func(t *testing.T) {
			rig := newServicesRig(t)
			rig.deps.Home = t.TempDir()
			downloads := filepath.Join(rig.deps.Home, "Downloads")
			if err := os.MkdirAll(downloads, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(downloads, "example.txt")
			if err := os.WriteFile(path, []byte("example"), 0600); err != nil {
				t.Fatal(err)
			}
			if supported {
				rig.deps.Capabilities = platform.NewCapabilities(platform.CapabilityShellQuickLook)
			}
			r := servicedRuntime(t, "local.downloads-big-files", rig, fixtureManifest("local.downloads-big-files", api.CapabilityDesktopFiles, api.CapabilitySystem))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			items, err := r.Load(ctx, "big-files")
			if err != nil || len(items) != 1 {
				t.Fatalf("load = %+v (%v), want one item", items, err)
			}
			found := false
			for _, action := range items[0].Actions {
				if action.Kind == api.ActionQuickLook {
					found = true
					if action.Path != path {
						t.Fatalf("quick look path = %q, want %q", action.Path, path)
					}
				}
			}
			if found != supported {
				t.Fatalf("quick look present = %v, want %v; actions %+v", found, supported, items[0].Actions)
			}
		})
	}
}
