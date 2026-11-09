package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteViews is Views backed by the host. RefreshMenuBar asks the host to re-render this extension's menu bar items
// in the background and returns at once. Refresh returns a host-owned palette action. Invalidate is routed to the host.
type remoteViews struct {
	link *hostLink
}

func (v remoteViews) Refresh() api.Action {
	return api.Action{Kind: api.ActionRefreshView, Title: "Refresh"}
}

// Invalidate drops the snapshots of this extension's data sources and asks the host to refresh its visible view, so
// what a handler changed shows now and on the next open instead of the last cached rows.
func (v remoteViews) Invalidate() {
	_ = v.link.Call(protocol.ServiceViews, protocol.OpInvalidateViews, nil, nil)
}

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
