package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

const readExtensionToolName = "read_extension"

var readExtensionSchema = json.RawMessage(`{"type":"object","properties":{` +
	`"id":{"type":"string","description":"The id of an installed extension, from the installed extensions list."},` +
	`"offset":{"type":"integer","description":"The byte offset of the page to read. Omit it for the first page."}` +
	`},"required":["id"],"additionalProperties":false}`)

type readExtensionArgs struct {
	ID     string `json:"id"`
	Offset int    `json:"offset"`
}

// readExtensionTool shows the manifest and source of any installed extension, a page at a time. Reading is the only
// thing it does: write_extension still writes only the chat's own extension.
func (w *Workshop) readExtensionTool() agent.Tool {
	return agent.Tool{
		Name:        readExtensionToolName,
		Description: "Read the manifest and source of an installed extension, a page at a time. It is read-only: only write_extension changes an extension, and only this chat's own.",
		Label:       "Reading an extension",
		Schema:      readExtensionSchema,
		Summarize:   summarizeReadExtension,
		Run: func(_ context.Context, args json.RawMessage) (agent.Result, error) {
			var in readExtensionArgs
			if err := json.Unmarshal(args, &in); err != nil {
				return agent.Result{}, err
			}
			return w.readExtension(in)
		},
	}
}

func (w *Workshop) readExtension(in readExtensionArgs) (agent.Result, error) {
	manifest, source, err := w.extensions.Definition(in.ID)
	if err != nil {
		return agent.Result{}, fmt.Errorf("no installed extension has the id %q", in.ID)
	}
	if source == "" {
		return agent.Result{}, errors.New("this extension has no source to read: it is built into Nevermind")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return agent.Result{}, err
	}
	return extensionPage(in.ID, string(data)+"\n\n"+source, in.Offset)
}

// extensionPage returns the page of an extension's text that starts at offset, and says where the next one starts.
func extensionPage(id, text string, offset int) (agent.Result, error) {
	if offset < 0 || offset > len(text) {
		return agent.Result{}, fmt.Errorf("offset must be between 0 and %d", len(text))
	}
	end := pageEnd(text, offset)
	header := fmt.Sprintf("Extension %s (read-only), bytes %d to %d of %d.", id, offset, end, len(text))
	if end < len(text) {
		header += fmt.Sprintf(" Read the next page with offset %d if you need it.", end)
	}
	return agent.Result{Text: header + "\n\n" + text[offset:end], Outcome: "Read " + id}, nil
}

func summarizeReadExtension(args json.RawMessage) string {
	var in readExtensionArgs
	if json.Unmarshal(args, &in) != nil {
		return ""
	}
	return in.ID
}
