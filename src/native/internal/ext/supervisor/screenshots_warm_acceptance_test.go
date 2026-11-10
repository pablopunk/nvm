package supervisor

import (
	"context"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/platform"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
)

type heldScreenshotFS struct {
	platform.FS
	held             atomic.Bool
	heldStat         atomic.Bool
	denied           atomic.Bool
	target           string
	statTarget       string
	entered, release chan struct{}
	enterOnce        sync.Once
}

func (f *heldScreenshotFS) Stat(path string) (fs.FileInfo, error) {
	if path == f.statTarget && f.heldStat.Load() {
		f.enterOnce.Do(func() { close(f.entered) })
		<-f.release
	}
	return f.FS.Stat(path)
}

func (f *heldScreenshotFS) Lstat(path string) (fs.FileInfo, error) {
	if path == f.target && f.denied.Load() {
		return nil, fs.ErrPermission
	}
	if path == f.target && (f.held.Load() || f.heldStat.Load()) {
		f.enterOnce.Do(func() { close(f.entered) })
		<-f.release
	}
	return f.FS.Lstat(path)
}
func (f *heldScreenshotFS) ReadFileNoFollow(path string, max int64) ([]byte, error) {
	if path == f.target && f.denied.Load() {
		return nil, fs.ErrPermission
	}
	if path == f.target && f.held.Load() {
		f.enterOnce.Do(func() { close(f.entered) })
		<-f.release
	}
	return f.FS.(interface {
		ReadFileNoFollow(string, int64) ([]byte, error)
	}).ReadFileNoFollow(path, max)
}

// Reads remain independently controllable for consumers that actually request source bytes.
func (f *heldScreenshotFS) ReadFilePrefix(path string, max int64) ([]byte, int64, error) {
	if path == f.target && f.denied.Load() {
		return nil, 0, fs.ErrPermission
	}
	if path == f.target && f.held.Load() {
		f.enterOnce.Do(func() { close(f.entered) })
		<-f.release
	}
	return f.FS.ReadFilePrefix(path, max)
}

func TestScreenshotsWarmHostPaintsWhileIOHeld(t *testing.T) {
	for _, kind := range []string{"metadata", "stat"} {
		t.Run(kind, func(t *testing.T) { testScreenshotsWarmHostPaintsWhileIOHeld(t, kind) })
	}
}

func testScreenshotsWarmHostPaintsWhileIOHeld(t *testing.T, kind string) {
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
	if err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 100, 60))); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	fsys := &heldScreenshotFS{FS: rig.deps.Files, target: path, statTarget: root, entered: make(chan struct{}), release: make(chan struct{})}
	rig.deps.Files = fsys
	child := servicedRuntime(t, "local.screenshots", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Prime the extension's own persisted file list with the real child.
	resp, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}})
	if err != nil || resp.Failure != nil {
		t.Fatalf("prime: %v %+v", err, resp.Failure)
	}
	queue := make(chan func(), 8)
	invoke := func(callCtx context.Context, _ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		go func() {
			answer, callErr := child.Call(callCtx, protocol.MethodRun, "", &in)
			var result api.Result
			failure := answer.Failure
			if callErr != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: callErr.Error()}
			} else if err := json.Unmarshal(answer.Data, &result); err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			}
			result.SnapshotEvidence = answer.SnapshotEvidence
			queue <- func() { done(result, failure) }
		}()
	}
	id := new(byte)
	snapshots := &extview.CommandSnapshots{}
	deps := extview.Deps{CommandSnapshots: snapshots, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(ext string, in api.Invocation, done func(api.Result, *api.Failure)) { invoke(ctx, ext, in, done) }, InvokeCommandContext: invoke, InvokeRefresh: invoke}
	host := &probePaletteHost{pushed: make(chan any, 2)}
	action := extview.CommandActionFor(deps, "local.screenshots", api.Run("screenshots.grid", "Screenshots", nil))
	action.Run(host)
	waitUI := func() {
		t.Helper()
		select {
		case fn := <-queue:
			fn()
		case <-time.After(8 * time.Second):
			t.Fatal("timed out waiting for child")
		}
	}
	waitUI()
	first := (<-host.pushed).(*palette.View)
	waitUI()
	ref := api.ActionRef{ID: "screenshots.grid"}
	if view, ok := snapshots.Get(id, ref); !ok || len(view.Items) != 1 || view.Items[0].Image == nil {
		t.Fatalf("host did not retain validated view: %+v %v", view, ok)
	}
	first.OnLeave()
	{
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, info.Size()), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
	}
	if kind == "stat" {
		time.Sleep(6 * time.Second)
		fsys.heldStat.Store(true)
	} else {
		fsys.held.Store(true)
	}
	action.Run(host)
	var page *palette.View
	select {
	case v := <-host.pushed:
		page = v.(*palette.View)
	case <-time.After(time.Second):
		close(fsys.release)
		t.Fatal("warm view waited for metadata")
	}
	rows := page.Search("")
	if len(rows) != 1 || rows[0].Title == "" || len(rows[0].DragPaths) != 0 {
		close(fsys.release)
		t.Fatalf("warm row is missing or exposes a path: %+v", rows)
	}
	for _, action := range rows[0].Actions {
		if action.Title != "" {
			close(fsys.release)
			t.Fatalf("warm row exposes action %q", action.Title)
		}
	}
	if kind == "stat" {
		waitUI()
		<-host.pushed
	}
	select {
	case <-fsys.entered:
	case <-time.After(5 * time.Second):
		close(fsys.release)
		t.Fatal("fresh invocation did not reach held I/O")
	}
	if kind == "metadata" {
		page.OnLeave()
		waitUI()
	}
	close(fsys.release)
	if kind != "metadata" {
		waitUI()
	}
	got, ok := snapshots.Get(id, ref)
	if kind == "stat" && (!ok || len(got.Items) != 1 || got.Items[0].Image == nil) {
		t.Fatal("lazy enumeration discarded valid provenance because pixel decoding is deferred")
	}
	if kind == "metadata" && (!ok || len(got.Items) != 1 || got.Items[0].Image == nil) {
		t.Fatal("dismissal published obsolete result")
	}
}

type probePaletteHost struct{ pushed chan any }

func (*probePaletteHost) Hide()           {}
func (*probePaletteHost) Pop()            {}
func (*probePaletteHost) Root()           {}
func (h *probePaletteHost) Push(v any)    { h.pushed <- v }
func (*probePaletteHost) Toast(string)    {}
func (*probePaletteHost) Refresh()        {}
func (*probePaletteHost) Query() string   { return "" }
func (*probePaletteHost) SetQuery(string) {}
