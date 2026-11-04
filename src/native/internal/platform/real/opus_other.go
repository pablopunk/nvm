//go:build !darwin && !linux

package real

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// opusCodec reports ErrUnsupported on Windows until libopus is bundled with the app; purego cannot open
// libraries there without x/sys/windows.
type opusCodec struct{}

// NewEncoder reports ErrUnsupported.
func (opusCodec) NewEncoder(platform.AudioFormat) (platform.OpusEncoder, error) {
	return nil, fmt.Errorf("%w: Opus encoding is not available on this system yet", platform.ErrUnsupported)
}
