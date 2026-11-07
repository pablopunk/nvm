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

type revocableScreenshotsServices struct {
	ServiceHandler
	denied atomic.Bool
}

func (s *revocableScreenshotsServices) Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	if s.denied.Load() && call.Service == protocol.ServiceDesktopFiles {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodePermission, Message: "files revoked"}}
	}
	return s.ServiceHandler.Serve(ctx, call)
}
func TestSupervisedScreenshotsReportsSwallowedFileDenial(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	in := &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}}
	first, err := child.Call(ctx, protocol.MethodRun, "", in)
	if err != nil || first.Failure != nil {
		t.Fatalf("prime %v %+v", err, first.Failure)
	}
	services.denied.Store(true)
	in.Launch.Refresh = false
	second, err := child.Call(ctx, protocol.MethodRun, "", in)
	if err != nil || second.PermissionFailure == nil || second.PermissionFailure.Code != api.CodePermission {
		t.Fatalf("denial was lost: %v %+v", err, second.PermissionFailure)
	}
}

func TestSupervisedScreenshotsReportsOSReadDenial(t *testing.T) {
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	root := filepath.Join(rig.deps.Home, "Desktop")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Screen Shot.png")
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
	fsys := &heldScreenshotFS{FS: rig.deps.Files, target: path, entered: make(chan struct{}), release: make(chan struct{})}
	rig.deps.Files = fsys
	child := servicedRuntime(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	in := &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}}
	first, err := child.Call(ctx, protocol.MethodRun, "", in)
	if err != nil || first.Failure != nil {
		t.Fatalf("prime %v %+v", err, first.Failure)
	}
	fsys.denied.Store(true)
	in.Launch.Refresh = false
	in.ProtectSnapshot = true
	second, err := child.Call(ctx, protocol.MethodRun, "", in)
	if err != nil || second.PermissionFailure == nil || second.PermissionFailure.Code != api.CodePermission {
		t.Fatalf("OS denial was lost: %v %+v", err, second.PermissionFailure)
	}
}

func TestSupervisedScreenshotsWarmOpenRunsChangedCommand(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ref := api.ActionRef{ID: "screenshots.grid"}
	first, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: ref, Launch: api.LaunchContext{Refresh: true}})
	if err != nil {
		t.Fatal(err)
	}
	var result api.Result
	if err = json.Unmarshal(first.Data, &result); err != nil || result.View == nil || len(result.View.Items) != 1 {
		t.Fatalf("prime %v", err)
	}
	result.View.Refresh = &api.Refresh{Immediate: true, Ref: &ref}
	id := new(byte)
	cache := &extview.CommandSnapshots{}
	cache.PutVerified(id, ref, *result.View, result.View.Items, first.SnapshotEvidence)
	raw, _ := json.Marshal(protocol.KeyValueArgs{Key: "screenshots-files-v2", Value: json.RawMessage(`[]`)})
	reply := child.services.Serve(ctx, protocol.ServiceCall{Service: protocol.ServiceStorage, Op: protocol.OpSet, Args: raw})
	if reply.Failure != nil {
		t.Fatal(reply.Failure)
	}
	queue := make(chan func(), 1)
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
	deps := extview.Deps{CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(ext string, in api.Invocation, done func(api.Result, *api.Failure)) { invoke(ctx, ext, in, done) }, InvokeCommandContext: invoke}
	h := &probePaletteHost{pushed: make(chan any, 2)}
	extview.CommandActionFor(deps, "local.screenshots", api.Run("screenshots.grid", "Screenshots", nil)).Run(h)
	old := (<-h.pushed).(*palette.View)
	if rows := old.Search(""); len(rows) != 1 {
		t.Fatal("snapshot did not appear")
	}
	select {
	case fn := <-queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("actual command did not run")
	}
	fresh := (<-h.pushed).(*palette.View)
	if rows := fresh.Search(""); len(rows) != 0 {
		t.Fatalf("changed command result ignored: %+v", rows)
	}
	if _, ok := cache.Get(id, ref); ok {
		t.Fatal("superseded snapshot remained")
	}
}

type denyThenHoldServices struct {
	ServiceHandler
	calls            atomic.Int32
	entered, release chan struct{}
}

func (s *denyThenHoldServices) Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	if call.Service == protocol.ServiceDesktopFiles {
		n := s.calls.Add(1)
		if n == 1 {
			return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodePermission, Message: "revoked"}}
		}
		if n == 2 {
			close(s.entered)
			<-s.release
		}
	}
	return s.ServiceHandler.Serve(ctx, call)
}
func TestSupervisedDenialReturnsBeforeLaterBlockedIO(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	in := &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}}
	if _, err := child.Call(ctx, protocol.MethodRun, "", in); err != nil {
		t.Fatal(err)
	}
	wrapper := &denyThenHoldServices{ServiceHandler: child.services, entered: make(chan struct{}), release: make(chan struct{})}
	child.services = wrapper
	in.Launch.Refresh = false
	in.ProtectSnapshot = true
	denied := make(chan *api.SnapshotDenial, 1)
	in.OnSnapshotDenied = func(f *api.SnapshotDenial) { denied <- f }
	answer := make(chan protocol.Response, 1)
	go func() { response, _ := child.Call(ctx, protocol.MethodRun, "", in); answer <- response }()
	select {
	case f := <-denied:
		if f.Failure == nil || f.Failure.Code != api.CodePermission {
			close(wrapper.release)
			t.Fatal("wrong denial")
		}
	case <-time.After(3 * time.Second):
		close(wrapper.release)
		t.Fatal("denial waited for later I/O")
	}
	select {
	case <-wrapper.entered:
	case <-time.After(3 * time.Second):
		close(wrapper.release)
		t.Fatal("later call not held")
	}
	select {
	case <-answer:
		close(wrapper.release)
		t.Fatal("full command result returned before held I/O")
	default:
	}
	close(wrapper.release)
	select {
	case response := <-answer:
		if response.PermissionFailure == nil {
			t.Fatal("denial not retained in final result")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("command did not finish")
	}

}

func TestSupervisedScreenshotsDenialEvictsAndReopenRemainsProtected(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ref := api.ActionRef{ID: "screenshots.grid"}
	first, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: ref, Launch: api.LaunchContext{Refresh: true}})
	if err != nil {
		t.Fatal(err)
	}
	var result api.Result
	if err = json.Unmarshal(first.Data, &result); err != nil || result.View == nil || len(result.View.Items) != 1 {
		t.Fatal("prime failed")
	}
	result.View.Refresh = &api.Refresh{Immediate: true, Ref: &ref}
	id := new(byte)
	cache := &extview.CommandSnapshots{}
	cache.PutVerified(id, ref, *result.View, result.View.Items, first.SnapshotEvidence)
	services := &revocableScreenshotsServices{ServiceHandler: child.services}
	services.denied.Store(true)
	child.services = services
	queue := make(chan func(), 2)
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
	deps := extview.Deps{OnUI: func(fn func()) { queue <- fn }, CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(ext string, in api.Invocation, done func(api.Result, *api.Failure)) { invoke(ctx, ext, in, done) }, InvokeCommandContext: invoke}
	h := &probePaletteHost{pushed: make(chan any, 2)}
	action := extview.CommandActionFor(deps, "local.screenshots", api.Run("screenshots.grid", "Screenshots", nil))
	action.Run(h)
	page := (<-h.pushed).(*palette.View)
	select {
	case fn := <-queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("denial not delivered")
	}
	if _, ok := cache.Get(id, ref); ok {
		t.Fatal("denied snapshot survived")
	}
	if page.Title != "Access unavailable" {
		t.Fatal("denial did not erase visible title")
	}
	select {
	case fn := <-queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("command did not finish")
	}
	returned := (<-h.pushed).(*palette.View)
	for _, row := range returned.Search("") {
		if row.Title == "Screen Shot.png" || len(row.Actions) != 0 {
			t.Fatal("denied file content returned")
		}
	}
	action.Run(h)
	select {
	case <-h.pushed:
		t.Fatal("reopen displayed denied cached pixels")
	default:
	}
	select {
	case fn := <-queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("reopen did not finish")
	}
	select {
	case v := <-h.pushed:
		for _, row := range v.(*palette.View).Search("") {
			if row.Title == "Screen Shot.png" || len(row.Actions) != 0 {
				t.Fatal("reopen displayed denied file content")
			}
		}
	default:
	}
}
