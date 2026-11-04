package shortcuts

import "github.com/pablopunk/nvm/native/internal/ext/api"

// ExtensionTable is the shortcut table as extensions read it. It reads the stored document and the commands only, so
// it never reads the state the UI thread writes; the extension goroutine can call it at any time.
type ExtensionTable struct {
	manager *Manager
	labeler func() Labeler
}

// NewExtensionTable returns the extension view of m. labeler spells the accelerators the table reports, as the user
// reads them now; it is asked on each call, because the Hyper key can change.
func NewExtensionTable(m *Manager, labeler func() Labeler) ExtensionTable {
	return ExtensionTable{manager: m, labeler: labeler}
}

// List returns every shortcut the user has bound to a command or to the palette.
func (t ExtensionTable) List() []api.ShortcutRecord {
	var records []api.ShortcutRecord
	for _, entry := range Entries(t.manager.load(), t.manager.setup.Commands, nil, nil) {
		if entry.Accelerator != "" {
			records = append(records, t.labeled(recordOf(entry)))
		}
	}
	return records
}

// Palette returns the shortcut that opens Nevermind.
func (t ExtensionTable) Palette() api.ShortcutRecord {
	return t.labeled(api.ShortcutRecord{ActionID: PaletteID, Title: PaletteTitle, Accelerator: t.manager.load().Palette, Scope: "global", Source: extensionSourceName(SourceUser)})
}

// labeled adds the record's label as this platform spells its accelerator.
func (t ExtensionTable) labeled(record api.ShortcutRecord) api.ShortcutRecord {
	record.Label = t.labeler().Label(record.Accelerator)
	return record
}

func recordOf(entry Entry) api.ShortcutRecord {
	return api.ShortcutRecord{ActionID: entry.ActionID, Title: entry.Title, Accelerator: entry.Accelerator, Scope: "global", Source: extensionSourceName(entry.Source)}
}

func extensionSourceName(source Source) string {
	if source == SourceDeclared {
		return "extension"
	}
	return "user"
}
