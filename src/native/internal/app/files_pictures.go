package app

import (
	"context"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
)

// gridPictureEntries bounds the decoded grid pictures kept between opens: a few hundred megabytes at most, and enough for
// every tile of the largest grids.
const gridPictureEntries = 160

// gridPicturePermits is how many grid pictures are made at once.
var gridPicturePermits = make(chan struct{}, 4)

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

// gridPicture is the picture of one tile of an extension grid. A picture decoded earlier comes straight from memory, so
// reopening a grid paints at once; after a launch it comes from the disk cache, and only a file never seen is scaled. It
// runs on a worker and answers nil when the file has no picture.
func (s *fileService) gridPicture(ctx context.Context, path string) *ui.Bitmap {
	info, err := s.fsys.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	mtimeMs, size := info.ModTime().UnixMilli(), info.Size()
	key := path + "\x00" + strconv.FormatInt(mtimeMs, 10) + "\x00" + strconv.FormatInt(size, 10)
	if bm, ok := s.gridPictures.Peek(key); ok {
		return bm
	}
	select {
	case gridPicturePermits <- struct{}{}:
	case <-ctx.Done():
		return nil
	}
	defer func() { <-gridPicturePermits }()
	if bm, ok := s.gridPictures.Peek(key); ok {
		return bm
	}
	cache, err := s.thumbnails()
	if err != nil {
		return nil
	}
	data, err := cache.Rendered(path, mtimeMs, size, thumbs.GridEdge)
	if err != nil {
		return nil
	}
	bm, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	s.gridPictures.Put(key, bm)
	return bm
}
