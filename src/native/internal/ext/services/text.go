package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/calc"
)

// defaultCursorToken is a run of private-use characters that no snippet contains, so the cursor can be found after
// the variables are filled in.
const defaultCursorToken = "NEVERMIND_CURSOR"

func (h *Handler) serveText(call protocol.ServiceCall) (any, error) {
	if call.Op != protocol.OpTemplate {
		return nil, invalid("text has no operation %q", call.Op)
	}
	var args protocol.TemplateArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return nil, err
	}
	return h.expand(args), nil
}

// expand fills the template with the standard variables, the clipboard when the extension may read it, and the
// variables the extension passed, which win over the standard ones.
func (h *Handler) expand(args protocol.TemplateArgs) api.TemplateResult {
	cursorToken := args.Options.CursorToken
	if cursorToken == "" {
		cursorToken = defaultCursorToken
	}
	values := h.standardVariables(args.Input, args.Options, cursorToken)
	for name, value := range args.Options.Variables {
		values[name] = value
	}
	text, cursor, missing := expandTemplate(args.Input, values, cursorToken, h.calculate)
	return api.TemplateResult{Text: text, Cursor: cursor, Missing: missing}
}

func (h *Handler) standardVariables(input string, options api.TemplateOptions, cursorToken string) map[string]string {
	now := h.deps.Clock.Now().Local()
	return map[string]string{
		"date":         now.Format("2006-01-02"),
		"time":         now.Format("15:04"),
		"datetime":     now.Format("2006-01-02 15:04"),
		"uuid":         newUUID(),
		"clipboard":    h.clipboardFor(options),
		"selectedText": h.selectedTextFor(input, options),
		"cursor":       cursorToken,
	}
}

// clipboardFor reads the clipboard only when the extension declared clipboard.history and did not opt out.
func (h *Handler) clipboardFor(options api.TemplateOptions) string {
	if options.IncludeClipboard != nil && !*options.IncludeClipboard {
		return ""
	}
	if !h.capabilities[api.CapabilityClipboardHistory] || h.deps.Clipboard == nil {
		return ""
	}
	text, err := h.deps.Clipboard.ReadText()
	if err != nil {
		return ""
	}
	return text
}

// selectedTextFor captures the selection only for a template that asks for it, because capturing it takes focus and
// the clipboard. It needs the desktop.selection capability, and an extension can opt out.
func (h *Handler) selectedTextFor(input string, options api.TemplateOptions) string {
	if !strings.Contains(input, "selectedText") || (options.IncludeSelectedText != nil && !*options.IncludeSelectedText) {
		return ""
	}
	if !h.capabilities[api.CapabilityDesktopSelection] || h.deps.Selection == nil {
		return ""
	}
	text, found, err := h.deps.Selection.Text()
	if err != nil || !found {
		return ""
	}
	return text
}

// calculate evaluates one calculator expression offline. Currency answers need rates and are not offered here.
func (h *Handler) calculate(expression string) (string, bool) {
	result, ok := calc.New(h.deps.Clock, time.Local, nil).Evaluate(expression)
	if !ok {
		return "", false
	}
	if result.Copy != "" {
		return result.Copy, true
	}
	return result.Display, true
}

// newUUID returns a random version 4 UUID.
func newUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32])
}
