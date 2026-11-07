package supervisor

import (
	"context"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScreenshotsMixedReadableAndDeniedPreservesOtherRows(t *testing.T) {
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	root := filepath.Join(rig.deps.Home, "Desktop")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	denied := filepath.Join(root, "Screen Shot denied.png")
	allowed := filepath.Join(root, "Screen Shot allowed.png")
	for i, path := range []string{denied, allowed} {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewNRGBA(image.Rect(0, 0, 48, 48))
		img.SetNRGBA(2, 2, color.NRGBA{R: uint8(90 + i), A: 255})
		if err = png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	fsys := &heldScreenshotFS{FS: rig.deps.Files, target: denied, entered: make(chan struct{}), release: make(chan struct{})}
	rig.deps.Files = fsys
	child := servicedRuntime(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ref := api.ActionRef{ID: "screenshots.grid"}
	initial, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: ref, Launch: api.LaunchContext{Refresh: true}})
	if err != nil {
		t.Fatal(err)
	}
	var primed api.Result
	if err = json.Unmarshal(initial.Data, &primed); err != nil || primed.View == nil || len(primed.View.Items) != 2 {
		t.Fatal("prime failed")
	}
	primed.View.Refresh = &api.Refresh{Immediate: true, Ref: &ref}
	id := new(byte)
	cache := &extview.CommandSnapshots{}
	cache.PutVerified(id, ref, *primed.View, primed.View.Items, initial.SnapshotEvidence)
	if _, ok := cache.Get(id, ref); !ok {
		t.Fatal("no validated snapshot")
	}
	fsys.denied.Store(true)
	queue := make(chan func(), 8)
	invoke := func(c context.Context, _ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		go func() {
			response, err := child.Call(c, protocol.MethodRun, "", &in)
			var out api.Result
			var failure *api.Failure
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			} else {
				failure = response.Failure
				_ = json.Unmarshal(response.Data, &out)
				out.SnapshotEvidence = response.SnapshotEvidence
			}
			queue <- func() { done(out, failure) }
		}()
	}
	deps := extview.Deps{OnUI: func(fn func()) { queue <- fn }, CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(ext string, in api.Invocation, done func(api.Result, *api.Failure)) { invoke(ctx, ext, in, done) }, InvokeCommandContext: invoke, InvokeRefresh: func(context.Context, string, api.Invocation, func(api.Result, *api.Failure)) {}}
	h := &probePaletteHost{pushed: make(chan any, 2)}
	action := extview.CommandActionFor(deps, "local.screenshots", api.Run("screenshots.grid", "Screenshots", nil))
	action.Run(h)
	pending := (<-h.pushed).(*palette.View)
	wait := func() {
		t.Helper()
		select {
		case fn := <-queue:
			fn()
		case <-time.After(5 * time.Second):
			t.Fatal("child timed out")
		}
	}
	wait()
	if pending.Title != "Access unavailable" {
		t.Fatal("old pixels not cleared")
	}
	wait()
	fresh := (<-h.pushed).(*palette.View)
	check := func(page *palette.View) {
		t.Helper()
		rows := page.Search("")
		if len(rows) != 2 {
			t.Fatalf("lost independently authorized row: %+v", rows)
		}
		for _, row := range rows {
			if row.ID == allowed && len(row.Actions) == 0 {
				t.Fatal("allowed actions lost")
			}
			if row.ID == denied && len(row.Actions) != 0 {
				t.Fatal("denied row retained file actions")
			}
		}
	}
	check(fresh)
	if _, ok := cache.Get(id, ref); ok {
		t.Fatal("denied run left old snapshot")
	}
	action.Run(h)
	select {
	case <-h.pushed:
		t.Fatal("reopen reused denied snapshot")
	default:
	}
	wait()
	check((<-h.pushed).(*palette.View))
}
