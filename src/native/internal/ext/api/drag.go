package api

import (
	"errors"
	"path/filepath"
)

// DragPaths returns the files a row or grid cell drags out to other applications, as 1.x's palette does: its File,
// else the path of the first action that names one (Primary, then Actions, then the action panel). Relative paths
// never drag, so the result holds only absolute paths.
func DragPaths(item Item) []string {
	if item.File != nil && filepath.IsAbs(item.File.Path) {
		return []string{item.File.Path}
	}
	for _, action := range item.dragCandidateActions() {
		if filepath.IsAbs(action.Path) {
			return []string{action.Path}
		}
	}
	return nil
}

// DragImagePath returns the image file a drag of item shows, or "" when the item sets none.
func DragImagePath(item Item) string {
	if item.DragImage == nil {
		return ""
	}
	return item.DragImage.Path
}

func (item Item) dragCandidateActions() []Action {
	var out []Action
	if item.Primary != nil {
		out = append(out, *item.Primary)
	}
	out = append(out, item.Actions...)
	if item.ActionPanel != nil {
		for _, section := range item.ActionPanel.Sections {
			out = append(out, section.Actions...)
		}
	}
	return out
}

// validateDragImage checks the optional drag image: it names an absolute image file and needs a File to drag.
func (item Item) validateDragImage() error {
	if item.DragImage == nil {
		return nil
	}
	if item.File == nil {
		return errors.New("dragImage needs a file to drag")
	}
	if !filepath.IsAbs(item.DragImage.Path) {
		return errors.New("dragImage path must be absolute")
	}
	return nil
}
