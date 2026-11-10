package fake

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/platform"
	"slices"
	"sync"
)

// Desktops records workspace navigation without touching the OS.
type Desktops struct {
	mu         sync.Mutex
	supported  bool
	directions []string
}

func NewDesktops() *Desktops { return &Desktops{supported: true} }

func (d *Desktops) Supported() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.supported
}

func (d *Desktops) SetSupported(supported bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.supported = supported
}

func (d *Desktops) Switch(ctx context.Context, direction string) (platform.HelperResult, error) {
	if err := ctx.Err(); err != nil {
		return platform.HelperResult{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.supported {
		return platform.HelperResult{}, platform.ErrUnsupported
	}
	d.directions = append(d.directions, direction)
	return platform.HelperResult{}, nil
}

func (d *Desktops) Calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.directions)
}

var _ platform.DesktopSwitcher = (*Desktops)(nil)
