package supervisor

import (
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestScreenshotsCopyActionUsesDiscoveredFile(t *testing.T) {
	requireUnixProcesses(t)
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	desktop := filepath.Join(rig.deps.Home, "Desktop")
	if err := os.MkdirAll(desktop, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(desktop, "Screen Shot test.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	r := servicedRuntime(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := r.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}})
	if err != nil || response.Failure != nil {
		t.Fatalf("run: %v %+v", err, response.Failure)
	}
	var result api.Result
	if err := json.Unmarshal(response.Data, &result); err != nil || result.View == nil || len(result.View.Items) != 1 {
		t.Fatalf("view: %s (%v)", response.Data, err)
	}
	for _, action := range result.View.Items[0].Actions {
		if action.Kind == api.ActionCopyImage {
			if action.Path != path {
				t.Fatalf("copy path %q, want %q", action.Path, path)
			}
			return
		}
	}
	t.Fatal("screenshot has no copy image action")
}
