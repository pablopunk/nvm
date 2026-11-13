package extview

import (
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"net/url"
	"path/filepath"
	"strings"
)

func snapshotDeniedPath(e *api.SnapshotEvidence, path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	if e.FilesAuthorityDenied {
		return true
	}
	for _, denied := range e.DeniedPaths {
		if filepath.Clean(path) == filepath.Clean(denied) {
			return true
		}
	}
	return false
}
func snapshotDeniedSource(e *api.SnapshotEvidence, stale map[[32]byte]bool, src string) bool {
	if strings.HasPrefix(src, "data:image/png;base64,") {
		hash := sha256.Sum256([]byte(src))
		return stale[hash] && !e.ThumbnailHashes[hash]
	}
	u, err := url.Parse(src)
	return err == nil && strings.EqualFold(u.Scheme, "file") && snapshotDeniedPath(e, platform.FileURLPath(u))
}
func snapshotSafeAction(action *api.Action, e *api.SnapshotEvidence, stale map[[32]byte]bool, depth int) bool {
	if action == nil || depth > 16 {
		return false
	}
	if snapshotDeniedPath(e, action.Path) || snapshotDeniedSource(e, stale, action.URL) {
		return false
	}
	for _, p := range action.Paths {
		if snapshotDeniedPath(e, p) {
			return false
		}
	}
	if content := action.Content; content != nil {
		if snapshotDeniedPath(e, content.ImagePath) || snapshotDeniedSource(e, stale, content.ImageDataURL) {
			return false
		}
		for _, p := range content.Paths {
			if snapshotDeniedPath(e, p) {
				return false
			}
		}
	}
	if action.View != nil && !snapshotSafeNestedView(action.View, e, stale, depth+1) {
		return false
	}
	if action.Window != nil && action.Window.View != nil && !snapshotSafeNestedView(action.Window.View, e, stale, depth+1) {
		return false
	}
	if action.Submenu != nil {
		snapshotFilterPanel(action.Submenu, e, stale, depth+1)
	}
	return true
}
func snapshotFilterActions(actions []api.Action, e *api.SnapshotEvidence, stale map[[32]byte]bool, depth int) []api.Action {
	kept := actions[:0]
	for _, action := range actions {
		if snapshotSafeAction(&action, e, stale, depth) {
			kept = append(kept, action)
		}
	}
	return kept
}
func snapshotFilterPanel(panel *api.ActionPanel, e *api.SnapshotEvidence, stale map[[32]byte]bool, depth int) {
	if panel == nil {
		return
	}
	for i := range panel.Sections {
		panel.Sections[i].Actions = snapshotFilterActions(panel.Sections[i].Actions, e, stale, depth)
	}
}
func snapshotSafeNestedView(view *api.View, e *api.SnapshotEvidence, stale map[[32]byte]bool, depth int) bool {
	if depth > 16 {
		return false
	}
	if view.File != nil && snapshotDeniedPath(e, view.File.File.Path) {
		return false
	}
	if view.Preview != nil {
		p := view.Preview
		if p.Image != nil && (snapshotDeniedSource(e, stale, p.Image.Src) || snapshotDeniedSource(e, stale, p.Image.Light) || snapshotDeniedSource(e, stale, p.Image.Dark) || snapshotDeniedSource(e, stale, p.Image.Fallback)) {
			p.Image = nil
		}
		if snapshotDeniedSource(e, stale, p.Video) {
			p.Video = ""
		}
	}
	result := api.Result{View: view, SnapshotEvidence: e}
	filterSnapshotDeniedDepth(&result, stale, depth)
	return true
}
