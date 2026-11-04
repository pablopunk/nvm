package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// openWebview opens the page of a webview view and loads its document, themed for the palette. The page is untrusted:
// it has no Go bridge, and it gets only the permissions the view declares. Declared camera or microphone access is
// asked of the system first, as Electron does, and the page loads whatever the answer.
func (s *Session) openWebview() {
	spec := s.view.Webview
	page, err := s.openPage(platform.WebPageSpec{
		Grant:        webviewGrant(spec.Permissions),
		Navigate:     webviewNavigationAllowed,
		OpenExternal: externalOpener(s.deps.OpenURL),
	})
	if err != nil {
		return
	}
	s.page = page
	s.requestDeclaredAccess(spec.Permissions)
	page.Load(withWebviewTheme(spec.HTML, webviewThemeDeclarations(s.isDark())))
}

// requestDeclaredAccess asks the system for each access the view declares that the system has a prompt for.
func (s *Session) requestDeclaredAccess(declared []api.WebviewPermission) {
	if s.deps.Permissions == nil {
		return
	}
	for _, permission := range declared {
		switch permission {
		case api.WebviewCamera:
			_ = s.deps.Permissions.Request(platform.PermissionCamera)
		case api.WebviewMicrophone:
			_ = s.deps.Permissions.Request(platform.PermissionMicrophone)
		}
	}
}

// drawWebview places the webview's page, or the notice of a system that has no web view.
func (s *Session) drawWebview(c *ui.Context) {
	if s.page == nil {
		drawNotice(c, webViewUnavailableMessage)
		return
	}
	drawPage(c, s.page, nil)
}
