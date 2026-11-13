package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteHistory is the clipboard history the host keeps.
type remoteHistory struct {
	link *hostLink
}

func (h remoteHistory) List(query api.ClipboardQuery) ([]api.ClipboardEntry, error) {
	var entries []api.ClipboardEntry
	err := h.link.Call(protocol.ServiceClipboardHistory, protocol.OpList, query, &entries)
	return entries, err
}

func (h remoteHistory) Get(id string) (api.ClipboardEntry, bool, error) {
	var result protocol.ClipboardGetResult
	err := h.link.Call(protocol.ServiceClipboardHistory, protocol.OpGet, protocol.KeyArgs{Key: id}, &result)
	return result.Entry, result.Found, err
}

func (h remoteHistory) Remove(ids []string) (int, error) {
	var removed int
	err := h.link.Call(protocol.ServiceClipboardHistory, protocol.OpRemove, protocol.IDsArgs{IDs: ids}, &removed)
	return removed, err
}

func (h remoteHistory) Clear(types []string, olderThanMs int64) (int, error) {
	var removed int
	err := h.link.Call(protocol.ServiceClipboardHistory, protocol.OpClear, protocol.ClearArgs{Types: types, OlderThanMs: olderThanMs}, &removed)
	return removed, err
}
