package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

type remoteSelf struct {
	link *hostLink
}

func (s remoteSelf) Rename(change api.ExtensionRename) error {
	return s.link.Call(protocol.ServiceExtension, protocol.OpRename, change, nil)
}
