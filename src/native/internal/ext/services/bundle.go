package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveBundle answers a read of a dataset the app ships. The records come from the store, which decodes each dataset
// once; the child keeps them for its own later reads.
func (h *Handler) serveBundle(call protocol.ServiceCall) (any, error) {
	if call.Op != protocol.OpRecords {
		return nil, invalid("bundle has no operation %q", call.Op)
	}
	var args protocol.BundleArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return nil, err
	}
	if h.deps.Bundles == nil {
		return nil, api.ErrNotAvailable
	}
	return h.deps.Bundles.Records(args.Name)
}
