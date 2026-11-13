package supervisor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// tailscaleStatusJSON is what `tailscale status --json` prints: this machine, an online peer and an offline one.
const tailscaleStatusJSON = `{
  "BackendState": "Running",
  "Self": {"ID": "n1", "HostName": "pablo-mac", "DNSName": "pablo-mac.tail1234.ts.net.", "OS": "macOS", "TailscaleIPs": ["100.64.0.1", "fd7a::1"], "Online": true},
  "Peer": {
    "k2": {"ID": "n2", "HostName": "nas", "DNSName": "nas.tail1234.ts.net.", "OS": "linux", "TailscaleIPs": ["100.64.0.2"], "Online": true},
    "k3": {"ID": "n3", "HostName": "old-laptop", "DNSName": "old-laptop.tail1234.ts.net.", "OS": "windows", "TailscaleIPs": ["100.64.0.3"], "Online": false}
  }
}`

// TestInstalledTailscaleListsDevicesOnlineFirstWithCopyActions runs the installed Tailscale Machines extension against
// a scripted `tailscale` and checks what its list shows: online devices first, this device marked, and a hostname and
// an IP the user can copy from each row.
func TestInstalledTailscaleListsDevicesOnlineFirstWithCopyActions(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.tailscale-machines")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := installedManifest(t, raw)
	commands := fake.NewCommands()
	commands.Script("/usr/bin/which", fake.HelperScript{Stdout: []byte("/opt/homebrew/bin/tailscale\n")})
	commands.Script("/opt/homebrew/bin/tailscale", fake.HelperScript{Stdout: []byte(tailscaleStatusJSON)})
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Commands = commands
	r := servicedRuntime(t, filepath.Join("installed", "local.tailscale-machines"), rig, manifest)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	items, err := r.Load(ctx, "devices")
	if err != nil {
		t.Fatalf("load devices: %v", err)
	}
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	want := []string{"nas", "pablo-mac", "old-laptop"}
	if len(titles) != len(want) {
		t.Fatalf("titles = %v, want %v", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("titles = %v, want %v", titles, want)
		}
	}
	self := items[1]
	if self.Primary == nil || self.Primary.Title != "Copy .ts.net Hostname" || self.Primary.Text != "pablo-mac.tail1234.ts.net" {
		t.Fatalf("primary action = %+v, want to copy the hostname without its trailing dot", self.Primary)
	}
	if len(self.Actions) < 2 || self.Actions[0].Title != "Copy IP" || self.Actions[0].Text != "100.64.0.1" || self.Actions[1].Title != "Copy IPv6" {
		t.Fatalf("actions = %+v, want Copy IP (the IPv4) then Copy IPv6", self.Actions)
	}
	if len(self.Accessories) != 2 || self.Accessories[0].Text != "This device" || self.Accessories[1].Text != "Online" {
		t.Fatalf("accessories = %+v, want This device and Online", self.Accessories)
	}
	if offline := items[2]; len(offline.Accessories) != 1 || offline.Accessories[0].Text != "Offline" {
		t.Fatalf("offline accessories = %+v, want Offline", offline.Accessories)
	}
}

func installedManifest(t *testing.T, raw []byte) api.Manifest {
	t.Helper()
	var manifest api.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	return manifest
}
