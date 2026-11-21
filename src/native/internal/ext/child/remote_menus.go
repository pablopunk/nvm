package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

type remoteMenus struct{ link *hostLink }

func (m remoteMenus) Snapshot() (api.MenuSnapshot, error) {
	var snapshot api.MenuSnapshot
	err := m.link.Call(protocol.ServiceDesktopMenus, protocol.OpSnapshot, nil, &snapshot)
	return snapshot, err
}
func (m remoteMenus) Invoke(handle string) error {
	return m.link.Call(protocol.ServiceDesktopMenus, protocol.OpInvoke, protocol.HandleArgs{Handle: handle}, nil)
}
