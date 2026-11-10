package supervisor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func TestDeclarativeNavigationFixtureThroughChild(t *testing.T) {
	requireUnixProcesses(t)
	rig := newServicesRig(t)
	r := servicedRuntime(t, "acme.navigation", rig, fixtureManifest("acme.navigation"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	commands, err := r.Call(ctx, protocol.MethodCommands, "", nil)
	if err != nil || commands.Failure != nil {
		t.Fatalf("commands: %v %+v", err, commands.Failure)
	}
	t.Logf("commands: %s", commands.Data)
	response, err := r.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: "navigation"}})
	if err != nil || response.Failure != nil {
		t.Fatalf("run: %v %+v", err, response.Failure)
	}
	var result api.Result
	if err := json.Unmarshal(response.Data, &result); err != nil || result.View == nil || len(result.View.Items) != 1 {
		t.Fatalf("view: %s %v", response.Data, err)
	}
	branch := result.View.Items[0].Primary
	if branch == nil || branch.Kind != api.ActionPushView || branch.View == nil || len(branch.View.Items) != 2 {
		t.Fatalf("branch: %+v", branch)
	}
	if branch.View.Items[0].Primary == nil || branch.View.Items[0].Primary.Kind != api.ActionReplaceView || branch.View.Items[1].Primary == nil || branch.View.Items[1].Primary.Kind != api.ActionRootView {
		t.Fatalf("declarative actions: %+v", branch.View.Items)
	}
}
