package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteUpdates reads the state of the app's updater from the host. A failed exchange reads as an empty state.
type remoteUpdates struct {
	link *hostLink
}

func (u remoteUpdates) State() api.UpdateState {
	var state api.UpdateState
	_ = u.link.Call(protocol.ServiceUpdates, protocol.OpState, nil, &state)
	return state
}
