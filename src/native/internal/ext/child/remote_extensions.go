package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteExtensions reads and changes the installed extensions through the host. A failed read reads as none.
type remoteExtensions struct {
	link *hostLink
}

func (e remoteExtensions) List() []api.ExtensionRecord {
	var records []api.ExtensionRecord
	_ = e.link.Call(protocol.ServiceExtensions, protocol.OpList, nil, &records)
	return records
}

// Disable asks the host to turn an extension off, and returns the reason it refused.
func (e remoteExtensions) Disable(id string) error {
	return e.link.Call(protocol.ServiceExtensions, protocol.OpDisable, protocol.ExtensionArgs{ID: id}, nil)
}

// OpenReview asks the host to show the capability review of a disabled extension. It never enables it.
func (e remoteExtensions) OpenReview(id string) error {
	return e.link.Call(protocol.ServiceExtensions, protocol.OpOpenReview, protocol.ExtensionArgs{ID: id}, nil)
}

// SetMenuBarItem asks the host to show or hide an extension's menu bar item.
func (e remoteExtensions) SetMenuBarItem(id string, shown bool) error {
	return e.link.Call(protocol.ServiceExtensions, protocol.OpSetMenuBarItem, protocol.ExtensionMenuBarArgs{ID: id, Shown: shown}, nil)
}
