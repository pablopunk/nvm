package app

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// focusCaptureTimeout bounds the window system calls made while the palette is summoned. A slow capture only
// means a window command has no target; it never delays the palette.
const focusCaptureTimeout = 150 * time.Millisecond

// paletteFocusCapture is the app that had focus when the palette was summoned, and the window that held it. The
// palette hides on blur, so the capture happens before it shows and the window command hands focus back later.
type paletteFocusCapture struct {
	mu     sync.Mutex
	target windowmgmt.Target
	window string
	known  bool
}

func (c *paletteFocusCapture) set(target windowmgmt.Target, window string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.target, c.window, c.known = target, window, true
}

func (c *paletteFocusCapture) forget() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.known = false
}

func (c *paletteFocusCapture) get() (windowmgmt.Target, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.target, c.window, c.known
}

// onPaletteShown captures the app in front, then shows the palette's rows.
func (p *App) onPaletteShown() {
	p.captureWindowFocus()
	p.showApps()
}

// captureWindowFocus records the app and window in front. It runs on the UI thread before the palette appears.
func (p *App) captureWindowFocus() {
	if p.windows == nil || !p.windows.Available() || !p.ports.Capabilities.Supported(platform.CapabilityWindowManagement) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), focusCaptureTimeout)
	defer cancel()
	app, found, err := p.ports.Windows.Frontmost(ctx)
	if err != nil || !found {
		p.focusCapture.forget()
		return
	}
	window := ""
	if record, err := p.ports.Windows.FocusedWindow(ctx, app.PID); err == nil {
		window = record.Key
	}
	p.focusCapture.set(windowmgmt.Target{PID: app.PID, BundleID: app.BundleID}, window)
}

// PaletteFocusTarget returns the app the palette was summoned from, if it was captured. It is the windowmgmt
// FocusHost port.
func (p *App) PaletteFocusTarget() (windowmgmt.Target, bool) {
	target, _, known := p.focusCapture.get()
	return target, known
}

// HidePaletteAndRestoreFocus hides the palette on the UI thread, then raises the window that had focus. It reports
// whether the raise was requested; the engine checks that the target is in front.
func (p *App) HidePaletteAndRestoreFocus(ctx context.Context, target windowmgmt.Target) bool {
	_, window, _ := p.focusCapture.get()
	p.hidePaletteOnUI(ctx)
	if window == "" || p.ports.Windows == nil {
		return false
	}
	return p.ports.Windows.Raise(ctx, target.PID, window) == nil
}

// hidePaletteOnUI runs the palette's hide on the UI thread and waits for it, or until ctx ends.
func (p *App) hidePaletteOnUI(ctx context.Context) {
	if p.panel == nil {
		p.Hide()
		return
	}
	done := make(chan struct{})
	p.panel.Update(func() {
		p.Hide()
		close(done)
	})
	select {
	case <-done:
	case <-ctx.Done():
	}
}
