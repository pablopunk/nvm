package shortcuts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
)

// Snapshot is the shortcut table as any goroutine may read it: the palette shortcut, every action that can take a
// shortcut with the one that applies to it, and the revision of the stored document it was built from. The UI thread
// publishes a new one after every change, so a reader never touches the state the UI thread owns.
type Snapshot struct {
	Palette  string
	Entries  []Entry
	Revision string
}

// Entry returns the snapshot's entry for actionID.
func (s Snapshot) Entry(actionID string) (Entry, bool) {
	for _, entry := range s.Entries {
		if entry.ActionID == actionID {
			return entry, true
		}
	}
	return Entry{}, false
}

// Owners returns the entries other than actionID whose shortcut is accelerator.
func (s Snapshot) Owners(accelerator, actionID string) []Entry {
	var owners []Entry
	for _, entry := range s.Entries {
		if entry.ActionID != actionID && entry.Accelerator != "" && entry.Accelerator == accelerator {
			owners = append(owners, entry)
		}
	}
	return owners
}

// Snapshot returns the latest published table. Before the first publish it holds the stored document only.
func (m *Manager) Snapshot() Snapshot {
	if published := m.snapshot.Load(); published != nil {
		return *published
	}
	doc := m.load()
	return Snapshot{Palette: doc.Palette, Revision: revisionOf(doc)}
}

// publish builds the snapshot readers see. It runs on the UI thread after any change of the document, the declared
// shortcuts or the bindable commands.
func (m *Manager) publish() {
	doc := m.load()
	snapshot := Snapshot{Palette: doc.Palette, Entries: m.bindableEntries(doc), Revision: revisionOf(doc)}
	m.snapshot.Store(&snapshot)
}

// bindableEntries is every action that can take a shortcut: the list's commands and chosen actions, and the commands
// Setup.Bindable adds, such as every extension command, bound or not.
func (m *Manager) bindableEntries(doc Document) []Entry {
	actions := append(slices.Clone(m.setup.Commands), m.resolvedChoices(doc)...)
	if m.setup.Bindable != nil {
		actions = append(actions, m.setup.Bindable()...)
	}
	return Entries(doc, uniqueActions(actions), m.declared(), m.inactive)
}

func uniqueActions(actions []Action) []Action {
	seen := map[string]bool{}
	out := make([]Action, 0, len(actions))
	for _, action := range actions {
		if action.ID != "" && !seen[action.ID] {
			seen[action.ID] = true
			out = append(out, action)
		}
	}
	return out
}

// revisionOf names the state of a document, so a change prepared against one state is refused on another.
func revisionOf(doc Document) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%q|%v|%v", doc.Palette, doc.Actions, doc.Disabled))
	return hex.EncodeToString(sum[:8])
}

// Revision is the revision of the stored document now. It runs on the UI thread, where writes happen.
func (m *Manager) Revision() string {
	return revisionOf(m.load())
}

// SetActionOrKeepPrevious sets an action's shortcut like SetAction, but puts the previous document back when the OS
// refuses to bind the new key, so a failed change never costs the user a shortcut that worked.
func (m *Manager) SetActionOrKeepPrevious(actionID, accelerator string) error {
	previous := m.load()
	if err := m.SetAction(actionID, accelerator); err != nil {
		return err
	}
	reason := m.inactive[actionID]
	if reason == "" || m.suspended {
		return nil
	}
	if err := m.save(previous); err != nil {
		return fmt.Errorf("%s, and the previous shortcut could not be restored: %w", reason, err)
	}
	m.applyActions()
	return errors.New(reason)
}
