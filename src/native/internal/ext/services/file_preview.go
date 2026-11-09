package services

import (
	"context"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

type filePreviewOrigin uint8

const (
	filePreviewService filePreviewOrigin = iota
	filePreviewDeclaredAction
)

func (h *Handler) filePreview() platform.FilePreview {
	return platform.BindFilePreview(h.deps.Shell, h.deps.Capabilities)
}

// PreviewFileAction preserves the declarative action's existing host-owned access policy.
// The manager must bind it to the enabled installation that supplied the action.
func (h *Handler) PreviewFileAction(ctx context.Context, path string) error {
	return h.showFilePreview(ctx, path, filePreviewDeclaredAction)
}

func (h *Handler) showFilePreview(ctx context.Context, path string, origin filePreviewOrigin) error {
	if h.closed.Load() {
		return errHandlerClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	switch origin {
	case filePreviewService:
		if err := h.require(api.CapabilityDesktopFiles); err != nil {
			return err
		}
	case filePreviewDeclaredAction:
	default:
		return unavailable("this file preview invocation is not supported")
	}
	if !filepath.IsAbs(path) {
		return invalid("a file path must be absolute")
	}
	preview := h.filePreview()
	if !preview.Supported() {
		return unavailable("file preview is not available on this system")
	}
	if origin == filePreviewService && !h.actions.allow() {
		return rateExceeded("file opens")
	}
	if h.closed.Load() {
		return errHandlerClosed
	}
	return unsupportedAsUnavailable(preview.Show(ctx, filepath.Clean(path)), "file preview is not available on this system")
}
