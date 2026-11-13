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

func TestUnrelatedHandledDenialsPreserveSupervisedResult(t *testing.T) {
	rig := newServicesRig(t)
	path := filepath.Join(t.TempDir(), "allowed.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 48, 48))); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	child := servicedRuntime(t, "acme.snapshot_scope", rig, fixtureManifest("acme.snapshot_scope", api.CapabilityDesktopFiles))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var notified atomic.Int32
	response, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "open", Args: map[string]string{"path": path}}, ProtectSnapshot: true, OnSnapshotDenied: func(*api.SnapshotDenial) { notified.Add(1) }})
	if err != nil || response.Failure != nil || response.PermissionFailure != nil || notified.Load() != 0 {
		t.Fatalf("unrelated denial changed response: %v %+v %+v notifications=%d", err, response.Failure, response.PermissionFailure, notified.Load())
	}
	var result api.Result
	if err = json.Unmarshal(response.Data, &result); err != nil || result.View == nil || len(result.View.Items) != 1 || result.View.Items[0].Title != "Handled unrelated denials" || result.View.Items[0].Image == nil {
		t.Fatalf("handled result lost: %v %+v", err, result.View)
	}
	if response.SnapshotEvidence == nil || len(response.SnapshotEvidence.ThumbnailHashes) != 1 || response.SnapshotEvidence.FilesAuthorityDenied || len(response.SnapshotEvidence.DeniedPaths) != 0 {
		t.Fatalf("evidence %+v", response.SnapshotEvidence)
	}
}

func TestUnrelatedFileDenialKeepsValidatedWarmView(t *testing.T) {
	rig := newServicesRig(t)
	dir := t.TempDir()
	allowed := filepath.Join(dir, "allowed.png")
	probe := filepath.Join(dir, "probe.png")
	for _, path := range []string{allowed, probe} {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 40, 40))); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	fsys := &heldScreenshotFS{FS: rig.deps.Files, target: probe, entered: make(chan struct{}), release: make(chan struct{})}
	rig.deps.Files = fsys
	child := servicedRuntime(t, "acme.snapshot_scope", rig, fixtureManifest("acme.snapshot_scope", api.CapabilityDesktopFiles))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ref := api.ActionRef{ID: "open", Args: map[string]string{"path": allowed, "probe": probe}}
	first, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: ref})
	if err != nil {
		t.Fatal(err)
	}
	var result api.Result
	if err = json.Unmarshal(first.Data, &result); err != nil || result.View == nil {
		t.Fatal("prime failed")
	}
	result.View.Refresh = &api.Refresh{Immediate: true, Ref: &ref}
	id := new(byte)
	cache := &extview.CommandSnapshots{}
	cache.PutVerified(id, ref, *result.View, result.View.Items, first.SnapshotEvidence)
	if _, ok := cache.Get(id, ref); !ok {
		t.Fatal("verified row did not cache")
	}
	fsys.denied.Store(true)
	queue := make(chan func(), 4)
	invoke := func(c context.Context, _ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		go func() {
			r, err := child.Call(c, protocol.MethodRun, "", &in)
			var out api.Result
			var failure *api.Failure
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			} else {
				failure = r.Failure
				_ = json.Unmarshal(r.Data, &out)
				out.SnapshotEvidence = r.SnapshotEvidence
			}
			queue <- func() { done(out, failure) }
		}()
	}
	deps := extview.Deps{OnUI: func(fn func()) { queue <- fn }, CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(ext string, in api.Invocation, done func(api.Result, *api.Failure)) { invoke(ctx, ext, in, done) }, InvokeCommandContext: invoke}
	h := &probePaletteHost{pushed: make(chan any, 2)}
	extview.CommandActionFor(deps, "acme.snapshot_scope", api.Run("open", "Scope", ref.Args)).Run(h)
	page := (<-h.pushed).(*palette.View)
	select {
	case fn := <-queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("denial callback missing")
	}
	if page.Title != "Cached files" {
		t.Fatal("unrelated file denial cleared cached view")
	}
	if _, ok := cache.Get(id, ref); !ok {
		t.Fatal("unrelated file denial evicted snapshot")
	}
	select {
	case fn := <-queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("command did not finish")
	}
	returned := (<-h.pushed).(*palette.View)
	rows := returned.Search("")
	if len(rows) != 1 || rows[0].Title != "Handled unrelated denials" {
		t.Fatal("child-handled result changed")
	}
}
