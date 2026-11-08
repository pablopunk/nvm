package supervisor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestInstalledScreenshotsFixture(t *testing.T) {
	source, err := os.ReadFile(fixture(t, "local.screenshots"))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "c1b51da832fbede110e5169470aee321b7085337c42e1699de8f35a2aceb9a7a" {
		t.Fatal("fixture source changed")
	}
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	desktop := filepath.Join(rig.deps.Home, "Desktop")
	if err := os.MkdirAll(desktop, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 80; i++ {
		img := image.NewNRGBA(image.Rect(0, 0, 128+i%13, 90+i%17))
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				img.SetNRGBA(x, y, color.NRGBA{uint8(x * i), uint8(y*3 + i), uint8(x*y + i), 255})
			}
		}
		f, err := os.Create(filepath.Join(desktop, fmt.Sprintf("Screen Shot %03d.png", i)))
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
	r := servicedRuntime(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
	timing := &screenshotTiming{ServiceHandler: r.services}
	r.services = timing
	phase := "initial"
	started := time.Now()
	defer func() { timing.report(t, r, started, phase) }()
	run := func(refresh bool) protocol.Response {
		t.Helper()
		callStart := time.Now()
		occupied := len(r.slots) != 0
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		response, err := r.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: refresh}})
		t.Logf("screenshots call refresh=%t wall=%s slot_occupied_on_entry=%t slot_occupied_on_return=%t response_bytes=%d", refresh, time.Since(callStart), occupied, len(r.slots) != 0, len(response.Data))
		if err != nil || response.Failure != nil {
			t.Fatalf("run: %v %+v", err, response.Failure)
		}
		return response
	}
	first := run(false)
	var initial api.Result
	if err := json.Unmarshal(first.Data, &initial); err != nil || initial.View == nil || initial.View.Refresh == nil {
		t.Fatalf("initial refresh: %s %v", first.Data, err)
	}
	start := time.Now()
	phase = "refresh"
	response := run(true)
	elapsed := time.Since(start)
	var result api.Result
	if err := json.Unmarshal(response.Data, &result); err != nil || result.View == nil || len(result.View.Items) != 80 {
		t.Fatalf("result: %s %v", response.Data, err)
	}
	if len(response.Data) >= 1<<20 {
		t.Fatalf("serialized result %d bytes", len(response.Data))
	}
	t.Logf("80 items: %d serialized bytes in %s", len(response.Data), elapsed)
	phase = "cancelled-refresh"
	blocker := &blockingScreenshotsService{ServiceHandler: r.services, entered: make(chan struct{}), release: make(chan struct{})}
	r.services = blocker
	cancelled, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, err := r.Call(cancelled, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}})
		finished <- err
	}()
	select {
	case <-blocker.entered:
	case <-time.After(5 * time.Second):
		close(blocker.release)
		t.Fatal("refresh did not reach host service")
	}
	cancel()
	select {
	case err := <-finished:
		if err != context.Canceled {
			t.Fatalf("cancelled refresh: %v", err)
		}
	case <-time.After(5 * time.Second):
		close(blocker.release)
		t.Fatal("cancelled refresh did not return")
	}
	close(blocker.release)
	phase = "cached-after-cancellation"
	cached := run(false)
	var again api.Result
	if err := json.Unmarshal(cached.Data, &again); err != nil || again.View == nil || len(again.View.Items) != 80 {
		t.Fatalf("cached result: %v", err)
	}
}

type blockingScreenshotsService struct {
	ServiceHandler
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (s *blockingScreenshotsService) Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	s.once.Do(func() { close(s.entered); <-s.release })
	return s.ServiceHandler.Serve(ctx, call)
}
