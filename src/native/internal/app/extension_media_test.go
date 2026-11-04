package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// mediaFixture is the dev fixture of the camera and webview views, each a command of its own.
const mediaFixture = "acme.media"

var mediaGoldens = golden.Options{Group: "media", Width: width, Height: height}

// mediaManager runs the media fixture in the child process and waits until it answers.
func mediaManager(t *testing.T) *manager.Manager {
	t.Helper()
	m := newFixtureManager(t, mediaFixture)
	waitForState(t, m, mediaFixture, supervisor.StateRunning)
	return m
}

// mediaSession opens a media fixture view in the golden palette with the system's web pages and permissions faked.
// The page is the palette's page above the root, and the fake page is returned for the test to play.
func mediaSession(t *testing.T, p *App, m *manager.Manager, ref string, pages platform.WebPages, perms platform.Permissions) (*palette.View, *fake.WebPage) {
	t.Helper()
	deps := syncViewDeps(m)
	deps.Pages = pages
	deps.Permissions = perms
	page := fixtureSession(t, m, mediaFixture, ref, deps).Open(&viewHost{})
	goldenOpen(p, page)
	return page, fakePageOf(pages)
}

// fakePageOf returns the most recent page of a fake web page system, or nil when none was opened.
func fakePageOf(pages platform.WebPages) *fake.WebPage {
	fakes, ok := pages.(*fake.WebPages)
	if !ok || len(fakes.Pages()) == 0 {
		return nil
	}
	list := fakes.Pages()
	return list[len(list)-1]
}

// cameraReport is the title the camera page sets for a report.
func cameraReport(json string) string { return "nvm-camera:" + json }

// TestFixtureCameraViewOpensThroughTheChild proves the camera fixture's view reaches the page through the child
// process, configured by the view, and that its contain form carries its framing.
func TestFixtureCameraViewOpensThroughTheChild(t *testing.T) {
	m := mediaManager(t)
	p := goldenPalette(t)
	pages := fake.NewWebPages()
	_, page := mediaSession(t, p, m, "camera-contain", pages, fake.NewPermissions())
	if page == nil {
		t.Fatal("the camera view opened no page")
	}
	if loaded := page.Loaded()[0]; !strings.Contains(loaded, `"framing":"contain"`) || !strings.Contains(loaded, `"mirrored":true`) {
		t.Fatalf("camera page config = %s", loaded[strings.Index(loaded, "configure(")-5:strings.Index(loaded, "configure(")+80])
	}
}

// TestFixtureWebviewViewLoadsThroughTheChild proves the hostile fixture's page loads untrusted and themed, with
// the permissions it declares and none more.
func TestFixtureWebviewViewLoadsThroughTheChild(t *testing.T) {
	m := mediaManager(t)
	p := goldenPalette(t)
	pages := fake.NewWebPages()
	_, page := mediaSession(t, p, m, "webview-hostile", pages, fake.NewPermissions())
	if page == nil || !strings.Contains(page.Loaded()[0], "Probe the host") {
		t.Fatalf("the hostile page did not load through the child")
	}
	if !strings.Contains(page.Loaded()[0], "data-nevermind-theme") {
		t.Fatal("the page was not themed")
	}
	if page.Requests("camera") {
		t.Fatal("a page that declared no permission was granted the camera")
	}
}

// TestGoldenCameraStatesAndUnavailableViews draws the camera view in each state the palette shows, and the two
// views of a system without a web view.
func TestGoldenCameraStatesAndUnavailableViews(t *testing.T) {
	m := mediaManager(t)
	p := goldenPalette(t)
	pages := fake.NewWebPages()
	page, fakePage := mediaSession(t, p, m, "camera", pages, fake.NewPermissions())
	golden.AssertMatrix(t, "camera-paused", p.View, mediaGoldens)

	page.OnActive(true)
	goldenQuery(p, "")
	golden.AssertMatrix(t, "camera-requesting", p.View, mediaGoldens)

	fakePage.SetTitle(cameraReport(`{"seq":1,"event":"live","id":1,"deviceId":"front","label":"FaceTime HD Camera"}`))
	fakePage.SetTitle(cameraReport(`{"seq":2,"event":"playing","id":1}`))
	goldenQuery(p, "")
	golden.AssertMatrix(t, "camera-live", p.View, mediaGoldens)

	fakePage.SetTitle(cameraReport(`{"seq":3,"event":"error","id":1,"name":"NotAllowedError"}`))
	goldenQuery(p, "")
	golden.AssertMatrix(t, "camera-denied", p.View, mediaGoldens)
}

// TestGoldenCameraSwitchButtonWithTwoCameras draws the live camera with the switch control that two cameras show.
func TestGoldenCameraSwitchButtonWithTwoCameras(t *testing.T) {
	m := mediaManager(t)
	p := goldenPalette(t)
	page, fakePage := mediaSession(t, p, m, "camera", fake.NewWebPages(), fake.NewPermissions())
	page.OnActive(true)
	fakePage.SetTitle(cameraReport(`{"seq":1,"event":"devices","devices":[{"deviceId":"a","label":"FaceTime HD Camera"},{"deviceId":"b","label":"USB Camera"}]}`))
	fakePage.SetTitle(cameraReport(`{"seq":2,"event":"live","id":1,"deviceId":"a","label":"FaceTime HD Camera"}`))
	fakePage.SetTitle(cameraReport(`{"seq":3,"event":"playing","id":1}`))
	goldenQuery(p, "")
	golden.AssertMatrix(t, "camera-switch", p.View, mediaGoldens)
}

// TestGoldenWebviewPlateAndNotice draws a webview whose page the system has not placed yet, and a webview on a
// system without a web view.
func TestGoldenWebviewPlateAndNotice(t *testing.T) {
	m := mediaManager(t)
	p := goldenPalette(t)
	mediaSession(t, p, m, "webview", fake.NewWebPages(), fake.NewPermissions())
	golden.AssertMatrix(t, "webview-plate", p.View, mediaGoldens)

	unavailable := fake.NewWebPages()
	unavailable.Available = false
	mediaSession(t, p, m, "webview", unavailable, fake.NewPermissions())
	golden.AssertMatrix(t, "webview-unavailable", p.View, mediaGoldens)
}
