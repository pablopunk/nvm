package extview

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestCameraWaitsForDriverAndAppliesLatestConfiguration(t *testing.T) {
	rig := newMediaRigBeforePageReady(t, cameraFixtureView, nil)
	page := rig.fakePage()
	rig.activate()
	for _, op := range []string{"toggle-muted", "toggle-controls"} {
		rig.session.action(api.Action{Kind: api.ActionCamera, Op: op}, "").Run(rig.host)
	}
	if len(page.Scripts()) != 0 || !rig.draw().HasText("Requesting camera…") {
		t.Fatalf("camera must wait for its driver without losing loading feedback: scripts=%v", page.Scripts())
	}
	page.SetTitle(report(`{"event":"ready"}`))
	scripts := page.Scripts()
	if len(scripts) != 2 || !strings.Contains(scripts[0], `"op":"configure"`) || !strings.Contains(scripts[0], `"muted":false,"controls":true`) || scripts[1] != `window.nvmCamera.command({"op":"start","id":1})` {
		t.Fatalf("ready page must receive the latest configuration before one start: %v", scripts)
	}
	page.SetTitle(report(`{"event":"ready"}`))
	if len(page.Scripts()) != 2 {
		t.Fatal("duplicate readiness restarted the camera")
	}
	page.SetTitle(report(`{"event":"error","id":1,"name":"NotFoundError"}`))
	if !rig.draw().HasText("Camera Issue") || !rig.draw().HasText("Camera unavailable") {
		t.Fatal("the initialized page's camera failure was not shown")
	}
	rig.page.OnLeave()
}

func TestCameraReadyAfterCoverDoesNotStartAndClosedPageReportsAreIgnored(t *testing.T) {
	rig := newMediaRigBeforePageReady(t, cameraFixtureView, nil)
	page := rig.fakePage()
	rig.activate()
	rig.page.OnActive(false)
	if len(page.Scripts()) != 0 {
		t.Fatalf("uninitialized camera received commands: %v", page.Scripts())
	}
	page.SetTitle(report(`{"event":"ready"}`))
	if scripts := page.Scripts(); len(scripts) != 1 || scripts[0] != `window.nvmCamera.command({"op":"stop"})` {
		t.Fatalf("a covered page started while becoming ready: %v", scripts)
	}
	rig.activate()
	if lastScript(page) != `window.nvmCamera.command({"op":"start","id":3})` {
		t.Fatalf("reshown camera did not start its current generation: %v", page.Scripts())
	}
	rig.page.OnLeave()
	count := len(page.Scripts())
	page.SetTitle(report(`{"event":"ready"}`))
	page.SetTitle(report(`{"event":"live","id":4}`))
	if !page.Destroyed() || len(page.Scripts()) != count || rig.session.camera.engine.state.Status != cameraIdle {
		t.Fatal("a destroyed page changed camera state or ran another command")
	}
}
