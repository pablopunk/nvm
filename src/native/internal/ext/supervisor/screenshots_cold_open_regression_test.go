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
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
)

func TestScreenshotsColdHostOpensGridWhileThumbnailHeld(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "cancel"}[cancel], func(t *testing.T) { screenshotsColdHost(t, cancel) })
	}
}

func screenshotsColdHost(t *testing.T, cancelNavigation bool) {
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
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 100, 60))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	fsys := &heldScreenshotFS{FS: rig.deps.Files, target: path, entered: make(chan struct{}), release: make(chan struct{})}
	rig.deps.Files = fsys
	child := servicedRuntime(t, "local.screenshots-migrated", rig, fixtureManifest("local.screenshots", api.CapabilityDesktopFiles))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	resp, err := child.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "screenshots.grid"}, Launch: api.LaunchContext{Refresh: true}})
	if err != nil || resp.Failure != nil {
		t.Fatalf("prime persisted list: %v %+v", err, resp.Failure)
	}
	commandsResponse, err := child.Call(ctx, protocol.MethodCommands, "", nil)
	if err != nil || commandsResponse.Failure != nil {
		t.Fatalf("commands: %v %+v", err, commandsResponse.Failure)
	}
	var commands []api.Command
	if err := json.Unmarshal(commandsResponse.Data, &commands); err != nil || len(commands) != 1 || commands[0].InitialView == nil {
		t.Fatalf("missing declared initial view: %s %v", commandsResponse.Data, err)
	}
	fsys.held.Store(true)
	released := false
	defer func() {
		if !released {
			close(fsys.release)
		}
	}()
	queue := make(chan func(), 2)
	invoke := func(callCtx context.Context, _ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		go func() {
			answer, err := child.Call(callCtx, protocol.MethodRun, "", &in)
			var result api.Result
			failure := answer.Failure
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			} else {
				_ = json.Unmarshal(answer.Data, &result)
			}
			queue <- func() { done(result, failure) }
		}()
	}
	host := &probePaletteHost{pushed: make(chan any, 2)}
	deps := extview.Deps{CommandSnapshots: &extview.CommandSnapshots{}, CommandSnapshotIdentity: func(string) any { return new(byte) }, InvokeCommand: func(id string, in api.Invocation, done func(api.Result, *api.Failure)) { invoke(ctx, id, in, done) }, InvokeCommandContext: invoke}
	extview.CommandActionFor(deps, "local.screenshots", api.Run("screenshots.grid", "Screenshots", nil), commands[0].InitialView).Run(host)
	select {
	case <-fsys.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("thumbnail read was not reached")
	}
	var initialPage *palette.View
	select {
	case value := <-host.pushed:
		initialPage = value.(*palette.View)
		if initialPage.Grid == nil || !initialPage.Loading() {
			t.Fatal("initial grid is not loading")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cold command did not open a view while thumbnail read was blocked")
	}
	if cancelNavigation {
		initialPage.OnLeave()
		select {
		case done := <-queue:
			done()
		case <-time.After(time.Second):
			t.Fatal("navigation did not cancel command")
		}
		select {
		case <-host.pushed:
			t.Fatal("canceled command navigated")
		default:
		}
		select {
		case child.slots <- struct{}{}:
			<-child.slots
		case <-time.After(2 * time.Second):
			t.Fatal("canceled command retained child slot while IO was held")
		}
		startAndWait(t, child)
		if _, err := child.Call(ctx, protocol.MethodCommands, "", nil); err != nil {
			t.Fatalf("restarted child: %v", err)
		}
		return
	}
	if os.Getenv("NVM_SLOW_IO_TEST") == "1" {
		time.Sleep(11 * time.Second)
	}
	close(fsys.release)
	released = true
	select {
	case done := <-queue:
		done()
	case <-time.After(5 * time.Second):
		t.Fatal("command did not complete")
	}
	select {
	case value := <-host.pushed:
		page := value.(*palette.View)
		rows := page.Search("")
		if page.Grid == nil || len(rows) != 1 {
			t.Fatalf("completed grid rows: %+v", rows)
		}
	case <-time.After(time.Second):
		t.Fatal("completed grid was not shown")
	}
}
