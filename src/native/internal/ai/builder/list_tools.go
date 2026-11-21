package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

const (
	listExtensionsToolName       = "list_extensions"
	readCurrentExtensionToolName = "read_current_extension"
)

// listExtensionsTool lists the installed extensions as the catalog reports them now, a page at a time. It reads the
// catalog only: no file is opened.
func listExtensionsTool(extensions Extensions) agent.Tool {
	return agent.Tool{
		Name:        listExtensionsToolName,
		Description: "List a page of installed extensions (id, title, on/off, row titles). Continue with the returned offset.",
		Label:       "Listing extensions",
		Schema:      readSchema,
		Run: func(ctx context.Context, args json.RawMessage) (agent.Result, error) {
			if err := ctx.Err(); err != nil {
				return agent.Result{}, err
			}
			var in readArgs
			if err := json.Unmarshal(args, &in); err != nil {
				return agent.Result{}, err
			}
			return listExtensionsPage(extensions.Catalog(), in.Offset)
		},
	}
}

func listExtensionsPage(installed []Installed, offset int) (agent.Result, error) {
	if len(installed) == 0 {
		return agent.Result{Text: "No extensions are installed.", Outcome: "Listed extensions"}, nil
	}
	sorted := append([]Installed(nil), installed...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	lines := make([]string, len(sorted))
	for i, entry := range sorted {
		state := "off"
		if entry.Enabled {
			state = "on"
		}
		lines[i] = fmt.Sprintf("%s: %q (%s); rows: %s", entry.ID, entry.Title, state, strings.Join(quoteAll(entry.Titles), ", "))
	}
	body := strings.Join(lines, "\n")
	if offset < 0 || offset > len(body) {
		return agent.Result{}, fmt.Errorf("offset must be between 0 and %d", len(body))
	}
	end := pageEnd(body, offset)
	header := fmt.Sprintf("Installed extensions, bytes %d to %d of %d.", offset, end, len(body))
	if end < len(body) {
		header += fmt.Sprintf(" Continue with offset %d.", end)
	}
	return agent.Result{Text: header + "\n\n" + body[offset:end], Outcome: "Listed extensions"}, nil
}

// readCurrentExtensionTool reads the extension this chat is bound to, through the same snapshot reads as
// read_extension, so completing every page authorizes the write. A chat bound to nothing has no current extension.
func (w *Workshop) readCurrentExtensionTool(chatID string, reads *editReads) agent.Tool {
	return agent.Tool{
		Name:        readCurrentExtensionToolName,
		Description: "Read this chat's own extension page by page, in order, before changing it.",
		Label:       "Reading the current extension",
		Schema:      readSchema,
		Run: func(ctx context.Context, args json.RawMessage) (agent.Result, error) {
			var in readArgs
			if err := json.Unmarshal(args, &in); err != nil {
				return agent.Result{}, err
			}
			current, ok := w.chats.Chat(chatID)
			if !ok {
				return agent.Result{}, errors.New("this chat no longer exists")
			}
			if current.Extension == "" {
				return agent.Result{Text: "This chat has no extension yet. Write one with write_extension.", Outcome: "No current extension"}, nil
			}
			return w.readInstalled(ctx, reads, current.Extension, in.Offset)
		},
	}
}
