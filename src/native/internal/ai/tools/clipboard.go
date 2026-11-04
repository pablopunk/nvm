package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

// maxClipboardRunes bounds the text a copy call accepts.
const maxClipboardRunes = 100000

// clipboardTools is the clipboard tools over the clipboard port.
type clipboardTools struct {
	deps Deps
}

// read returns the clipboard text. The clipboard may hold a password or a token, so the user must allow each read.
func (c clipboardTools) read() agent.Tool {
	return agent.Tool{
		Name:        "read_clipboard",
		Description: "Read the plain text on the clipboard. The user must allow each read, because the text is sent to the model.",
		Label:       "Reading the clipboard",
		Schema:      json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		Confirm:     true,
		Run:         c.runRead,
	}
}

func (c clipboardTools) runRead(context.Context, json.RawMessage) (agent.Result, error) {
	text, err := c.deps.Clipboard.ReadText()
	if err != nil {
		return agent.Result{}, err
	}
	if text == "" {
		return agent.Result{Text: "The clipboard has no text.", Outcome: "Clipboard has no text"}, nil
	}
	return agent.Result{Text: text, Outcome: fmt.Sprintf("Read %d characters", utf8.RuneCountInString(text))}, nil
}

func (c clipboardTools) copy() agent.Tool {
	return agent.Tool{
		Name:        "copy_text",
		Description: "Put text on the clipboard, replacing what is there.",
		Label:       "Copying text",
		Schema: json.RawMessage(fmt.Sprintf(`{"type":"object","properties":{`+
			`"text":{"type":"string","minLength":1,"maxLength":%d}},`+
			`"required":["text"],"additionalProperties":false}`, maxClipboardRunes)),
		Summarize: func(args json.RawMessage) string {
			var in copyArgs
			_ = json.Unmarshal(args, &in)
			return clip(in.Text, 80)
		},
		Run: c.runCopy,
	}
}

type copyArgs struct {
	Text string `json:"text"`
}

func (c clipboardTools) runCopy(_ context.Context, args json.RawMessage) (agent.Result, error) {
	var in copyArgs
	if err := decodeArgs(args, &in); err != nil {
		return agent.Result{}, err
	}
	if err := c.deps.Clipboard.WriteText(in.Text); err != nil {
		return agent.Result{}, err
	}
	return agent.Result{Text: "Copied the text to the clipboard.", Outcome: fmt.Sprintf("Copied %d characters", utf8.RuneCountInString(in.Text))}, nil
}
