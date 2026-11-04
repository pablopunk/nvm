package services

import (
	"encoding/json"
	"net/url"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveDesktopFiles opens and reveals files and folders in other apps. The paths must be absolute, so a relative
// name never resolves against the app's working folder.
func (h *Handler) serveDesktopFiles(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopFiles); err != nil {
		return nil, err
	}
	if h.deps.Shell == nil {
		return nil, unavailable("files are not available in this build")
	}
	switch call.Op {
	case protocol.OpOpen, protocol.OpReveal:
		return nil, h.openOrRevealPath(call)
	}
	return nil, invalid("desktop.files has no operation %q", call.Op)
}

func (h *Handler) openOrRevealPath(call protocol.ServiceCall) error {
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

// serveDesktopShell opens web links in the default browser. Only http and https links are accepted.
func (h *Handler) serveDesktopShell(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopFiles); err != nil {
		return nil, err
	}
	if h.deps.Shell == nil {
		return nil, unavailable("links are not available in this build")
	}
	if call.Op != protocol.OpOpenExternal {
		return nil, invalid("desktop.shell has no operation %q", call.Op)
	}
	var args protocol.URLArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return nil, err
	}
	if !webLink(args.URL) {
		return nil, invalid("only http and https links can be opened")
	}
	if !h.actions.allow() {
		return nil, rateExceeded("link opens")
	}
	return nil, h.deps.Shell.OpenURL(args.URL)
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

func webLink(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
