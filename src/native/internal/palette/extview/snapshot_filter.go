package extview

import (
	"context"
	"crypto/sha256"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func snapshotEvidenceDeniesPaths(e *api.SnapshotEvidence, paths map[[32]byte]bool) bool {
	if e == nil || len(paths) == 0 {
		return false
	}
	if e.FilesAuthorityDenied {
		return true
	}
	for _, p := range e.DeniedPaths {
		if paths[sha256.Sum256([]byte(filepath.Clean(p)))] {
			return true
		}
	}
	return false
}

func (s *Session) restoreSnapshotPresentation(result api.Result) {
	if result.View == nil || s.imageContext == nil || s.imageContext.Err() == nil {
		return
	}
	s.imageContext, s.cancelImages = context.WithCancel(context.Background())
	s.view.Title, s.view.Subtitle = result.View.Title, result.View.Subtitle
	s.view.Placeholder, s.view.Hint = result.View.Placeholder, result.View.Hint
	if s.palettePage != nil {
		s.palettePage.Title, s.palettePage.Subtitle = s.view.Title, s.view.Subtitle
		s.palettePage.Placeholder, s.palettePage.Hint = s.view.Placeholder, s.view.Hint
	}
}

func (s *Session) rememberSnapshotImages() {
	if s.snapshotStaleHashes == nil {
		s.snapshotStaleHashes = make(map[[32]byte]bool)
	}
	for _, item := range s.items {
		if item.Image != nil && strings.HasPrefix(item.Image.Src, "data:image/png;base64,") {
			s.snapshotStaleHashes[sha256.Sum256([]byte(item.Image.Src))] = true
		}
	}
}

// filterSnapshotDenied strips only stale or denied file-backed visuals from a fresh extension result.
func filterSnapshotDenied(result *api.Result, stale map[[32]byte]bool) {
	filterSnapshotDeniedDepth(result, stale, 0)
}

func filterSnapshotDeniedDepth(result *api.Result, stale map[[32]byte]bool, depth int) {
	e := result.SnapshotEvidence
	if e == nil || (!e.FilesAuthorityDenied && len(e.DeniedPaths) == 0) {
		return
	}
	denied := make(map[string]bool, len(e.DeniedPaths))
	for _, path := range e.DeniedPaths {
		denied[filepath.Clean(path)] = true
	}
	deniedPath := func(path string) bool { return path != "" && (e.FilesAuthorityDenied || denied[filepath.Clean(path)]) }
	strip := func(src string) bool {
		if src == "" {
			return false
		}
		if strings.HasPrefix(src, "data:image/png;base64,") {
			hash := sha256.Sum256([]byte(src))
			return stale[hash] && !e.ThumbnailHashes[hash]
		}
		u, err := url.Parse(src)
		if err != nil || !strings.EqualFold(u.Scheme, "file") {
			return false
		}
		return deniedPath(platform.FileURLPath(u))
	}
	clean := func(items []api.Item) {
		for i := range items {
			item := &items[i]
			unsafe := false
			pathDenied := (filepath.IsAbs(item.ID) && deniedPath(item.ID)) || (item.File != nil && deniedPath(item.File.Path))
			if item.Image != nil {
				image := *item.Image
				if strip(image.Src) {
					image.Src = ""
					unsafe = true
				}
				if strip(image.Light) {
					image.Light = ""
					unsafe = true
				}
				if strip(image.Dark) {
					image.Dark = ""
					unsafe = true
				}
				if strip(image.Fallback) {
					image.Fallback = ""
					unsafe = true
				}
				if image.Src == "" && image.Light == "" && image.Dark == "" && image.Fallback == "" {
					item.Image = nil
				} else {
					item.Image = &image
				}
			}
			if item.File != nil && deniedPath(item.File.Path) {
				item.File = nil
				unsafe = true
			}
			if item.DragImage != nil && deniedPath(item.DragImage.Path) {
				item.DragImage = nil
				unsafe = true
			}
			if strip(item.Video) {
				item.Video = ""
				unsafe = true
			}
			if item.Detail != nil {
				if item.Detail.File != nil && deniedPath(item.Detail.File.Path) {
					item.Detail = nil
					unsafe = true
				} else if item.Detail.Image != nil && (strip(item.Detail.Image.Src) || strip(item.Detail.Image.Light) || strip(item.Detail.Image.Dark) || strip(item.Detail.Image.Fallback)) {
					item.Detail = nil
					unsafe = true
				}
			}
			if filepath.IsAbs(item.ID) && deniedPath(item.ID) {
				unsafe = true
			}
			if pathDenied {
				item.Image = nil
			}
			if unsafe {
				item.Primary = nil
				item.Actions = nil
				item.ActionPanel = nil
				item.Detail = nil
				item.Video = ""
				if e.FilesAuthorityDenied {
					item.Title = "Unavailable"
					item.Subtitle = ""
					item.Disabled = true
				}
			} else {
				if item.Primary != nil && !snapshotSafeAction(item.Primary, e, stale, depth+1) {
					item.Primary = nil
				}
				item.Actions = snapshotFilterActions(item.Actions, e, stale, depth+1)
				snapshotFilterPanel(item.ActionPanel, e, stale, depth+1)
				if item.Detail != nil {
					item.Detail.Actions = snapshotFilterActions(item.Detail.Actions, e, stale, depth+1)
				}
			}
		}
	}
	if result.View != nil {
		v := result.View
		if v.File != nil && deniedPath(v.File.File.Path) {
			v.File = nil
			v.Kind = api.ViewPreview
			v.Title = "Access unavailable"
			v.Preview = &api.PreviewSpec{Text: "File access was denied."}
		}
		if v.Preview != nil {
			p := v.Preview
			if p.Image != nil && (strip(p.Image.Src) || strip(p.Image.Light) || strip(p.Image.Dark) || strip(p.Image.Fallback)) {
				p.Image = nil
			}
			if strip(p.Video) {
				p.Video = ""
			}
		}
		clean(v.Items)
		for i := range v.Sections {
			clean(v.Sections[i].Items)
		}
		v.Actions = snapshotFilterActions(v.Actions, e, stale, depth+1)
		snapshotFilterPanel(v.ActionPanel, e, stale, depth+1)
	}
	if result.Patch != nil {
		clean(result.Patch.Items)
		clean(result.Patch.SearchItems)
	}
	if result.Next != nil && !snapshotSafeAction(result.Next, e, stale, depth+1) {
		result.Next = nil
	}
}
