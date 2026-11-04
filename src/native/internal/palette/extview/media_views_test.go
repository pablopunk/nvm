package extview

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const mediaExtension = "acme.media"

var cameraFixtureView = api.View{ID: "camera-fixture", Kind: api.ViewCamera, Title: "Camera fixture", Camera: &api.CameraSpec{}}

// mediaRig is a session of a media view with the system's web pages, permissions and browser replaced by fakes.
type mediaRig struct {
	t       *testing.T
	pages   *fake.WebPages
	perms   *fake.Permissions
	opened  []string
	host    *fakeHost
	session *Session
	page    *palette.View
}

func newMediaRig(t *testing.T, view api.View) *mediaRig {
	return newMediaRigWithCameras(t, view, nil)
}

// newMediaRigWithCameras is newMediaRig with the camera memory the view reads when it opens.
func newMediaRigWithCameras(t *testing.T, view api.View, cameras CameraMemory) *mediaRig {
	t.Helper()
	rig := &mediaRig{t: t, pages: fake.NewWebPages(), perms: fake.NewPermissions(), host: &fakeHost{}}
	rig.session = NewSession(Deps{
		Pages:       rig.pages,
		Permissions: rig.perms,
		Cameras:     cameras,
		OpenURL:     func(url string) { rig.opened = append(rig.opened, url) },
		IsDark:      func() bool { return false },
	}, mediaExtension, view)
	rig.page = rig.session.Open(rig.host)
	return rig
}

// fakePage returns the one page the view opened.
func (r *mediaRig) fakePage() *fake.WebPage {
	r.t.Helper()
	pages := r.pages.Pages()
	if len(pages) != 1 {
		r.t.Fatalf("the view opened %d pages, want 1", len(pages))
	}
	return pages[0]
}

// draw renders the view's body and returns the tester that shows what it drew.
func (r *mediaRig) draw() *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) { r.page.Body(c, r.host) }, 720, 470)
	tt.Frame()
	return tt
}

func (r *mediaRig) activate() { r.page.OnActive(true) }

func lastScript(page *fake.WebPage) string {
	scripts := page.Scripts()
	if len(scripts) == 0 {
		return ""
	}
	return scripts[len(scripts)-1]
}

func report(json string) string { return "nvm-camera:" + json }

// TestCameraStartsWhileActiveAndStopsWhenLeft proves the capture runs only while the view is the active page, and
// that leaving the view stops the stream and destroys the page.
func TestCameraStartsWhileActiveAndStopsWhenLeft(t *testing.T) {
	rig := newMediaRig(t, cameraFixtureView)
	page := rig.fakePage()
	if !strings.Contains(page.Loaded()[0], "window.nvmCamera") {
		t.Fatal("the camera page was not loaded")
	}
	if len(page.Scripts()) != 0 {
		t.Fatalf("the camera ran before the view was active: %v", page.Scripts())
	}
	rig.activate()
	if want := `window.nvmCamera.command({"op":"start","id":1})`; lastScript(page) != want {
		t.Fatalf("last script = %s, want %s", lastScript(page), want)
	}
	rig.page.OnLeave()
	if want := `window.nvmCamera.command({"op":"stop"})`; lastScript(page) != want || !page.Destroyed() {
		t.Fatalf("leaving: last script %s, destroyed %v", lastScript(page), page.Destroyed())
	}
}

// TestCameraStopsWhenCoveredAndStartsAgainWhenShown proves a covered or hidden camera releases the device.
func TestCameraStopsWhenCoveredAndStartsAgainWhenShown(t *testing.T) {
	rig := newMediaRig(t, cameraFixtureView)
	page := rig.fakePage()
	rig.activate()
	rig.page.OnActive(false)
	if want := `window.nvmCamera.command({"op":"stop"})`; lastScript(page) != want {
		t.Fatalf("covered: last script %s", lastScript(page))
	}
	rig.activate()
	if want := `window.nvmCamera.command({"op":"start","id":3})`; lastScript(page) != want {
		t.Fatalf("shown again: last script %s", lastScript(page))
	}
}

// TestCameraShowsItsStatusAndTheReasonWhenItFails proves the palette shows the status the page reports and the
// Electron copy for a failed start.
func TestCameraShowsItsStatusAndTheReasonWhenItFails(t *testing.T) {
	rig := newMediaRig(t, cameraFixtureView)
	page := rig.fakePage()
	rig.activate()
	page.SetTitle(report(`{"seq":1,"event":"live","id":1,"deviceId":"front","label":"FaceTime"}`))
	if !rig.draw().HasText("Starting…") {
		t.Fatal("a live stream before its first frame must read Starting…")
	}
	page.SetTitle(report(`{"seq":2,"event":"playing","id":1}`))
	if !rig.draw().HasText("Live") {
		t.Fatal("a playing stream must read Live")
	}
	page.SetTitle(report(`{"seq":3,"event":"error","id":1,"name":"NotAllowedError"}`))
	tt := rig.draw()
	for _, want := range []string{"Camera Issue", "Camera access is denied. Allow Nevermind in system privacy settings.", "Camera unavailable"} {
		if !tt.HasText(want) {
			t.Errorf("failed camera misses %q; texts %v", want, tt.Texts())
		}
	}
}

// TestCameraReportsFromAReplacedStreamAreIgnored proves a page report that belongs to an old start changes nothing.
func TestCameraReportsFromAReplacedStreamAreIgnored(t *testing.T) {
	rig := newMediaRig(t, cameraFixtureView)
	page := rig.fakePage()
	rig.activate()
	rig.page.OnActive(false)
	rig.activate()
	page.SetTitle(report(`{"seq":1,"event":"error","id":1,"name":"NotAllowedError"}`))
	if rig.draw().HasText("Camera Issue") {
		t.Fatal("a report of the replaced start showed an error")
	}
}

// TestCameraRefusedBySystemNeverReachesThePage proves a denied camera stops before the page asks for a device.
func TestCameraRefusedBySystemNeverReachesThePage(t *testing.T) {
	rig := newMediaRig(t, cameraFixtureView)
	page := rig.fakePage()
	rig.perms.SetState(platform.PermissionCamera, platform.PermissionDenied)
	rig.activate()
	for _, script := range page.Scripts() {
		if strings.Contains(script, `"op":"start"`) {
			t.Fatalf("a denied camera started: %s", script)
		}
	}
	if !rig.draw().HasText("Camera access is denied in system privacy settings.") {
		t.Fatal("the denial was not shown")
	}
}

// TestCameraWithoutWebViewSaysSo proves a system with no web view shows the notice and starts nothing.
func TestCameraWithoutWebViewSaysSo(t *testing.T) {
	rig := &mediaRig{t: t, pages: fake.NewWebPages(), perms: fake.NewPermissions(), host: &fakeHost{}}
	rig.pages.Available = false
	rig.session = NewSession(Deps{Pages: rig.pages}, mediaExtension, cameraFixtureView)
	rig.page = rig.session.Open(rig.host)
	rig.activate()
	if !rig.draw().HasText(webViewUnavailableMessage) {
		t.Fatal("the unavailable notice was not shown")
	}
	if len(rig.pages.Pages()) != 0 {
		t.Fatal("a page was recorded without a web view")
	}
}

// TestCameraSwitchesDevicesAndRemembersTheChoice proves switching restarts the capture on the next camera, the
// choice is kept for the next open of the view, and the switch button appears only with two cameras.
func TestCameraSwitchesDevicesAndRemembersTheChoice(t *testing.T) {
	memory := NewMemoryCameras()
	rig := newMediaRigWithCameras(t, cameraFixtureView, memory)
	page := rig.fakePage()
	rig.activate()
	page.SetTitle(report(`{"seq":1,"event":"devices","devices":[{"deviceId":"a","label":"A"},{"deviceId":"b","label":"B"}]}`))
	page.SetTitle(report(`{"seq":2,"event":"live","id":1,"deviceId":"a","label":"A"}`))
	if !rig.draw().HasText("Switch Camera · A") {
		t.Fatal("the switch button did not show with two cameras")
	}
	rig.session.camera.operate("next-device")
	if want := `window.nvmCamera.command({"op":"start","id":2,"deviceId":"b"})`; lastScript(page) != want {
		t.Fatalf("switched: last script %s", lastScript(page))
	}
	if saved, ok := memory.Saved("camera-device:camera-fixture"); !ok || saved != "b" {
		t.Fatalf("remembered = %q, %v; want b", saved, ok)
	}
	reopened := newMediaRigWithCameras(t, cameraFixtureView, memory)
	reopened.activate()
	if want := `window.nvmCamera.command({"op":"start","id":1,"deviceId":"b"})`; lastScript(reopened.fakePage()) != want {
		t.Fatalf("reopened: last script %s", lastScript(reopened.fakePage()))
	}
}

// TestSavedCameraThatIsGoneFallsBackAndIsForgotten proves a remembered camera the system lost is forgotten, and the
// default camera is started in its place.
func TestSavedCameraThatIsGoneFallsBackAndIsForgotten(t *testing.T) {
	memory := NewMemoryCameras()
	memory.Remember("camera-device:camera-fixture", "gone")
	rig := &mediaRig{t: t, pages: fake.NewWebPages(), perms: fake.NewPermissions(), host: &fakeHost{}}
	rig.session = NewSession(Deps{Pages: rig.pages, Cameras: memory}, mediaExtension, cameraFixtureView)
	rig.page = rig.session.Open(rig.host)
	page := rig.fakePage()
	rig.activate()
	if want := `window.nvmCamera.command({"op":"start","id":1,"deviceId":"gone"})`; lastScript(page) != want {
		t.Fatalf("started: last script %s", lastScript(page))
	}
	page.SetTitle(report(`{"seq":1,"event":"error","id":1,"name":"NotFoundError"}`))
	if want := `window.nvmCamera.command({"op":"start","id":2})`; lastScript(page) != want {
		t.Fatalf("fallback: last script %s", lastScript(page))
	}
	if _, ok := memory.Saved("camera-device:camera-fixture"); ok {
		t.Fatal("the missing camera was still remembered")
	}
}

// TestCameraMuteAndControlsReachThePageWithoutRestart proves the toggles update the presentation only.
func TestCameraMuteAndControlsReachThePageWithoutRestart(t *testing.T) {
	rig := newMediaRig(t, cameraFixtureView)
	page := rig.fakePage()
	rig.activate()
	before := len(page.Scripts())
	rig.session.action(api.Action{Kind: api.ActionCamera, Op: "toggle-muted"}, "").Run(rig.host)
	rig.session.action(api.Action{Kind: api.ActionCamera, Op: "toggle-controls"}, "").Run(rig.host)
	if len(page.Scripts()) != before+2 {
		t.Fatalf("toggles sent %d scripts", len(page.Scripts())-before)
	}
	if want := `"muted":false`; !strings.Contains(lastScript(page), want) {
		t.Fatalf("last toggle = %s, want muted false", lastScript(page))
	}
	if !slices.ContainsFunc(page.Scripts()[before:], func(s string) bool { return strings.Contains(s, `"controls":true`) }) {
		t.Fatalf("controls toggle missing: %v", page.Scripts()[before:])
	}
}

// TestWebviewLoadsAThemedUntrustedPage proves the page starts with the palette's theme, asks for what the view
// declares only, and leaves the app for every URL it may not load.
func TestWebviewLoadsAThemedUntrustedPage(t *testing.T) {
	view := api.View{
		ID: "webview-fixture", Kind: api.ViewWebview, Title: "Webview fixture",
		Webview: &api.WebviewSpec{HTML: "<!doctype html><head></head><body>x</body>", Permissions: []api.WebviewPermission{api.WebviewCamera}},
	}
	rig := newMediaRig(t, view)
	page := rig.fakePage()
	if !strings.HasPrefix(page.Loaded()[0], "<!doctype html><head><style data-nevermind-theme>") {
		t.Fatalf("page not themed in its head: %s", page.Loaded()[0])
	}
	if !slices.Contains(rig.perms.Requests(), platform.PermissionCamera) {
		t.Fatal("the declared camera was not asked of the system")
	}
	if !page.Requests("camera") || page.Requests("microphone") || page.Requests("camera", "microphone") {
		t.Fatal("the page's permissions are not the declared ones")
	}
	page.Navigate("about:blank#top")
	if page.Navigate("https://example.test/") || page.Navigate("file:///etc/hosts") {
		t.Fatal("a page left its document")
	}
	if want := []string{"https://example.test/"}; !slices.Equal(rig.opened, want) {
		t.Fatalf("opened outside = %v, want %v", rig.opened, want)
	}
	rig.page.OnLeave()
	if !page.Destroyed() {
		t.Fatal("leaving the webview did not destroy its page")
	}
}

// TestWebviewWithoutWebViewSaysSo proves a system with no web view shows the notice where the page would be.
func TestWebviewWithoutWebViewSaysSo(t *testing.T) {
	pages := fake.NewWebPages()
	pages.Available = false
	view := api.View{ID: "webview-fixture", Kind: api.ViewWebview, Title: "Webview fixture", Webview: &api.WebviewSpec{HTML: "<p>x</p>"}}
	session := NewSession(Deps{Pages: pages}, mediaExtension, view)
	host := &fakeHost{}
	page := session.Open(host)
	tt := ui.NewTester(func(c *ui.Context) { page.Body(c, host) }, 720, 470)
	tt.Frame()
	if !tt.HasText(webViewUnavailableMessage) {
		t.Fatal("the unavailable notice was not shown")
	}
}

// TestWebviewWithoutPagesFactoryRefusesWithoutPanicking proves a nil page factory is the same as no web view.
func TestWebviewWithoutPagesFactoryRefusesWithoutPanicking(t *testing.T) {
	view := api.View{ID: "webview-fixture", Kind: api.ViewWebview, Title: "Webview fixture", Webview: &api.WebviewSpec{HTML: "<p>x</p>"}}
	session := NewSession(Deps{}, mediaExtension, view)
	host := &fakeHost{}
	page := session.Open(host)
	page.OnLeave()
	if session.page != nil {
		t.Fatal("a page appeared without a factory")
	}
}
