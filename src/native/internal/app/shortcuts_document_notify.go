package app

import (
	"reflect"

	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// notifyingDocument tells onChange after every save that changed the shortcut document. Every way a shortcut is set,
// changed or removed (the recorder, the list, an extension, the palette shortcut) saves through the document, so this
// is the one place that announces a change.
type notifyingDocument struct {
	shortcuts.Persistence
	onChange func()
}

// Update saves through the wrapped document and announces a changed result.
func (d notifyingDocument) Update(change func(*shortcuts.Document)) error {
	before, _ := d.Load()
	if err := d.Persistence.Update(change); err != nil {
		return err
	}
	if after, _ := d.Load(); !reflect.DeepEqual(before, after) {
		d.onChange()
	}
	return nil
}
