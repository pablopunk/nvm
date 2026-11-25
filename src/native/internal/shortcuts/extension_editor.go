package shortcuts

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// errChangeNotStarted is the failure of a change whose caller gave up before the UI thread took it. Nothing was saved.
var errChangeNotStarted = errors.New("the shortcut change was stopped before it started; nothing changed")

// errTableMoved refuses a change prepared on an older table, such as one the user edited while a question waited.
var errTableMoved = fmt.Errorf("the shortcuts changed since this change was prepared; nothing changed: %w", api.ErrInvalidInput)

// ExtensionEditor changes the user's global shortcuts for extensions and conversation programs. Reads come from the
// manager's published snapshot, so they never wait. A change runs on the UI thread through onUI, which owns the
// manager, and is checked again there against the table as it is then.
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
	return e.recordsOf(e.table.manager.Snapshot())
}

func (e ExtensionEditor) recordsOf(snapshot Snapshot) []api.ShortcutRecord {
	records := make([]api.ShortcutRecord, 0, len(snapshot.Entries)+1)
	records = append(records, e.table.labeled(paletteRecord(snapshot.Palette)))
	for _, entry := range snapshot.Entries {
		records = append(records, e.table.labeled(recordOf(entry)))
	}
	return records
}

// Plan prepares a change from the snapshot: setting accelerator for actionID, or removing its shortcut when accelerator
// is empty. It refuses what the table would refuse, so nobody is asked to approve a change that cannot happen.
func (e ExtensionEditor) Plan(actionID, accelerator string) (protocol.ShortcutChange, error) {
	return e.planOn(e.table.manager.Snapshot(), actionID, Normalize(accelerator))
}

func (e ExtensionEditor) planOn(snapshot Snapshot, actionID, accelerator string) (protocol.ShortcutChange, error) {
	current, found := e.currentOf(snapshot, actionID)
	if !found {
		return protocol.ShortcutChange{}, fmt.Errorf("no command has the action id %q; list the bindable ones first: %w", actionID, api.ErrInvalidInput)
	}
	labeler := e.table.labeler()
	change := protocol.ShortcutChange{ActionID: actionID, Title: current.Title, Accelerator: accelerator, Label: labeler.Label(accelerator),
		Current: labeler.Label(current.Accelerator), Revision: snapshot.Revision}
	if accelerator == "" {
		return change, refusedRemoval(current)
	}
	if actionID != PaletteID && accelerator == snapshot.Palette {
		return change, fmt.Errorf("%s is reserved for opening Nevermind: %w", accelerator, api.ErrInvalidInput)
	}
	for _, owner := range snapshot.Owners(accelerator, actionID) {
		if owner.Source == SourceUser {
			return change, fmt.Errorf("%s is already used by %s: %w", accelerator, owner.Title, api.ErrInvalidInput)
		}
		change.Displaced = append(change.Displaced, owner.Title)
	}
	return change, nil
}

// currentOf is the action's entry, with the palette shortcut as an entry of its own.
func (e ExtensionEditor) currentOf(snapshot Snapshot, actionID string) (Entry, bool) {
	if actionID == PaletteID {
		return Entry{ActionID: PaletteID, Title: PaletteTitle, Accelerator: snapshot.Palette, Source: SourceUser}, true
	}
	return snapshot.Entry(actionID)
}

// refusedRemoval refuses removing the palette shortcut, without which Nevermind cannot open, and a shortcut that is
// not there.
func refusedRemoval(current Entry) error {
	switch {
	case current.ActionID == PaletteID:
		return fmt.Errorf("the shortcut that opens Nevermind cannot be removed, only changed: %w", api.ErrInvalidInput)
	case current.Accelerator == "":
		return fmt.Errorf("%s has no shortcut to remove: %w", current.Title, api.ErrInvalidInput)
	}
	return nil
}

// Set binds accelerator as actionID's shortcut, or moves the palette shortcut, and returns the record as bound. A
// non-empty expect refuses the change when the table moved since that revision.
func (e ExtensionEditor) Set(ctx context.Context, actionID, accelerator, expect string) (api.ShortcutRecord, error) {
	var bound api.ShortcutRecord
	err := e.changeOnUI(ctx, func() error {
		if err := e.recheck(actionID, Normalize(accelerator), expect); err != nil {
			return err
		}
		if err := e.apply(actionID, Normalize(accelerator)); err != nil {
			return err
		}
		bound = e.boundRecord(actionID)
		return nil
	})
	return bound, err
}

// Remove drops the user's shortcut of actionID. The palette shortcut is refused: without it Nevermind cannot open.
func (e ExtensionEditor) Remove(ctx context.Context, actionID, expect string) error {
	return e.changeOnUI(ctx, func() error {
		if err := e.recheck(actionID, "", expect); err != nil {
			return err
		}
		return refusal(e.table.manager.RemoveAction(actionID))
	})
}

// recheck plans the change again on the UI thread, against the table as it is now, and refuses it when the table moved
// since expect.
func (e ExtensionEditor) recheck(actionID, accelerator, expect string) error {
	manager := e.table.manager
	manager.publish()
	if expect != "" && expect != manager.Revision() {
		return errTableMoved
	}
	_, err := e.planOn(manager.Snapshot(), actionID, accelerator)
	return err
}

// apply saves one change on the UI thread. An action whose new key the OS refuses keeps its previous shortcut.
func (e ExtensionEditor) apply(actionID, accelerator string) error {
	if actionID == PaletteID {
		_, err := e.table.manager.SetPalette(accelerator)
		return refusal(err)
	}
	return refusal(e.table.manager.SetActionOrKeepPrevious(actionID, accelerator))
}

func (e ExtensionEditor) boundRecord(actionID string) api.ShortcutRecord {
	for _, record := range e.Bindable() {
		if record.ActionID == actionID {
			return record
		}
	}
	return api.ShortcutRecord{ActionID: actionID}
}

// Phases of a change handed to the UI thread. Exactly one side wins the move out of changeQueued: the UI thread starts
// the change, or the caller abandons it, and an abandoned change never runs.
const (
	changeQueued int32 = iota
	changeStarted
	changeAbandoned
)

// changeOnUI runs change on the UI thread, then afterChange when it succeeded, and waits for it. When ctx ends first,
// a change that has not started is abandoned and never runs; one that started is waited for, so the caller reports
// what really happened.
func (e ExtensionEditor) changeOnUI(ctx context.Context, change func() error) error {
	var phase atomic.Int32
	done := make(chan error, 1)
	e.onUI(func() {
		if !phase.CompareAndSwap(changeQueued, changeStarted) {
			return
		}
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
		if phase.CompareAndSwap(changeQueued, changeAbandoned) {
			return errChangeNotStarted
		}
		return <-done
	}
}

// refusal marks the manager's refusal of a change as invalid input, so the caller reads its reason and not an internal
// failure.
func refusal(err error) error {
	if err == nil || errors.Is(err, api.ErrInvalidInput) {
		return err
	}
	return fmt.Errorf("%w: %w", err, api.ErrInvalidInput)
}
