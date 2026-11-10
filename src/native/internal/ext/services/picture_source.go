package services

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"path/filepath"
)

// OpenPictureSource authorizes a deferred host image read under this installation's files declaration.
// It is host-only; neither the descriptor nor an authorization callback crosses extension IPC.
func (h *Handler) OpenPictureSource(ctx context.Context, path string) (platform.ReadSource, error) {
	if h.closed.Load() {
		return nil, errHandlerClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := h.require(api.CapabilityDesktopFiles); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(path) {
		return nil, invalid("a picture path must be absolute")
	}
	files, ok := h.deps.Files.(platform.FileSources)
	if !ok {
		return nil, unavailable("authorized picture sources are unavailable")
	}
	source, err := files.OpenReadNoFollow(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if h.closed.Load() {
		_ = source.Close()
		return nil, errHandlerClosed
	}
	if err := ctx.Err(); err != nil {
		_ = source.Close()
		return nil, err
	}
	return source, nil
}
