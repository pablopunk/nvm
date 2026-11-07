package services

import (
	"bytes"
	"errors"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/selection"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// PasteEngine puts content on the clipboard and sends the paste gesture to the app in front, and types text there. The
// host's selection engine satisfies it; a build without one answers unavailable.
type PasteEngine interface {
	Paste(content selection.Content, options selection.PasteOptions) error
	TypeText(text string, delay time.Duration) error
}

// Limits of one typed text, and the restore delays a paste may ask for: 50 ms to 5 s, 250 ms by default, as in 1.x.
const (
	maxTypedText     = 1 << 16
	maxTypeDelayMs   = 1000
	defaultRestoreMs = 250
	minRestoreMs     = 50
	maxRestoreMs     = 5000
)

// pngSignature opens every PNG file; an image that lacks it is refused before it reaches the clipboard.
var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// PerformPaste runs a declarative paste or typing action the palette sends. Pasting needs the desktop.clipboard and
// desktop.keyboard capabilities, and typing needs desktop.keyboard. A refusal or failure is logged and returned with
// the palette-ready message the HUD shows.
func (h *Handler) PerformPaste(action api.Action) error {
	err := h.performPaste(action)
	if err != nil {
		h.logger.Warn("the paste action failed", logx.F{"action": string(action.Kind), "reason": err.Error()})
	}
	return err
}

func (h *Handler) servePaste(call protocol.ServiceCall) (any, error) {
	if call.Op != protocol.OpPerform {
		return nil, invalid("desktop.paste has no operation %q", call.Op)
	}
	var action api.Action
	if err := decodeArgs(call.Args, &action); err != nil {
		return nil, err
	}
	return nil, h.PerformPaste(action)
}

func (h *Handler) performPaste(action api.Action) error {
	switch action.Kind {
	case api.ActionPaste, api.ActionPasteText:
		if err := h.requireAll(api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard); err != nil {
			return err
		}
		content, options, err := h.pasteRequest(action)
		if err != nil {
			return err
		}
		return h.paste(content, options)
	case api.ActionTypeText:
		if err := h.require(api.CapabilityDesktopKeyboard); err != nil {
			return err
		}
		return h.typeText(action.Text, typingDelayMs(action.Typing))
	}
	return invalid("paste has no action %q", action.Kind)
}

// pasteRequest turns a paste action into the content it writes and the options it pastes with.
func (h *Handler) pasteRequest(action api.Action) (selection.Content, selection.PasteOptions, error) {
	content, err := h.pasteContent(action)
	if err != nil {
		return selection.Content{}, selection.PasteOptions{}, err
	}
	return content, pasteOptionsOf(action), nil
}

// pasteContent reads the content of a paste action. Plain text is the default for text; rich text needs
// PlainTextOnly set false, as 1.x required.
func (h *Handler) pasteContent(action api.Action) (selection.Content, error) {
	if action.Kind == api.ActionPasteText {
		return selection.Content{Kind: selection.KindText, Text: action.Text}, nil
	}
	if action.Content == nil {
		return selection.Content{}, invalid("paste needs content")
	}
	content := action.Content
	switch content.Type {
	case "text":
		text := selection.Content{Kind: selection.KindText, Text: content.Text}
		if content.HTML != "" && !plainTextOnly(action.Paste) {
			text.HTML = content.HTML
		}
		return text, nil
	case "html":
		if content.HTML == "" {
			return selection.Content{}, invalid("html content needs its markup")
		}
		return selection.Content{Kind: selection.KindHTML, Text: content.Text, HTML: content.HTML}, nil
	case "image":
		png, err := h.pasteImage(content)
		if err != nil {
			return selection.Content{}, err
		}
		return selection.Content{Kind: selection.KindImage, PNG: png}, nil
	case "files":
		paths, err := absolutePaths(content.Paths)
		if err != nil {
			return selection.Content{}, err
		}
		return selection.Content{Kind: selection.KindFiles, Files: paths}, nil
	}
	return selection.Content{}, invalid("paste has no content type %q", content.Type)
}

// pasteOptionsOf maps the paste options of an action to the selection engine's. Concealed holds when either the
// options or the content ask for it, and restoring the clipboard implies it.
func pasteOptionsOf(action api.Action) selection.PasteOptions {
	var options api.PasteOptions
	if action.Paste != nil {
		options = *action.Paste
	}
	concealed := options.Concealed || (action.Content != nil && action.Content.Concealed)
	return selection.PasteOptions{
		RestoreClipboard: options.RestoreClipboard,
		Concealed:        concealed,
		RestoreDelay:     restoreDelay(options.RestoreDelayMs),
		ExpectedAppID:    options.ExpectedFrontmostID,
	}
}

func plainTextOnly(options *api.PasteOptions) bool {
	return options == nil || options.PlainTextOnly == nil || *options.PlainTextOnly
}

// restoreDelay is the wait before a restored clipboard comes back, clamped to the 1.x range.
func restoreDelay(ms int64) time.Duration {
	if ms == 0 {
		ms = defaultRestoreMs
	}
	return time.Duration(min(max(ms, minRestoreMs), maxRestoreMs)) * time.Millisecond
}

// typingDelayMs is the pause between typed characters an action asks for; typeText clamps it.
func typingDelayMs(options *api.TypeOptions) int64 {
	if options == nil {
		return 0
	}
	return options.DelayMs
}

// pasteImage reads the PNG of an image paste from its data URL or from an absolute file path.
func (h *Handler) pasteImage(content *api.ClipboardContent) ([]byte, error) {
	if content.ImageDataURL != "" {
		return decodePNGDataURL(content.ImageDataURL)
	}
	if !filepath.IsAbs(content.ImagePath) {
		return nil, invalid("an image path must be absolute")
	}
	if h.deps.Files == nil {
		return nil, unavailable("files are not available in this build")
	}
	info, err := h.deps.Files.Stat(content.ImagePath)
	if err != nil {
		return nil, invalid("the image cannot be read")
	}
	if info.Size() > maxClipboardImageBytes {
		return nil, invalid("the image is larger than %d MB", maxClipboardImageBytes>>20)
	}
	data, err := h.deps.Files.ReadFile(content.ImagePath)
	if err != nil {
		return nil, invalid("the image cannot be read")
	}
	return checkedPNG(data)
}

// checkedPNG refuses an image file that is not a PNG.
func checkedPNG(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, invalid("the image is not a PNG")
	}
	return data, nil
}

// absolutePaths checks the files of a paste: at least one, each absolute.
func absolutePaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, invalid("files content needs paths")
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return nil, invalid("paste file paths must be absolute")
		}
		out = append(out, filepath.Clean(path))
	}
	return out, nil
}

// paste sends content to the app in front, under the desktop action rate limit.
func (h *Handler) paste(content selection.Content, options selection.PasteOptions) error {
	if !h.actions.allow() {
		return rateExceeded("desktop actions")
	}
	if h.deps.Paste == nil {
		return unavailable("paste is not available in this build")
	}
	return pasteFailure(h.deps.Paste.Paste(content, options))
}

// typeText types text into the app in front, under the desktop action rate limit.
func (h *Handler) typeText(text string, delayMs int64) error {
	if len(text) > maxTypedText {
		return invalid("the text is longer than %d bytes", maxTypedText)
	}
	if !h.actions.allow() {
		return rateExceeded("desktop actions")
	}
	if h.deps.Paste == nil {
		return unavailable("typing is not available in this build")
	}
	delay := time.Duration(min(max(delayMs, 0), maxTypeDelayMs)) * time.Millisecond
	return pasteFailure(h.deps.Paste.TypeText(text, delay))
}

// requireAll refuses the call unless every capability is declared.
func (h *Handler) requireAll(capabilities ...api.Capability) error {
	for _, capability := range capabilities {
		if err := h.require(capability); err != nil {
			return err
		}
	}
	return nil
}

// pasteFailure says a paste or typing failure in the words the HUD shows.
func pasteFailure(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, platform.ErrConflict):
		return unavailable("The focused app changed, so nothing was pasted.")
	case errors.Is(err, platform.ErrDenied):
		return unavailable("Allow Nevermind to control the keyboard in System Settings.")
	case errors.Is(err, platform.ErrUnsupported):
		return unavailable("This paste is not available on this system yet.")
	case errors.Is(err, selection.ErrConcealedContent), errors.Is(err, selection.ErrFormattedClipboard),
		errors.Is(err, selection.ErrKeyboardHeld), errors.Is(err, selection.ErrSourceGone), errors.Is(err, selection.ErrSourceChanged):
		return unavailable("%s", err.Error())
	}
	return err
}
