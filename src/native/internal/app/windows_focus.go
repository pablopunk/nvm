package app

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// focusCaptureTimeout bounds the window-system reads of the macOS path, the only capture that is not the palette's own.
const focusCaptureTimeout = 150 * time.Millisecond

// summonRecord is the app a palette window command acts on and how the keyboard goes back to it. focus is the palette's
// capture, restored with the Focus port (X11); where there is no focus port (macOS) the record holds only the app.
type summonRecord struct {
	target windowmgmt.Target
	focus  platform.FocusTarget
	known  bool
}

// windowSummon holds the record. The UI thread writes it when a command starts or the palette shows, and the engine's
// goroutine reads it, so it is locked.
type windowSummon struct {
	mu     sync.Mutex
	record summonRecord
}

func (s *windowSummon) store(record summonRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record = record
}

func (s *windowSummon) load() summonRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.record
}

// onPaletteShown captures the app in front where the palette captures nothing of its own, then shows the palette's rows.
func (p *App) onPaletteShown() {
	p.captureWindowFocus()
	p.showApps()
}

// captureWindowFocus records the frontmost app before the palette appears. It runs only where the palette has no focus
// port: there the palette's summon capture is missing, and the window commands need one.
func (p *App) captureWindowFocus() {
	if p.focusOption() != nil || !p.windowManagementEnabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), focusCaptureTimeout)
	defer cancel()
	app, found, err := p.ports.Windows.Frontmost(ctx)
	if err != nil || !found {
		p.windowFocus.store(summonRecord{})
		return
	}
	p.windowFocus.store(summonRecord{target: windowmgmt.Target{PID: app.PID, BundleID: app.BundleID}, known: true})
}

// windowTarget returns the window a palette window command acts on: the window that had the keyboard when the palette
// was summoned. Where the palette has a focus port, that capture is the only one. It runs on the UI thread, before the
// palette hides.
func (p *App) windowTarget() (windowmgmt.Target, bool) {
	if p.focusOption() == nil {
		record := p.windowFocus.load()
		return record.target, record.known
	}
	focus, known := p.Summoned()
	if !known {
		p.windowFocus.store(summonRecord{})
		return windowmgmt.Target{}, false
	}
	record := summonRecord{target: windowmgmt.Target{PID: focus.PID}, focus: focus, known: true}
	p.windowFocus.store(record)
	return record.target, true
}

// PaletteFocusTarget returns the app the palette window command was started from. It is the windowmgmt FocusHost port.
func (p *App) PaletteFocusTarget() (windowmgmt.Target, bool) {
	record := p.windowFocus.load()
	return record.target, record.known
}

// HidePaletteAndRestoreFocus hides the palette on the UI thread, then gives the keyboard back where the palette captured
// it. Without a focus port (macOS) the palette is a nonactivating panel that never takes the app's front position, so
// nothing needs giving back: the hand-off is requested, and the engine checks that the target is in front.
func (p *App) HidePaletteAndRestoreFocus(ctx context.Context, _ windowmgmt.Target) bool {
	record := p.windowFocus.load()
	p.hidePaletteOnUI(ctx)
	if focus := p.focusOption(); focus != nil {
		return record.focus.Window != "" && focus.Restore(record.focus) == nil
	}
	return true
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
