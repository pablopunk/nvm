package clipboard

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ContentKind names what the clipboard holds right now, in the priority order of the 1.x code: files, image,
// HTML, text.
type ContentKind string

// Content kinds reported by ReadContent.
const (
	ContentFiles ContentKind = "files"
	ContentImage ContentKind = "image"
	ContentHTML  ContentKind = "html"
	ContentText  ContentKind = "text"
	ContentEmpty ContentKind = "empty"
)

// Content is a read of the clipboard. Only the fields of its Kind are set.
type Content struct {
	Kind  ContentKind
	Text  string
	HTML  string
	Files []string
	PNG   []byte
}

// ClipboardState is a copy of the clipboard taken before the app writes to it, so the previous content can
// be put back. The clipboard holds one format kind at a time, so a restore writes one of them.
type ClipboardState struct {
	Text  string
	HTML  string
	PNG   []byte
	Files []string
}

// ReadContent reads what the clipboard holds now.
func (a *Actions) ReadContent() (Content, error) {
	if a.rich != nil {
		if files, err := a.rich.ReadFilePaths(); err == nil && len(files) > 0 {
			return Content{Kind: ContentFiles, Files: files}, nil
		}
	}
	if png, err := a.clip.ReadImagePNG(); err == nil && len(png) > 0 {
		return Content{Kind: ContentImage, PNG: png}, nil
	}
	text, err := a.clip.ReadText()
	if err != nil {
		return Content{}, err
	}
	if a.rich != nil {
		if markup, err := a.rich.ReadHTML(); err == nil && markup != "" {
			return Content{Kind: ContentHTML, HTML: markup, Text: text}, nil
		}
	}
	if text != "" {
		return Content{Kind: ContentText, Text: text}, nil
	}
	return Content{Kind: ContentEmpty}, nil
}

// SaveState copies the clipboard so RestoreState can put it back.
func (a *Actions) SaveState() (ClipboardState, error) {
	text, err := a.clip.ReadText()
	if err != nil {
		return ClipboardState{}, err
	}
	state := ClipboardState{Text: text}
	if a.rich != nil {
		if state.HTML, err = a.rich.ReadHTML(); err != nil {
			return ClipboardState{}, err
		}
		if state.Files, err = a.rich.ReadFilePaths(); err != nil {
			return ClipboardState{}, err
		}
	}
	png, err := a.clip.ReadImagePNG()
	switch {
	case err == nil:
		state.PNG = png
	case !errors.Is(err, platform.ErrNotFound):
		return ClipboardState{}, err
	}
	return state, nil
}

// RestoreState puts a saved clipboard back. A saved file list needs a rich clipboard; without one the state is
// refused before anything changes. The restored text is suppressed, as a copy of the app is.
func (a *Actions) RestoreState(state ClipboardState) error {
	if len(state.Files) > 0 && a.rich == nil {
		return platform.ErrUnsupported
	}
	if state.Text != "" {
		a.suppress.Suppress(TextID(state.Text))
	}
	switch {
	case len(state.Files) > 0:
		return a.rich.WriteFiles(state.Files)
	case len(state.PNG) > 0:
		return a.clip.WriteImagePNG(state.PNG)
	case state.HTML != "" && a.rich != nil:
		return a.rich.WriteRich(state.Text, state.HTML)
	case state.Text != "":
		return a.clip.WriteText(state.Text)
	}
	return a.clip.Clear()
}
