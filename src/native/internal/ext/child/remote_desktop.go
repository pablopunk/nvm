package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// remoteDesktop returns typed service proxies. Unsupported operations return structured host failures.
type remoteDesktop struct {
	link  *hostLink
	clock platform.Clock
}

func (d remoteDesktop) Clipboard() api.SystemClipboard {
	return remoteSystemClipboard{link: d.link}
}

func (d remoteDesktop) Keyboard() api.Keyboard {
	return remoteKeyboard{link: d.link}
}

func (d remoteDesktop) Selection() api.Selection {
	return remoteSelection{link: d.link}
}

func (d remoteDesktop) Apps() api.Apps {
	return remoteApps{link: d.link}
}

func (d remoteDesktop) Menus() api.Menus {
	return remoteMenus{link: d.link}
}

func (d remoteDesktop) Files() api.Files {
	return remoteFiles{link: d.link}
}

func (d remoteDesktop) Shell() api.Shell {
	return remoteShell{link: d.link}
}

func (d remoteDesktop) WindowManagement() api.WindowManagement {
	return remoteWindowManagement{link: d.link, clock: d.clock}
}

// remoteSystemClipboard reads and writes the system clipboard. Each method is one host call; see remote_clipboard.go for the formats.
type remoteSystemClipboard struct {
	link *hostLink
}

func (c remoteSystemClipboard) ReadText() (string, error) {
	var text string
	err := c.link.Call(protocol.ServiceDesktopClipboard, protocol.OpReadText, nil, &text)
	return text, err
}

func (c remoteSystemClipboard) WriteText(text string, concealed bool) error {
	return c.link.Call(protocol.ServiceDesktopClipboard, protocol.OpWriteText, protocol.WriteTextArgs{Text: text, Concealed: concealed}, nil)
}

func (c remoteSystemClipboard) ReadImage() (string, bool, error) {
	var image protocol.ImageResult
	err := c.link.Call(protocol.ServiceDesktopClipboard, protocol.OpReadImage, nil, &image)
	return image.DataURL, image.Found, err
}

func (c remoteSystemClipboard) WriteImage(dataURL string, concealed bool) error {
	return c.link.Call(protocol.ServiceDesktopClipboard, protocol.OpWriteImage, protocol.WriteImageArgs{DataURL: dataURL, Concealed: concealed}, nil)
}

// remoteApps lists, searches, launches and reads icons of the apps the host indexed.
type remoteApps struct {
	link *hostLink
}

func (a remoteApps) List() ([]api.App, error) {
	var apps []api.App
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpList, nil, &apps)
	return apps, err
}

func (a remoteApps) Search(query string) ([]api.App, error) {
	var apps []api.App
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpSearch, protocol.QueryArgs{Query: query}, &apps)
	return apps, err
}

func (a remoteApps) Launch(path string) error {
	return a.link.Call(protocol.ServiceDesktopApps, protocol.OpLaunch, protocol.PathArgs{Path: path}, nil)
}

// Frontmost names the app in front. The host answers with the bundle id or class, and the indexed path when it has one.
func (a remoteApps) Frontmost() (api.App, error) {
	var app api.App
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpFrontmost, nil, &app)
	return app, err
}

func (a remoteApps) Icon(path string) (string, bool, error) {
	var icon protocol.IconResult
	err := a.link.Call(protocol.ServiceDesktopApps, protocol.OpIcon, protocol.PathArgs{Path: path}, &icon)
	return icon.DataURL, icon.Found, err
}
