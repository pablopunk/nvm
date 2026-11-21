package services

import (
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxClipboardText bounds one clipboard write from an extension.
const maxClipboardText = 1 << 20

func (h *Handler) serveDesktopClipboard(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopClipboard); err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpReadText:
		return h.readClipboardText()
	case protocol.OpWriteText:
		return nil, h.writeClipboardText(call.Args)
	case protocol.OpReadImage:
		return h.readClipboardImage()
	case protocol.OpWriteImage:
		return nil, h.writeClipboardImage(call.Args)
	}
	return h.serveClipboardFormats(call)
}

func (h *Handler) readClipboardText() (string, error) {
	if h.deps.Clipboard == nil {
		return "", unavailable("the clipboard is not available in this build")
	}
	text, err := h.deps.Clipboard.ReadText()
	if err != nil {
		return "", fmt.Errorf("read the clipboard: %w", err)
	}
	return text, nil
}

func (h *Handler) writeClipboardText(raw json.RawMessage) error {
	var args protocol.WriteTextArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	return h.putText(args.Text, args.Concealed)
}

// putText writes plain text, refusing a concealed write the clipboard cannot mark.
func (h *Handler) putText(text string, concealed bool) error {
	if len(text) > maxClipboardText {
		return invalid("clipboard text is longer than %d bytes", maxClipboardText)
	}
	if h.deps.Clipboard == nil {
		return unavailable("the clipboard is not available in this build")
	}
	if !h.actions.allow() {
		return rateExceeded("clipboard writes")
	}
	if !concealed {
		return h.deps.Clipboard.WriteText(text)
	}
	return writeConcealed(h.deps.Clipboard, text)
}

// writeConcealed writes text that clipboard history must not record. A clipboard that cannot mark content as concealed
// refuses the write, so the text never lands on the clipboard unmarked.
func writeConcealed(clipboard platform.Clipboard, text string) error {
	concealed, ok := clipboard.(platform.ConcealedClipboard)
	if !ok {
		return unavailable("this system cannot keep clipboard text out of history, so nothing was copied")
	}
	return concealed.WriteConcealedText(text)
}
