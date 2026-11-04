package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteViews is Views backed by the host. RefreshMenuBar asks the host to re-render this extension's menu bar items
// in the background and returns at once. Refresh and Invalidate are palette view operations: menu bar providers have
// no visible view, so they do nothing here.
type remoteViews struct {
	link *hostLink
}

func (v remoteViews) Refresh() api.Action {
	return api.Action{}
}

func (v remoteViews) Invalidate() {}

func (v remoteViews) RefreshMenuBar() {
	_ = v.link.Call(protocol.ServiceViews, protocol.OpRefreshMenuBar, nil, nil)
}

// InvalidateRoot asks the host to run the root's rows again. The host answers at once and refreshes in the background.
func (v remoteViews) InvalidateRoot() {
	_ = v.link.Call(protocol.ServiceViews, protocol.OpInvalidateRoot, nil, nil)
}

// RefreshSearch asks the host to search again for query. The host answers at once.
func (v remoteViews) RefreshSearch(query string) {
	_ = v.link.Call(protocol.ServiceViews, protocol.OpRefreshSearch, protocol.SearchRefreshArgs{Query: query}, nil)
}
