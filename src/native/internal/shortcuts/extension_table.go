package shortcuts

import "github.com/pablopunk/nvm/native/internal/ext/api"

// ExtensionTable is the shortcut table as extensions read it. It reads the stored document and the commands only, so
// it never reads the state the UI thread writes; the extension goroutine can call it at any time.
type ExtensionTable struct {
	manager *Manager
}

// NewExtensionTable returns the extension view of m.
func NewExtensionTable(m *Manager) ExtensionTable {
	return ExtensionTable{manager: m}
}

// List returns every shortcut the user has bound to a command or to the palette.
func (t ExtensionTable) List() []api.ShortcutRecord {
	var records []api.ShortcutRecord
	for _, entry := range Entries(t.manager.load(), t.manager.setup.Commands, nil, nil) {
		if entry.Accelerator != "" {
			records = append(records, recordOf(entry))
		}
	}
	return records
}

// Palette returns the shortcut that opens Nevermind.
func (t ExtensionTable) Palette() api.ShortcutRecord {
	return api.ShortcutRecord{ActionID: PaletteID, Title: PaletteTitle, Accelerator: t.manager.load().Palette, Scope: "global", Source: extensionSourceName(SourceUser)}
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
