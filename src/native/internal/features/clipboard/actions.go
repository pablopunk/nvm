package clipboard

import (
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Suppressor is the part of the watcher the actions use to ignore their own writes.
type Suppressor interface {
	Suppress(id string)
}

// Actions copies and pastes history entries through the Clipboard and Paster ports. Before every write it
// tells the Suppressor the entry ID, so the watcher does not record the app's own copy as a new entry.
type Actions struct {
	clip     platform.Clipboard
	rich     platform.RichClipboard
	paster   platform.Paster
	images   *ImageStore
	history  *History
	suppress Suppressor
}

// NewActions returns the copy and paste actions over the given ports.
func NewActions(clip platform.Clipboard, paster platform.Paster, images *ImageStore, history *History, suppress Suppressor) *Actions {
	a := &Actions{clip: clip, paster: paster, images: images, history: history, suppress: suppress}
	a.rich, _ = clip.(platform.RichClipboard)
	return a
}

// Copy puts an entry on the clipboard and marks it used. A file list needs a rich clipboard and returns
// platform.ErrUnsupported without one.
func (a *Actions) Copy(e Entry) error {
	if err := a.write(e); err != nil {
		return err
	}
	a.markUsed(e.ID)
	return nil
}

// Paste puts an entry on the clipboard and sends the paste gesture to the frontmost app. expectedAppID names
// the app that must still have focus; see platform.Paster for the refusal rules.
func (a *Actions) Paste(e Entry, expectedAppID string) error {
	if err := a.write(e); err != nil {
		return err
	}
	if err := a.paster.PasteIntoFrontmost(expectedAppID); err != nil {
		return err
	}
	a.markUsed(e.ID)
	return nil
}

// markUsed records the use. A read-only history cannot record it, which is not a reason to fail a copy the
// user already has, so the error is dropped.
func (a *Actions) markUsed(id string) {
	_, _ = a.history.MarkUsed(id)
}

// write puts the entry's content on the clipboard, after telling the watcher to ignore it. Rich entries write
// their markup when the clipboard supports it and their plain text otherwise; RTF entries write plain text
// until a formatted RTF write exists.
func (a *Actions) write(e Entry) error {
	a.suppress.Suppress(e.ID)
	switch e.Kind {
	case KindText, KindRTF:
		return a.clip.WriteText(e.Text)
	case KindHTML:
		if a.rich != nil {
			return a.rich.WriteRich(e.Text, e.Original)
		}
		return a.clip.WriteText(e.Text)
	case KindImage:
		png, err := a.images.Read(e.ImageRef)
		if err != nil {
			return err
		}
		return a.clip.WriteImagePNG(png)
	case KindFiles:
		if a.rich == nil {
			return platform.ErrUnsupported
		}
		return a.rich.WriteFiles(e.Paths)
	}
	return ErrInvalidEntry
}
