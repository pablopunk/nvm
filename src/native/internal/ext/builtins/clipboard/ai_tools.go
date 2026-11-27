package clipboard

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Names of the tools the clipboard built-in gives Nevermind's AI.
const (
	toolSearch = "search-clipboard"
	toolCopy   = "copy-clipboard-item"
	toolDelete = "delete-clipboard-items"
)

// Bounds of what the tools read and return.
const (
	defaultToolResults = 20
	maxToolResults     = 100
	maxToolTextRunes   = 1000
)

// aiTools are the clipboard tools. The history may hold passwords and private text, so the host asks before the AI
// reads it, whatever class a tool declares. A copy puts history on the clipboard, where any app can read it, so it
// asks every time, as a delete does.
func aiTools() []api.AITool {
	return []api.AITool{
		{Name: toolSearch, Class: api.AIToolSensitiveRead,
			Description: "Lists the newest clipboard history items whose text holds query, newest first; an empty query lists the newest.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"additionalProperties":false}`)},
		{Name: toolCopy, Class: api.AIToolExternal,
			Description: "Puts the clipboard history item with id back on the clipboard, with its formats.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}`)},
		{Name: toolDelete, Class: api.AIToolDestructive,
			Description: "Deletes the clipboard history items with ids, at most 200.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"ids":{"type":"array"}},"required":["ids"],"additionalProperties":false}`)},
	}
}

type clipboardToolArgs struct {
	Query string   `json:"query"`
	Limit int      `json:"limit"`
	ID    string   `json:"id"`
	IDs   []string `json:"ids"`
}

// toolEntry is a history item as the search tool returns it: its text, clipped, or its files; pictures by kind only.
type toolEntry struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Text     string   `json:"text,omitempty"`
	Files    []string `json:"files,omitempty"`
	CopiedAt string   `json:"copiedAt"`
}

// CallAITool runs one clipboard tool.
func (Extension) CallAITool(ctx api.Context, call api.AIToolCall) (any, error) {
	args, err := clipboardArgsOf(call)
	if err != nil {
		return nil, err
	}
	switch call.Name {
	case toolSearch:
		entries, err := ctx.Clipboard().List(api.ClipboardQuery{Text: args.Query, Limit: toolResultLimit(args.Limit)})
		return toolEntries(entries), err
	case toolCopy:
		result, err := copyEntry(ctx, args.ID)
		if err == nil && result.Toast != nil {
			err = clipboardRefusal(result.Toast.Message)
		}
		return nil, err
	case toolDelete:
		return ctx.Clipboard().Remove(args.IDs)
	}
	return nil, clipboardRefusal(fmt.Sprintf("clipboard history has no tool %q", call.Name))
}

// DescribeAITool says what a clipboard tool call will do, naming the items by their first line from the history. An id
// the history no longer has is refused here, before the user is asked.
func (Extension) DescribeAITool(ctx api.Context, call api.AIToolCall) (string, error) {
	args, err := clipboardArgsOf(call)
	if err != nil {
		return "", err
	}
	switch call.Name {
	case toolSearch:
		if args.Query == "" {
			return "Read the newest clipboard history items", nil
		}
		return fmt.Sprintf("Read the clipboard history items that contain “%s”", args.Query), nil
	case toolCopy:
		titles, err := entryTitles(ctx, []string{args.ID})
		return fmt.Sprintf("Copy %s to the clipboard", strings.Join(titles, "")), err
	case toolDelete:
		return describeDeletion(ctx, args.IDs)
	}
	return "", clipboardRefusal(fmt.Sprintf("clipboard history has no tool %q", call.Name))
}

func describeDeletion(ctx api.Context, ids []string) (string, error) {
	if len(ids) == 0 || len(ids) > maxRemoveBatch {
		return "", clipboardRefusal(fmt.Sprintf("name between 1 and %d items to delete", maxRemoveBatch))
	}
	titles, err := entryTitles(ctx, ids)
	if err != nil {
		return "", err
	}
	if len(titles) == 1 {
		return fmt.Sprintf("Delete %s from clipboard history", titles[0]), nil
	}
	shown := titles[:min(len(titles), 3)]
	more := ""
	if len(titles) > len(shown) {
		more = fmt.Sprintf(" and %d more", len(titles)-len(shown))
	}
	return fmt.Sprintf("Delete %d items from clipboard history: %s%s", len(titles), strings.Join(shown, ", "), more), nil
}

// entryTitles names each item by its first line, quoted, as the list shows it.
func entryTitles(ctx api.Context, ids []string) ([]string, error) {
	titles := make([]string, 0, len(ids))
	for _, id := range ids {
		entry, found, err := ctx.Clipboard().Get(id)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, clipboardRefusal(fmt.Sprintf("no clipboard history item has the id %q; search first", id))
		}
		titles = append(titles, "“"+titleOf(entry)+"”")
	}
	return titles, nil
}

func toolEntries(entries []api.ClipboardEntry) []toolEntry {
	out := make([]toolEntry, 0, len(entries))
	for _, entry := range entries {
		item := toolEntry{ID: entry.ID, Type: entry.Type, Files: entry.FilePaths,
			CopiedAt: time.UnixMilli(entry.CreatedAtMs).UTC().Format(time.RFC3339)}
		if entry.Type != kindFiles {
			item.Text = truncate(entry.Text, maxToolTextRunes)
		}
		out = append(out, item)
	}
	return out
}

func toolResultLimit(requested int) int {
	if requested <= 0 {
		return defaultToolResults
	}
	return min(requested, maxToolResults)
}

func clipboardArgsOf(call api.AIToolCall) (clipboardToolArgs, error) {
	var args clipboardToolArgs
	if len(call.Args) > 0 && json.Unmarshal(call.Args, &args) != nil {
		return args, clipboardRefusal(fmt.Sprintf("the arguments of %s do not fit it", call.Name))
	}
	return args, nil
}

func clipboardRefusal(reason string) error {
	return fmt.Errorf("%s (%w)", reason, api.ErrInvalidInput)
}
