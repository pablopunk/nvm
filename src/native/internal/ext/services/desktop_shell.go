package services

import (
	"context"
	"net/url"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveDesktopShell answers the shell service: opening web links, and running the programs and scripts that the
// process operations below serve.
func (h *Handler) serveDesktopShell(ctx context.Context, call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpOpenExternal:
		return nil, h.openExternal(call)
	case protocol.OpExec, protocol.OpScript, protocol.OpAppleScript, protocol.OpWhich:
		return h.runProcess(ctx, call)
	}
	return nil, invalid("desktop.shell has no operation %q", call.Op)
}

// openExternal opens a web link in the default browser. Only http and https links are accepted.
func (h *Handler) openExternal(call protocol.ServiceCall) error {
	if err := h.require(api.CapabilityDesktopFiles); err != nil {
		return err
	}
	if h.deps.Shell == nil {
		return unavailable("links are not available in this build")
	}
	var args protocol.URLArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return err
	}
	if !webLink(args.URL) {
		return invalid("only http and https links can be opened")
	}
	if !h.actions.allow() {
		return rateExceeded("link opens")
	}
	return h.deps.Shell.OpenURL(args.URL)
}

func webLink(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
