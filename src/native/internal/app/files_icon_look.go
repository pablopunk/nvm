package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// refreshIconLook redraws the system icons when the system's icon look changed, as a tint set by a wallpaper does. It runs
// off the UI thread, so showing the palette never waits for it.
func (s *fileService) refreshIconLook() {
	looks, ok := s.pictures.(platform.IconLooks)
	if !ok {
		return
	}
	safego.Go("files.icon-look", func(ctx context.Context) { s.applyIconLook(ctx, looks) })
}

func (s *fileService) applyIconLook(ctx context.Context, looks platform.IconLooks) {
	look, changed := looks.IconLook(ctx)
	previous := s.iconLook.Swap(&look)
	s.themedIcons.Store(changed)
	if (previous == nil && changed) || (previous != nil && *previous != look) {
		s.icons.Clear()
		s.redraw()
	}
}

// systemIconPNG is the icon of path in the look the system has now. A running app can only draw the look it launched
// with, so a changed look comes from the helper.
func (s *fileService) systemIconPNG(path string) ([]byte, error) {
	if looks, ok := s.pictures.(platform.IconLooks); ok && s.themedIcons.Load() {
		if data, err := looks.ThemedIcon(s.ctx, path, iconSize*2); err == nil {
			return data, nil
		}
	}
	return s.shell.AppIcon(path, iconSize*2)
}
