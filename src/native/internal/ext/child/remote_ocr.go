package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteOCR reads text from images and the screen on the host.
type remoteOCR struct {
	link *hostLink
}

func (o remoteOCR) Image(input api.OCRInput, options api.OCROptions) (api.OCRResult, error) {
	var result api.OCRResult
	err := o.link.Call(protocol.ServiceOCR, protocol.OpImage, protocol.OCRImageArgs{Input: input, Options: options}, &result)
	return result, err
}

func (o remoteOCR) Screen(options api.OCROptions) (api.OCRResult, error) {
	var result api.OCRResult
	err := o.link.Call(protocol.ServiceOCR, protocol.OpScreen, protocol.OCRScreenArgs{Options: options}, &result)
	return result, err
}

func (o remoteOCR) Region(region api.Rect, options api.OCROptions) (api.OCRResult, error) {
	var result api.OCRResult
	err := o.link.Call(protocol.ServiceOCR, protocol.OpRegion, protocol.OCRRegionArgs{Region: region, Options: options}, &result)
	return result, err
}
