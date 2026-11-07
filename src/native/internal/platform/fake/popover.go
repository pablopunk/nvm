package fake

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Popover is a fake tray popover. It starts hidden and records its size and what it was asked to show.
type Popover struct {
	mu      sync.Mutex
	options platform.PopoverOptions
	visible bool
	focused bool
	shows   int
	x, y    int
}

var _ platform.Popover = (*Popover)(nil)

// Options returns the size the popover was opened with.
func (p *Popover) Options() platform.PopoverOptions {
	return p.options
}

// Update runs fn at once, as the fake's UI thread is the caller's.
func (p *Popover) Update(fn func()) { fn() }

// Show shows the popover and counts it.
func (p *Popover) Show() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visible = true
	p.shows++
}

// Hide hides the popover.
func (p *Popover) Hide() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visible = false
}

// IsVisible reports whether the popover is shown.
func (p *Popover) IsVisible() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.visible
}

// Shows reports how many times the popover was shown.
func (p *Popover) Shows() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shows
}

// Size returns the declared size.
func (p *Popover) Size() (int, int) {
	return p.options.Width, p.options.Height
}

// SetPosition records where the popover was placed.
// Focus records that the popover took the keyboard.
func (p *Popover) Focus() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.focused = true
}

// IsFocused reports whether the popover has the keyboard.
func (p *Popover) IsFocused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.focused
}

func (p *Popover) SetPosition(x, y int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.x, p.y = x, y
}
