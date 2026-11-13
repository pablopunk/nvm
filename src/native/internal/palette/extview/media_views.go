package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// webViewUnavailableMessage is shown where a web page would be, on a system without a web view.
const webViewUnavailableMessage = "This view needs the system web view, which this system does not have."

// isMediaView reports whether the view is a page of the system web view: a webview, or a camera that runs on one.
func (s *Session) isMediaView() bool {
	return s.view.Kind == api.ViewWebview || s.view.Kind == api.ViewCamera
}

// openMedia makes the view's page once, when the view opens. The palette then only places the page, and leaving the
// view destroys it and stops whatever it captures.
func (s *Session) openMedia() {
	if s.view.Kind == api.ViewCamera {
		s.openCamera()
		s.media = &palette.View{
			Title: s.view.Title, HideInput: true, OnLeave: s.closeMedia, OnActive: s.camera.setActive,
			Body: func(c *ui.Context, _ search.Host) { drawCamera(c, s.camera) },
		}
		return
	}
	s.openWebview()
	s.media = &palette.View{
		Title: s.view.Title, HideInput: true, OnLeave: s.closeMedia,
		Body: func(c *ui.Context, _ search.Host) { s.drawWebview(c) },
	}
}

// closeMedia leaves the view: the camera stops, and the page is destroyed. It runs once.
func (s *Session) closeMedia() {
	if s.dismissed {
		return
	}
	s.dismissed = true
	if s.camera != nil {
		s.camera.close()
	}
	if s.page != nil {
		s.page.Destroy()
	}
}

// openPage opens a page through the host, or reports that the system has none.
func (s *Session) openPage(spec platform.WebPageSpec) (platform.WebPage, error) {
	if s.deps.Pages == nil {
		return nil, platform.ErrUnsupported
	}
	return s.deps.Pages.Open(spec)
}

// isDark reports the palette's appearance for the page's theme.
func (s *Session) isDark() bool {
	return s.deps.IsDark != nil && s.deps.IsDark()
}

// drawPage places a page's system view where the body draws it, with overlay painted over it. While the system has
// no view for the page, the body shows a plate of the sunken surface in its place, so the layout does not move.
func drawPage(c *ui.Context, page platform.WebPage, overlay func()) {
	if overlay == nil {
		overlay = func() {}
	}
	native, _ := page.Native().(ui.NativeWebView)
	if native == nil {
		ui.Column(c).Grow(1).Background(design.Tokens(c).Sunken).Children(overlay)
		return
	}
	ui.WebView(c, native).Grow(1).Children(overlay)
}
