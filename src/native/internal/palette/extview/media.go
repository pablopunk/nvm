package extview

import (
	"context"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// imageRef picks the source an image draws from: Src, else the light variant, else the dark one, else the fallback.
// It is "" for an image with no source.
// imageShape is the corners an image asks for. Mask is another name for Shape, which wins when both are set.
func imageShape(image *api.Image) string {
	if image == nil {
		return ""
	}
	if image.Shape != "" {
		return string(image.Shape)
	}
	return string(image.Mask)
}

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
	s.guard.Lock()
	defer s.guard.Unlock()
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
	ctx := s.imageContext
	if ctx == nil {
		ctx = context.Background()
	}
	s.deps.LoadImage(ctx, ref, func(bm *ui.Bitmap) {
		if s.dismissed || ctx.Err() != nil {
			return
		}
		s.images[ref] = bm
		s.refreshHost()
	})
}
