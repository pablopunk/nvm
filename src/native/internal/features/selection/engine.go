package selection

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// DefaultRestoreDelay is how long a replacement waits before the clipboard it displaced comes back, so the paste has
// been read by the target app first.
const DefaultRestoreDelay = 500 * time.Millisecond

// Deps are the ports the engine runs on. Source returns the window the palette was summoned from: the keyboard goes
// back there before any gesture, and Undo only applies to it.
type Deps struct {
	Clock         platform.Clock
	Clipboard     platform.Clipboard
	SelectedText  platform.SelectedText
	SelectedFiles platform.SelectedFiles
	Paster        platform.Paster
	Focus         platform.Focus
	Source        func() platform.FocusTarget
	RestoreDelay  time.Duration
}

// Engine reads, replaces and undoes the selection of the source window. It remembers the window the last replacement
// landed in, so an undo cannot reach a different window.
type Engine struct {
	deps   Deps
	reader *Reader
	mu     sync.Mutex
	landed platform.FocusTarget
	undo   bool
}

// New returns an engine over deps.
func New(deps Deps) *Engine {
	if deps.RestoreDelay == 0 {
		deps.RestoreDelay = DefaultRestoreDelay
	}
	return &Engine{deps: deps, reader: NewReader(deps.Clock, deps.Clipboard, deps.SelectedText, deps.Focus)}
}

// Text returns the text selected in the source window and whether there was any. The clipboard is put back as it was.
func (e *Engine) Text() (string, bool, error) {
	text, err := e.reader.Read(e.deps.Source())
	if err != nil {
		return "", false, err
	}
	return text, text != "", nil
}

// Files reads paths from the captured source without changing focus or the clipboard.
func (e *Engine) Files(ctx context.Context) ([]string, error) {
	if e.deps.SelectedFiles == nil {
		return nil, platform.ErrUnsupported
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return e.deps.SelectedFiles.Paths(ctx, e.deps.Source())
}

// Replace puts text in place of the selection of the source window. The text goes on the clipboard concealed, the
// keyboard goes back to the source, the paste gesture is sent, and the clipboard the replace displaced comes back
// after the restore delay, unless the user has copied something since.
func (e *Engine) Replace(text string) (replaced bool, err error) {
	e.reader.mu.Lock()
	defer e.reader.mu.Unlock()
	source := e.deps.Source()
	snapshot, err := e.reader.snapshot()
	if err != nil {
		return false, err
	}
	defer func() {
		if !replaced {
			err = errors.Join(err, e.reader.restore(snapshot))
		}
	}()
	if err := e.reader.writeTemporary(&snapshot, text); err != nil {
		return false, err
	}
	if err := restoreSource(e.deps.Clock, e.deps.Focus, source); err != nil {
		return false, err
	}
	if err := e.paster().PasteIntoFrontmost(""); err != nil {
		return false, err
	}
	e.remember(source)
	e.restoreLater(text, snapshot)
	return true, nil
}

// Undo sends the undo gesture to the source window, which reverts the last replacement in that app. It refuses when
// no replacement is pending, when the source window is not the one the replacement landed in, or when the window is
// gone. A pending replacement is consumed by the attempt, so an undo is sent at most once per replacement.
func (e *Engine) Undo() error {
	landed, pending := e.takePending()
	if !pending {
		return ErrNothingToUndo
	}
	source := e.deps.Source()
	if source.Window != landed.Window {
		return ErrSourceChanged
	}
	if err := restoreSource(e.deps.Clock, e.deps.Focus, source); err != nil {
		return err
	}
	return e.paster().UndoInFrontmost("")
}

// paster is the paster port, or the unsupported answer where the composition root supplies none.
func (e *Engine) paster() platform.Paster {
	if e.deps.Paster == nil {
		return unsupportedPaster{}
	}
	return e.deps.Paster
}

// remember records the window a replacement landed in, so Undo can check it.
func (e *Engine) remember(source platform.FocusTarget) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.landed = source
	e.undo = true
}

// takePending returns the window of the pending replacement and clears it.
func (e *Engine) takePending() (platform.FocusTarget, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	landed, pending := e.landed, e.undo
	e.undo = false
	return landed, pending
}

// restoreLater puts the displaced clipboard back once the target app has read the replacement, unless the clipboard
// no longer holds the replacement text.
func (e *Engine) restoreLater(text string, snapshot clipboardSnapshot) {
	e.restoreAfter(e.deps.RestoreDelay, snapshot, e.stillHolds(Content{Kind: KindText, Text: text}))
}

// restoreAfter puts the snapshot back after delay, when stillHeld reports the clipboard still holds what was written.
func (e *Engine) restoreAfter(delay time.Duration, snapshot clipboardSnapshot, stillHeld func() bool) {
	pending := &snapshot
	e.reader.pending = pending
	safego.Go("selection.restore", func(context.Context) {
		<-e.deps.Clock.After(delay)
		e.reader.mu.Lock()
		defer e.reader.mu.Unlock()
		defer func() {
			if e.reader.pending == pending {
				e.reader.pending = nil
			}
		}()
		if stillHeld() {
			if err := e.reader.restore(snapshot); err != nil {
				logx.Default().Warn("could not restore the clipboard", logx.F{"error": err.Error()})
			}
		}
	})
}

// unsupportedPaster answers every gesture with platform.ErrUnsupported, for a build without a paster port.
type unsupportedPaster struct{}

func (unsupportedPaster) PasteIntoFrontmost(string) error { return platform.ErrUnsupported }

func (unsupportedPaster) UndoInFrontmost(string) error { return platform.ErrUnsupported }

func (unsupportedPaster) FrontmostAppID() (string, error) { return "", platform.ErrUnsupported }
