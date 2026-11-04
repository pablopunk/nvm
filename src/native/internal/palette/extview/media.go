package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// imageRef picks the source an image draws from: Src, else the light variant, else the dark one, else the fallback.
// It is "" for an image with no source.
func imageRef(image *api.Image) string {
	if image == nil {
		return ""
	}
	for _, ref := range []string{image.Src, image.Light, image.Dark, image.Fallback} {
		if ref != "" {
			return ref
		}
	}
	return ""
}

// bitmap returns the decoded image of an item or detail, or nil while it loads or when it cannot be loaded. The first
// call starts the load; the palette redraws when it finishes.
func (s *Session) bitmap(image *api.Image) *ui.Bitmap {
	ref := imageRef(image)
	if ref == "" {
		return nil
	}
	if bm, ok := s.images[ref]; ok {
		return bm
	}
	s.requestImage(ref)
	return nil
}

// requestImage starts the load of ref once. A failed load is remembered as nil, so it is not retried on every frame.
func (s *Session) requestImage(ref string) {
	if s.requested[ref] || s.deps.LoadImage == nil {
		return
	}
	s.requested[ref] = true
	s.deps.LoadImage(ref, func(bm *ui.Bitmap) {
		if s.dismissed {
			return
		}
		s.images[ref] = bm
		s.refreshHost()
	})
}
