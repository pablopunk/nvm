package services

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// platformRefusal is the defence in depth of api.PlatformOperations: an extension whose source calls an operation that
// this OS lacks is never run here, so the call should not arrive. If it does, for a source the static check could not
// read, it fails with api.ErrUnsupported before anything is attempted. The failure is returned to the extension only:
// nothing is shown to the user, since a feature of another OS missing here is not news to them. An unknown OS (empty)
// refuses nothing.
func (h *Handler) platformRefusal(call protocol.ServiceCall) error {
	operation, ok := api.PlatformOperationOf(call.Service, call.Op)
	if !ok || h.deps.OS == "" || operation.Platforms.Includes(h.deps.OS) {
		return nil
	}
	return fmt.Errorf("%s works on %s only: %w", operation.Name(), api.OSName(operation.Platforms[0]), api.ErrUnsupported)
}
