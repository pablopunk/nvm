package services

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// indicatorUpdatesPerMinute bounds how often an extension may redraw its passive indicator.
const indicatorUpdatesPerMinute = 120

// IndicatorSurface draws the passive indicators of extensions. The owner is the extension id, so the surface keeps
// each extension's indicators apart from every other's. Showing an indicator that is already shown replaces it.
type IndicatorSurface interface {
	ShowIndicator(owner string, input api.IndicatorInput)
	HideIndicator(owner, id string)
}

// serveIndicators answers the indicator service. Indicators need no declaration: they are passive and take no input.
func (h *Handler) serveIndicators(call protocol.ServiceCall) (any, error) {
	if h.deps.Indicators == nil {
		return nil, unavailable("indicators are not available in this build")
	}
	switch call.Op {
	case protocol.OpShow, protocol.OpUpdate:
		return nil, h.showIndicator(call)
	case protocol.OpHide:
		var args protocol.IndicatorHideArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		h.deps.Indicators.HideIndicator(h.manifest.ID, args.ID)
		return nil, nil
	}
	return nil, invalid("indicators has no operation %q", call.Op)
}

func (h *Handler) showIndicator(call protocol.ServiceCall) error {
	var input api.IndicatorInput
	if err := decodeArgs(call.Args, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Title) == "" {
		return invalid("an indicator needs a title")
	}
	if !h.indicatorUpdates.allow() {
		return rateExceeded("indicator updates")
	}
	h.deps.Indicators.ShowIndicator(h.manifest.ID, input)
	return nil
}
