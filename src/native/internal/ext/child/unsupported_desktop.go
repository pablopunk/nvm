package child

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The desktop sub-surfaces the host does not serve yet. Each answers with the unsupported error, or with the empty
// answer its signature allows, so an extension that calls one fails on that call and never panics.

func (remoteSystemClipboard) ReadHTML() (string, error) {
	return "", unsupported("ctx.desktop.clipboard.readHtml")
}

func (remoteSystemClipboard) WriteHTML(string, string, bool) error {
	return unsupported("ctx.desktop.clipboard.writeHtml")
}

func (remoteSystemClipboard) ReadFiles() ([]api.FileRef, error) {
	return nil, unsupported("ctx.desktop.clipboard.readFiles")
}

func (remoteSystemClipboard) WriteFiles([]string, bool) error {
	return unsupported("ctx.desktop.clipboard.writeFiles")
}

func (remoteSystemClipboard) Read([]string) (api.ClipboardContent, error) {
	return api.ClipboardContent{}, unsupported("ctx.desktop.clipboard.read")
}

func (remoteSystemClipboard) Write(api.ClipboardContent) error {
	return unsupported("ctx.desktop.clipboard.write")
}

func (remoteWindowManagement) Apply(api.WindowLayout, api.WindowManagementOptions) (api.ApplyResult, error) {
	return api.ApplyResult{}, unsupported("ctx.desktop.windowManagement.apply")
}

func (remoteWindowManagement) WaitForWindow(api.WindowQuery, time.Duration) (api.WindowInfo, bool, error) {
	return api.WindowInfo{}, false, unsupported("ctx.desktop.windowManagement.wait")
}

func (remoteWindowManagement) FocusNeighbor(string, bool) (api.FocusResult, error) {
	return api.FocusResult{}, unsupported("ctx.desktop.windowManagement.focusNeighbor")
}
