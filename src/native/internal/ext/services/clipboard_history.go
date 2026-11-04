package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
)

// History listing and removal bounds.
const (
	defaultHistoryList = 50
	maxHistoryList     = 500
	maxHistoryIDs      = 200
)

// serveClipboardHistory reads and removes Nevermind's clipboard history. The history never stores concealed copies,
// so nothing here can return them. Removing and clearing need a confirmed palette action.
func (h *Handler) serveClipboardHistory(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityClipboardHistory); err != nil {
		return nil, err
	}
	history := h.historyOrNil()
	if history == nil {
		return nil, unavailable("clipboard history is not available yet")
	}
	switch call.Op {
	case protocol.OpList:
		return h.listHistory(history, call.Args)
	case protocol.OpGet:
		return h.getHistoryEntry(history, call.Args)
	case protocol.OpRemove:
		return h.removeHistory(ctx, history, call.Args)
	case protocol.OpClear:
		return h.clearHistory(ctx, history, call.Args)
	}
	return nil, invalid("clipboard.history has no operation %q", call.Op)
}

func (h *Handler) historyOrNil() *clipboard.History {
	if h.deps.History == nil {
		return nil
	}
	return h.deps.History()
}

func (h *Handler) listHistory(history *clipboard.History, raw json.RawMessage) ([]api.ClipboardEntry, error) {
	var query api.ClipboardQuery
	if err := decodeArgs(raw, &query); err != nil {
		return nil, err
	}
	kinds, err := kindsOf(query.Types)
	if err != nil {
		return nil, err
	}
	entries := history.Find(clipboard.Query{Text: query.Text, Kinds: kinds, Limit: listLimit(query.Limit)})
	out := make([]api.ClipboardEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entryOf(entry))
	}
	return out, nil
}

func (h *Handler) getHistoryEntry(history *clipboard.History, raw json.RawMessage) (protocol.ClipboardGetResult, error) {
	var args protocol.KeyArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.ClipboardGetResult{}, err
	}
	entry, found := history.Get(args.Key)
	if !found {
		return protocol.ClipboardGetResult{}, nil
	}
	return protocol.ClipboardGetResult{Found: true, Entry: entryOf(entry)}, nil
}

func (h *Handler) removeHistory(ctx context.Context, history *clipboard.History, raw json.RawMessage) (int, error) {
	if err := requireRun(protocol.RequestMethodOf(ctx), "removing clipboard history"); err != nil {
		return 0, err
	}
	var args protocol.IDsArgs
	if err := decodeArgs(raw, &args); err != nil {
		return 0, err
	}
	if len(args.IDs) > maxHistoryIDs {
		return 0, invalid("at most %d entries can be removed at once", maxHistoryIDs)
	}
	if err := h.confirmDestructive(ctx, fmt.Sprintf("Remove %d clipboard items", len(args.IDs))); err != nil {
		return 0, err
	}
	return history.Remove(clipboard.RangeQuery{Range: clipboard.RangeIDs, ItemIDs: args.IDs})
}

func (h *Handler) clearHistory(ctx context.Context, history *clipboard.History, raw json.RawMessage) (int, error) {
	if err := requireRun(protocol.RequestMethodOf(ctx), "clearing clipboard history"); err != nil {
		return 0, err
	}
	var args protocol.ClearArgs
	if err := decodeArgs(raw, &args); err != nil {
		return 0, err
	}
	kinds, err := kindsOf(args.Types)
	if err != nil {
		return 0, err
	}
	if err := h.confirmDestructive(ctx, "Clear clipboard history"); err != nil {
		return 0, err
	}
	if len(kinds) == 0 && args.OlderThanMs == 0 {
		return history.ClearUnpinned()
	}
	query := clipboard.RangeQuery{Range: clipboard.RangeAll, Kinds: kinds}
	if args.OlderThanMs > 0 {
		query.Range = clipboard.RangeOlderThan
		query.OlderThan = time.Duration(args.OlderThanMs) * time.Millisecond
	}
	return history.Remove(query)
}

// kindsOf turns the type names an extension passed into history kinds. An unknown name is invalid input.
func kindsOf(names []string) ([]clipboard.Kind, error) {
	kinds := make([]clipboard.Kind, 0, len(names))
	for _, name := range names {
		kind := clipboard.Kind(name)
		if !kind.Valid() {
			return nil, invalid("%q is not a clipboard type", name)
		}
		kinds = append(kinds, kind)
	}
	return kinds, nil
}

func listLimit(requested int) int {
	switch {
	case requested <= 0:
		return defaultHistoryList
	case requested > maxHistoryList:
		return maxHistoryList
	default:
		return requested
	}
}

// entryOf maps a history entry to the wire form. Image bytes never leave the host: an image entry carries its type only.
func entryOf(entry clipboard.Entry) api.ClipboardEntry {
	out := api.ClipboardEntry{ID: entry.ID, Type: string(entry.Kind), CreatedAtMs: entry.CreatedAt.UnixMilli()}
	switch entry.Kind {
	case clipboard.KindText, clipboard.KindHTML, clipboard.KindRTF:
		out.Text = entry.Text
	case clipboard.KindFiles:
		if len(entry.Paths) > 0 {
			out.FilePath = entry.Paths[0]
		}
	}
	return out
}
