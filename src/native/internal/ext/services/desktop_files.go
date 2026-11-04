package services

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveDesktopFiles answers the files service: opening and revealing files in other apps, finding files, and
// describing, previewing and naming them. Paths must be absolute, so a relative name never resolves against the app's
// working folder.
func (h *Handler) serveDesktopFiles(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopFiles); err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpOpen, protocol.OpReveal:
		return nil, h.openOrRevealPath(call)
	case protocol.OpFind:
		return h.findFiles(ctx, call.Args)
	case protocol.OpMetadata:
		return h.fileMetadata(call.Args)
	case protocol.OpThumbnail:
		return h.fileThumbnail(call.Args)
	case protocol.OpFileURL:
		return h.fileURLOf(call.Args)
	case protocol.OpOpenWithApps:
		return h.openWithApps(ctx, call.Args)
	}
	return nil, invalid("desktop.files has no operation %q", call.Op)
}

func (h *Handler) openOrRevealPath(call protocol.ServiceCall) error {
	if h.deps.Shell == nil {
		return unavailable("files are not available in this build")
	}
	path, err := absolutePathArg(call.Args)
	if err != nil {
		return err
	}
	if !h.actions.allow() {
		return rateExceeded("file opens")
	}
	if call.Op == protocol.OpReveal {
		return h.deps.Shell.Reveal(path)
	}
	return h.deps.Shell.OpenPath(path)
}

func absolutePathArg(raw json.RawMessage) (string, error) {
	var args protocol.PathArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if !filepath.IsAbs(args.Path) {
		return "", invalid("a file path must be absolute")
	}
	return filepath.Clean(args.Path), nil
}
