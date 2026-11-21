package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteIndicators shows and hides the passive status indicators the host draws for this extension.
type remoteIndicators struct {
	link *hostLink
}

func (i remoteIndicators) Show(input api.IndicatorInput) error {
	return i.link.Call(protocol.ServiceIndicators, protocol.OpShow, input, nil)
}

func (i remoteIndicators) Update(input api.IndicatorInput) error {
	return i.link.Call(protocol.ServiceIndicators, protocol.OpUpdate, input, nil)
}

func (i remoteIndicators) Hide(id string) error {
	return i.link.Call(protocol.ServiceIndicators, protocol.OpHide, protocol.IndicatorHideArgs{ID: id}, nil)
}
