package supervisor

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// TestScreenshotsNoisyBudgetProbe runs the unchanged Screenshots extension on 80 high-detail images.
func TestScreenshotsNoisyBudgetProbe(t *testing.T) {
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	root := filepath.Join(rig.deps.Home, "Desktop")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	var x uint32 = 1
	for i := 0; i < 80; i++ {
		img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
		for p := 0; p < len(img.Pix); p += 4 {
			for j := 0; j < 3; j++ {
				x ^= x << 13
				x ^= x >> 17
				x ^= x << 5
				img.Pix[p+j] = byte(x)
			}
			img.Pix[p+3] = 255
		}
		f, err := os.Create(filepath.Join(root, fmt.Sprintf("Screen Shot %03d.png", i)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	worker, wire := newTimingWorker()
	child := servicedRuntimeWithWorker(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles), worker)
	defer wire.report(t)
	timing := &screenshotTiming{ServiceHandler: child.services}
	child.services = timing
	start := time.Now()
	defer timing.report(t, child, start, "noisy-refresh")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}})
	if err != nil || response.Failure != nil {
		t.Fatalf("80-image refresh did not fit protocol: err=%v failure=%+v", err, response.Failure)
	}
	t.Logf("response bytes=%d limit=%d", len(response.Data), protocol.MaxLineBytes)
}
