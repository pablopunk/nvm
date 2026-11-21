package supervisor

import (
	"context"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupervisedRefreshDenialClearsVisibleAndReopen(t *testing.T) {
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	root := filepath.Join(rig.deps.Home, "Desktop")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(root, "Screen Shot.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	child := servicedRuntime(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
	services := &revocableScreenshotsServices{ServiceHandler: child.services}
	child.services = services
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	ref := api.ActionRef{ID: "screenshots.grid"}
	if _, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: ref, Launch: api.LaunchContext{Refresh: true}}); err != nil {
		t.Fatal(err)
	}
	queue := make(chan func(), 8)
	invoke := func(callCtx context.Context, _ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		go func() {
			response, err := child.Call(callCtx, protocol.MethodRun, "", &in)
			var out api.Result
			var failure *api.Failure
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			} else {
				failure = response.Failure
				_ = json.Unmarshal(response.Data, &out)
			}
			out.SnapshotEvidence = response.SnapshotEvidence
			queue <- func() { done(out, failure) }
		}()
	}
	id := new(byte)
	cache := &extview.CommandSnapshots{}
	var refreshes atomic.Int32
	deps := extview.Deps{OnUI: func(fn func()) { queue <- fn }, CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(ext string, in api.Invocation, done func(api.Result, *api.Failure)) { invoke(ctx, ext, in, done) }, InvokeCommandContext: invoke, InvokeRefresh: func(c context.Context, ext string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if refreshes.Add(1) == 2 {
			services.denied.Store(true)
		}
		invoke(c, ext, in, done)
	}}
	h := &probePaletteHost{pushed: make(chan any, 3)}
	action := extview.CommandActionFor(deps, "local.screenshots", api.Run("screenshots.grid", "Screenshots", nil))
	wait := func() {
		t.Helper()
		select {
		case fn := <-queue:
			fn()
		case <-time.After(6 * time.Second):
			t.Fatal("child timed out")
		}
	}
	action.Run(h)
	wait()
	first := (<-h.pushed).(*palette.View)
	wait()
	if _, ok := cache.Get(id, ref); !ok {
		t.Fatal("no validated snapshot")
	}
	first.OnLeave()
	time.Sleep(6 * time.Second)
	action.Run(h)
	<-h.pushed
	wait()
	fresh := (<-h.pushed).(*palette.View)
	wait()
	if fresh.Title != "Access unavailable" {
		t.Fatalf("private title remained: %q", fresh.Title)
	}
	if _, ok := cache.Get(id, ref); ok {
		t.Fatal("snapshot remained after denied refresh")
	}
	wait()
	action.Run(h)
	select {
	case <-h.pushed:
		t.Fatal("reopen showed denied snapshot")
	default:
	}
	wait()
	select {
	case v := <-h.pushed:
		for _, row := range v.(*palette.View).Search("") {
			if row.Title == "Screen Shot.png" {
				t.Fatal("reopen showed denied private row")
			}
		}
	default:
	}
}
