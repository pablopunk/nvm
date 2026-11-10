package extview

import (
	"context"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
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
	bm, loaded := s.images[ref]
	authorized := s.imageAuthority[ref]
	generation := s.imageGeneration
	version := s.imageSnapshotVersions[ref]
	s.guard.Unlock()
	if loaded {
		if authorized != nil && !authorized() {
			s.onUI(func() {
				s.applyGridDenial(ref, generation, version, &api.SnapshotDenial{AuthorityDenied: true, Failure: &api.Failure{Code: api.CodePermission, Message: "File access was revoked."}})
			})
			return nil
		}
		return bm
	}
	s.requestImage(ref)
	return nil
}

// requestImage starts decoration once and binds its completion to the current view generation.
func (s *Session) requestImage(ref string) {
	if ref == "" {
		return
	}
	grid := thumbs.IsGridReference(ref)
	s.guard.Lock()
	if s.requested[ref] || (grid && s.deps.LoadGridImage == nil) || (!grid && s.deps.LoadImage == nil) {
		s.guard.Unlock()
		return
	}
	s.requested[ref] = true
	ctx := s.imageContext
	generation := s.imageGeneration
	s.guard.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	if grid {
		var version uint64
		if s.snapshotRef != nil && s.deps.CommandSnapshots != nil {
			version = s.deps.CommandSnapshots.Version(s.snapshotIdentity, *s.snapshotRef)
		}
		request := GridImageRequest{ExtensionID: s.extID, Identity: s.installationIdentity, Source: ref, Command: s.snapshotRef, Generation: generation, SnapshotVersion: version}
		s.deps.LoadGridImage(ctx, request, func(result GridImageResult) {
			s.onUI(func() {
				if s.dismissed || ctx.Err() != nil {
					return
				}
				s.guard.Lock()
				current := generation == s.imageGeneration
				s.guard.Unlock()
				if !current {
					return
				}
				if result.Denial != nil {
					s.applyGridDenial(ref, generation, version, result.Denial)
					return
				}
				if result.Authorized != nil && !result.Authorized() {
					s.applyGridDenial(ref, generation, version, &api.SnapshotDenial{AuthorityDenied: true, Failure: &api.Failure{Code: api.CodePermission, Message: "File access was revoked."}})
					return
				}
				s.guard.Lock()
				s.images[ref] = result.Bitmap
				if s.imageAuthority == nil {
					s.imageAuthority = make(map[string]func() bool)
				}
				s.imageAuthority[ref] = result.Authorized
				if s.imageSnapshotVersions == nil {
					s.imageSnapshotVersions = make(map[string]uint64)
				}
				s.imageSnapshotVersions[ref] = version
				s.guard.Unlock()
				s.refreshHost()
			})
		})
		return
	}
	s.deps.LoadImage(ctx, ref, func(bm *ui.Bitmap) {
		s.onUI(func() {
			if s.dismissed || ctx.Err() != nil {
				return
			}
			s.guard.Lock()
			if generation != s.imageGeneration {
				s.guard.Unlock()
				return
			}
			s.images[ref] = bm
			s.guard.Unlock()
			s.refreshHost()
		})
	})
}
