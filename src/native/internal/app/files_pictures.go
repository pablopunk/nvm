package app

import (
	"github.com/egoist/mygo/ui"
)

// rowPicture returns the bitmap a row draws for path. For an image it is the thumbnail, and while that is being made
// the row shows no picture rather than the generic icon, so the tile does not change twice. Any other path shows the
// system icon of its kind. Both come from the caches: the call never decodes or asks the OS on the UI thread.
func (s *fileService) rowPicture(path string) *ui.Bitmap {
	if isPictureName(path) {
		bm, ready := s.thumbs.Get(path, func() *ui.Bitmap { return s.thumbnailFor(path) })
		if !ready {
			return nil
		}
		if bm != nil {
			return bm
		}
	}
	bm, _ := s.icons.Get(systemIconKey(path), func() *ui.Bitmap { return s.systemIcon(path) })
	return bm
}

// thumbnailFor makes the list thumbnail of an image. It runs on a worker: it stats the file for its version, reads or
// renders the picture through the disk cache and decodes the PNG. A failure is silent and returns nil.
func (s *fileService) thumbnailFor(path string) *ui.Bitmap {
	info, err := s.fsys.Lstat(path)
	if err != nil {
		return nil
	}
	cache, err := s.thumbnails()
	if err != nil {
		return nil
	}
	data, err := cache.Thumbnail(path, info.ModTime().UnixMilli(), info.Size())
	if err != nil {
		return nil
	}
	bm, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bm
}

// systemIcon asks the operating system for the icon of path and decodes it. It runs on a worker; a failure is nil.
func (s *fileService) systemIcon(path string) *ui.Bitmap {
	data, err := s.shell.AppIcon(path, iconSize*2)
	if err != nil {
		return nil
	}
	bm, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bm
}
