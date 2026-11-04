package selection

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ContentKind names what a paste writes to the clipboard.
type ContentKind string

// The kinds a paste writes.
const (
	KindText  ContentKind = "text"
	KindHTML  ContentKind = "html"
	KindImage ContentKind = "image"
	KindFiles ContentKind = "files"
)

// Content is what a paste puts on the clipboard. KindText writes Text, with HTML beside it when set; KindHTML writes
// HTML with Text as its plain form; KindImage writes PNG; KindFiles writes absolute paths in Files.
type Content struct {
	Kind  ContentKind
	Text  string
	HTML  string
	PNG   []byte
	Files []string
}

// PasteOptions tunes one paste. RestoreClipboard puts back what the clipboard held, implies Concealed, and needs a
// clipboard without formatted content. RestoreDelay zero means DefaultRestoreDelay. ExpectedAppID is the app that
// must still have focus; see platform.Paster.
type PasteOptions struct {
	RestoreClipboard bool
	Concealed        bool
	RestoreDelay     time.Duration
	ExpectedAppID    string
}

// Paste puts content on the clipboard and sends the paste gesture to the source window. With RestoreClipboard the
// clipboard the paste displaced comes back after the restore delay, as Replace does; a failed paste restores it at
// once. Content that clipboard history must not keep (concealed rich text, images and files) is refused with
// ErrConcealedContent where the clipboard cannot mark it.
func (e *Engine) Paste(content Content, options PasteOptions) error {
	source := e.deps.Source()
	if !options.RestoreClipboard {
		return e.pasteContent(source, content, options)
	}
	snapshot, err := e.reader.snapshot()
	if err != nil {
		return err
	}
	if err := e.pasteContent(source, content, options); err != nil {
		e.reader.restore(snapshot)
		return err
	}
	e.restoreAfter(e.restoreDelayFor(options), snapshot, e.stillHolds(content))
	return nil
}

// TypeText types text into the source window as key presses. The clipboard is not touched. It returns
// platform.ErrUnsupported where the paster cannot type.
func (e *Engine) TypeText(text string, delay time.Duration) error {
	if err := restoreSource(e.deps.Clock, e.deps.Focus, e.deps.Source()); err != nil {
		return err
	}
	typist, ok := e.paster().(platform.Typist)
	if !ok {
		return platform.ErrUnsupported
	}
	return typist.TypeIntoFrontmost("", text, delay)
}

// pasteContent writes content, gives the keyboard back to the source and sends the paste gesture.
func (e *Engine) pasteContent(source platform.FocusTarget, content Content, options PasteOptions) error {
	if err := e.writeContent(content, options.Concealed || options.RestoreClipboard); err != nil {
		return err
	}
	if err := restoreSource(e.deps.Clock, e.deps.Focus, source); err != nil {
		return err
	}
	return e.paster().PasteIntoFrontmost(options.ExpectedAppID)
}

// writeContent puts content on the clipboard in the form its kind names.
func (e *Engine) writeContent(content Content, concealed bool) error {
	switch content.Kind {
	case KindText:
		if content.HTML != "" {
			return writeRich(e.deps.Clipboard, content.Text, content.HTML, concealed)
		}
		return writePlainText(e.deps.Clipboard, content.Text, concealed)
	case KindHTML:
		return writeRich(e.deps.Clipboard, content.Text, content.HTML, concealed)
	case KindImage:
		return writePNG(e.deps.Clipboard, content.PNG, concealed)
	case KindFiles:
		return writeFiles(e.deps.Clipboard, content.Files, concealed)
	}
	return platform.ErrUnsupported
}

// writePlainText writes text, marked concealed when clipboard history must not keep it. A clipboard that cannot mark
// text as concealed refuses rather than storing it unmarked.
func writePlainText(clip platform.Clipboard, text string, concealed bool) error {
	if !concealed {
		return clip.WriteText(text)
	}
	marked, ok := clip.(platform.ConcealedClipboard)
	if !ok {
		return ErrConcealedContent
	}
	return marked.WriteConcealedText(text)
}

// writeRich writes text with its HTML markup. Only a clipboard with rich formats can, and never concealed.
func writeRich(clip platform.Clipboard, text, html string, concealed bool) error {
	rich, ok := clip.(platform.RichClipboard)
	if !ok {
		return platform.ErrUnsupported
	}
	if concealed {
		return ErrConcealedContent
	}
	return rich.WriteRich(text, html)
}

// writePNG writes an image. Images cannot be marked concealed, so a concealed image is refused.
func writePNG(clip platform.Clipboard, png []byte, concealed bool) error {
	if concealed {
		return ErrConcealedContent
	}
	return clip.WriteImagePNG(png)
}

// writeFiles writes a list of file paths. Files cannot be marked concealed, so a concealed list is refused.
func writeFiles(clip platform.Clipboard, paths []string, concealed bool) error {
	rich, ok := clip.(platform.RichClipboard)
	if !ok {
		return platform.ErrUnsupported
	}
	if concealed {
		return ErrConcealedContent
	}
	return rich.WriteFiles(paths)
}

// restoreDelayFor is the delay a paste waits before its clipboard comes back.
func (e *Engine) restoreDelayFor(options PasteOptions) time.Duration {
	if options.RestoreDelay > 0 {
		return options.RestoreDelay
	}
	return e.deps.RestoreDelay
}

// stillHolds reports whether the clipboard still holds what the paste wrote. Text is compared; other content cannot be
// read back in this layer, so it always counts as still held.
func (e *Engine) stillHolds(content Content) func() bool {
	if content.Kind != KindText {
		return func() bool { return true }
	}
	return func() bool {
		current, err := e.deps.Clipboard.ReadText()
		return err == nil && current == content.Text
	}
}
