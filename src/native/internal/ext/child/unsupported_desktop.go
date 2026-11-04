package child

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The desktop sub-surfaces the host does not serve yet. Each answers with the unsupported error, or with the empty
// answer its signature allows, so an extension that calls one fails on that call and never panics.

func (remoteApps) Frontmost() (api.App, error) {
	return api.App{}, unsupported("ctx.desktop.apps.frontmost")
}

func (remoteFiles) QuickLook(string) error { return unsupported("ctx.desktop.files.quickLook") }

func (remoteFiles) ReadText(string) (string, error) {
	return "", unsupported("ctx.desktop.files.readText")
}

func (remoteFiles) WriteText(string, string, bool) (api.FileRef, error) {
	return api.FileRef{}, unsupported("ctx.desktop.files.writeText")
}

// DisplayPath returns the path unchanged: the host shows paths as they are until it serves its own display names.
func (remoteFiles) DisplayPath(path string) string { return path }

func (remoteFiles) IndexedRoots() []string { return nil }

func (remoteFiles) IndexSnapshot(api.IndexOptions) []api.IndexedFile { return nil }

func (remoteFiles) Reindex(api.IndexOptions) (api.IndexReport, error) {
	return api.IndexReport{}, unsupported("ctx.desktop.files.index")
}

func (remoteFiles) Recent(api.IndexOptions) []api.IndexedFile { return nil }

func (remoteFiles) SearchIndex(string, api.IndexOptions) []api.IndexedFile { return nil }

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
