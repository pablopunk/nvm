package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// gridPictureEntries bounds the decoded grid pictures kept between opens: a few hundred megabytes at most, and enough for
// every tile of the largest grids.
const gridPictureEntries = 160

// A grid picture is made ahead of time by this many workers, and this many files are remembered as already made.
const (
	gridWarmWorkers = 2
	gridWarmEntries = 2000
)

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

// gridPicture authorizes a readable source before serving any cached pixels. It is the picture of one tile of an extension grid. A picture decoded earlier comes straight from memory, so
// reopening a grid paints at once; after a launch it comes from the disk cache, and only a file never seen is scaled. It
// runs on a worker and answers nil when the file has no picture.
func (s *fileService) gridPicture(ctx context.Context, path string) *ui.Bitmap {
	select {
	case gridPicturePermits <- struct{}{}:
	case <-ctx.Done():
		return nil
	}
	defer func() { <-gridPicturePermits }()

	sources, ok := s.fsys.(platform.FileSources)
	if !ok {
		return nil
	}
	source, err := sources.OpenReadNoFollow(path)
	if err != nil {
		return nil
	}
	defer source.Close()
	return s.gridPictureFromSource(ctx, path, source)
}

// gridPictureFromSource renders only from the caller's already authorized source.
func (s *fileService) gridPictureFromSource(ctx context.Context, path string, source platform.ReadSource) *ui.Bitmap {
	bitmap, _ := s.gridPictureResult(ctx, path, source)
	return bitmap
}

func (s *fileService) gridPictureResult(ctx context.Context, path string, source platform.ReadSource) (*ui.Bitmap, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	info, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("picture source is not regular")
	}
	versionTime, size := info.ModTime().UnixNano(), info.Size()
	cachePath := path + "\x00" + source.Identity()
	key := cachePath + "\x00" + strconv.FormatInt(versionTime, 10) + "\x00" + strconv.FormatInt(size, 10)
	if bm, ok := s.gridPictures.Peek(key); ok {
		return bm, nil
	}
	cache, err := s.thumbnails()
	if err != nil {
		return nil, err
	}
	data, err := cache.RenderedWith(cachePath, versionTime, size, thumbs.GridEdge, func() ([]byte, error) {
		data, err := s.drawGridPicture(ctx, source, path)
		if err != nil {
			return nil, err
		}
		after, err := source.Stat()
		if err != nil {
			return nil, err
		}
		if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return nil, errors.New("picture source changed")
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	bm, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.gridPictures.Put(key, bm)
	return bm, nil
}

// drawGridPicture draws a file the cache does not have: with the system's own pictures where there are any, which cover
// videos, documents and every other kind, and by decoding the image otherwise.
func (s *fileService) drawGridPicture(ctx context.Context, source platform.ReadSource, path string) ([]byte, error) {
	if pictures, ok := s.pictures.(platform.SourceFilePictures); ok {
		if data, err := pictures.FilePictureFromSource(ctx, source, path, thumbs.GridEdge); err == nil {
			return data, nil
		}
	}
	return thumbs.RenderSource(source, thumbs.GridEdge)
}

// prepareGridPicture makes the picture of a file ahead of time, so a grid that lists the file later finds it ready. It
// returns at once; each version of a file is prepared once.
func (s *fileService) prepareGridPicture(path string) {
	info, err := s.fsys.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	key := path + "\x00" + strconv.FormatInt(info.ModTime().UnixMilli(), 10) + "\x00" + strconv.FormatInt(info.Size(), 10)
	s.gridWarmups.Get(key, func() bool { return s.gridPicture(s.ctx, path) != nil })
}
