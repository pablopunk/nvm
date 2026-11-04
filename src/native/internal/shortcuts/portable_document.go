package shortcuts

import (
	"maps"
	"slices"

	"github.com/pablopunk/nvm/native/internal/portable"
)

// portableDocument keeps the shortcut document in config.json: the palette in settings.paletteHotkey, and the actions in
// shortcuts under their 1.x keys. Keys this build does not bind are left as they are.
type portableDocument struct {
	store *portable.Store
}

// NewPortableDocument returns the shortcut persistence over config.json.
func NewPortableDocument(store *portable.Store) Persistence {
	return portableDocument{store: store}
}

// Load returns the shortcut document in config.json. An unreadable file returns an empty document with the reason.
func (d portableDocument) Load() (Document, error) {
	stored, err := d.store.Load()
	return documentOf(stored), err
}

// Update applies change to the document in config.json and writes it.
func (d portableDocument) Update(change func(*Document)) error {
	return d.store.Update(func(stored portable.Document) {
		before := documentOf(stored)
		doc := before
		change(&doc)
		writeDocument(stored, doc, paletteChanged(before.Palette, doc.Palette))
	})
}

// paletteChanged reports whether a save changes the palette shortcut in the file. A blank palette in the file means
// the default to 2.0, and a save that only fills that default in must not write it: 1.x reads the key as its own
// palette shortcut.
func paletteChanged(before, after string) bool {
	if before == "" && after == DefaultPalette {
		return false
	}
	return before != after
}

func documentOf(stored portable.Document) Document {
	doc := DefaultDocument("")
	doc.Palette = stored.Settings().PaletteHotkey
	for _, key := range stored.Keys("shortcuts") {
		actionID, bound := ActionIDOf(key)
		if !bound {
			continue
		}
		value, _ := stored.Value("shortcuts", key)
		if accelerator, isText := value.(string); isText {
			doc.Actions[actionID] = accelerator
		} else {
			doc.Disabled[actionID] = true
		}
	}
	return doc
}

// writeDocument makes the keys this build binds match doc. Keys that already exist keep their place in the file.
func writeDocument(stored portable.Document, doc Document, writePalette bool) {
	switch {
	case !writePalette:
	case doc.Palette == "":
		stored.Remove("settings", "paletteHotkey")
	default:
		stored.SetValue(doc.Palette, "settings", "paletteHotkey")
	}
	for _, key := range stored.Keys("shortcuts") {
		if actionID, bound := ActionIDOf(key); bound && !isWritten(doc, actionID) {
			stored.Remove("shortcuts", key)
		}
	}
	for _, actionID := range slices.Sorted(maps.Keys(doc.Actions)) {
		stored.SetValue(doc.Actions[actionID], "shortcuts", PortableKeyOf(actionID))
	}
	for _, actionID := range slices.Sorted(maps.Keys(doc.Disabled)) {
		if doc.Disabled[actionID] {
			stored.SetValue(nil, "shortcuts", PortableKeyOf(actionID))
		}
	}
}

func isWritten(doc Document, actionID string) bool {
	_, chosen := doc.Actions[actionID]
	return chosen || doc.Disabled[actionID]
}
