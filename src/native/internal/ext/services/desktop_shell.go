package services

import (
	"context"
	"net/url"
	"strings"

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
	if !externalLink(args.URL) {
		return invalid("only http, https and mailto links can be opened")
	}
	if !h.actions.allow() {
		return rateExceeded("link opens")
	}
	return h.deps.Shell.OpenURL(args.URL)
}

// externalLink reports whether a link may open outside the palette. It is the Electron host's rule: http and https
// need a host, and mailto is allowed; every other scheme is refused.
func externalLink(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return true
	}
	return false
}
