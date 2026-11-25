package shortcuts

import (
	"context"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// errEditorTimedOut is the failure of a change the UI thread did not take before the caller gave up. The change may
// still be applied.
var errEditorTimedOut = errors.New("the shortcut change did not finish in time; it may still apply")

// ExtensionEditor changes the user's global shortcuts for extensions. The manager binds hotkeys and keeps state the UI
// thread owns, so every change runs there through onUI while the caller waits.
type ExtensionEditor struct {
	table       ExtensionTable
	onUI        func(func())
	afterChange func()
}

// NewExtensionEditor returns the editor over table's manager. onUI runs a function on the UI thread; afterChange, when
// set, runs there after a change was saved, so the app can redraw what shows a shortcut.
func NewExtensionEditor(table ExtensionTable, onUI func(func()), afterChange func()) ExtensionEditor {
	return ExtensionEditor{table: table, onUI: onUI, afterChange: afterChange}
}

// Bindable lists the palette shortcut and every command that can take a shortcut, bound or not.
func (e ExtensionEditor) Bindable() []api.ShortcutRecord {
	records := []api.ShortcutRecord{e.table.Palette()}
	for _, entry := range e.table.manager.Entries() {
		records = append(records, e.table.labeled(recordOf(entry)))
	}
	return records
}

// Set binds accelerator as actionID's shortcut, or moves the palette shortcut, and returns the record as bound.
func (e ExtensionEditor) Set(ctx context.Context, actionID, accelerator string) (api.ShortcutRecord, error) {
	if !e.isBindable(actionID) {
		return api.ShortcutRecord{}, fmt.Errorf("no command has the action id %q; list the bindable ones first: %w", actionID, api.ErrInvalidInput)
	}
	err := e.onUIWait(ctx, func() error { return e.apply(actionID, Normalize(accelerator)) })
	if err != nil {
		return api.ShortcutRecord{}, err
	}
	return e.recordOf(actionID), nil
}

// Remove drops the user's shortcut of actionID. The palette shortcut is refused: without it Nevermind cannot open.
func (e ExtensionEditor) Remove(ctx context.Context, actionID string) error {
	if actionID == PaletteID {
		return fmt.Errorf("the shortcut that opens Nevermind cannot be removed, only changed: %w", api.ErrInvalidInput)
	}
	if !e.isBindable(actionID) {
		return fmt.Errorf("no command has the action id %q: %w", actionID, api.ErrInvalidInput)
	}
	return e.onUIWait(ctx, func() error { return refusal(e.table.manager.RemoveAction(actionID)) })
}

// apply saves one change on the UI thread.
func (e ExtensionEditor) apply(actionID, accelerator string) error {
	if actionID == PaletteID {
		_, err := e.table.manager.SetPalette(accelerator)
		return refusal(err)
	}
	return refusal(e.table.manager.SetAction(actionID, accelerator))
}

// onUIWait runs change on the UI thread, then afterChange when it succeeded, and waits for both or for ctx.
func (e ExtensionEditor) onUIWait(ctx context.Context, change func() error) error {
	done := make(chan error, 1)
	e.onUI(func() {
		err := change()
		if err == nil && e.afterChange != nil {
			e.afterChange()
		}
		done <- err
	})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return errEditorTimedOut
	}
}

func (e ExtensionEditor) isBindable(actionID string) bool {
	for _, record := range e.Bindable() {
		if record.ActionID == actionID {
			return true
		}
	}
	return false
}

func (e ExtensionEditor) recordOf(actionID string) api.ShortcutRecord {
	for _, record := range e.Bindable() {
		if record.ActionID == actionID {
			return record
		}
	}
	return api.ShortcutRecord{ActionID: actionID}
}

// refusal marks the manager's refusal of a change as invalid input, so the caller reads its reason and not an internal
// failure.
func refusal(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", err, api.ErrInvalidInput)
}
