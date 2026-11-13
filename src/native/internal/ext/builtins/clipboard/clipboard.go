// Package clipboard is the Clipboard History built-in: the copies at the root, the Clipboard History list with its
// preview, copy, paste-as-text and removal actions, and the root settings that show copies. The watcher, the store
// and the picture files stay in the host, behind the clipboard.history and desktop.clipboard services.
//
// Layer: ext. The package imports api, search and the standard library.
package clipboard

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.clipboard"

// ListCommandID names the Clipboard History command, the list of copies.
const ListCommandID = "clipboard-history"

// Titles the command and the list keep from the native rows.
const (
	listTitle       = "Clipboard History"
	listPlaceholder = "Search Clipboard History"
	emptyTitle      = "No clipboard items found."
	emptySubtitle   = "Copy text or images and they will appear here."
)

// manageSectionTitle titles the group of the list's removal actions, as 1.x does.
const manageSectionTitle = "Manage History"

// Refs of the actions the rows, the views and the removals run. Args carry the entry id or the removal window.
const (
	refCopy         = "copy"
	refPreview      = "preview"
	refRemove       = "remove"
	refRemoveWindow = "remove-window"
	refRemoveAll    = "remove-all"
	refRemoveLast   = "remove-last"
	refSearch       = "search"
	refRefresh      = "refresh"
	argID           = "id"
	argWindow       = "window"
)

// Scores of the root rows, as the native root gave them: the copies at the empty root, and the copies a search finds.
const (
	scoreRootCopy      = 60
	scoreSearchCopy    = 55
	scoreRootCommand   = 14
	rootCopyLimit      = 10
	rootSearchLimit    = 5
	historyListLimit   = 300
	searchPatchLimit   = 300
	shortcutPasteText  = "Control+Enter"
	shortcutRemoveItem = "Control+Backspace"
	shortcutPreview    = "Control+Y"
)

// Options are the settings the root and the list read on each call, so a change shows at once. Their storage belongs
// to the host, which passes a reader.
type Options struct {
	RootList     bool
	RootSearch   bool
	LinkPreviews bool
}

// Extension is the Clipboard History built-in. options reads the current settings.
type Extension struct {
	options func() Options
}

// New returns the Clipboard History built-in, reading its settings through options. Nil options show every copy and
// preview every link, the defaults the settings start with.
func New(options func() Options) Extension {
	if options == nil {
		options = defaultOptions
	}
	return Extension{options: options}
}

// defaultOptions are the settings a new install has: copies show at the root and in its search, and links preview.
func defaultOptions() Options {
	return Options{RootList: true, RootSearch: true, LinkPreviews: true}
}

// Manifest identifies the built-in. It reads the history and writes the clipboard.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: ID, Title: listTitle},
		Capabilities: []api.Capability{
			api.CapabilityClipboardHistory, api.CapabilityDesktopClipboard, api.CapabilityDesktopKeyboard,
		},
	}
}

// Commands lists Clipboard History at the root, as the native command did.
func (Extension) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID: ListCommandID, Title: listTitle, Subtitle: "Browse and paste what you copied", Icon: "clipboard",
		Keywords: []string{"clipboard", "paste", "copy", "history"}, Score: scoreRootCommand,
		Appearance: &api.Appearance{Foreground: api.ColorBlue}, Section: search.KindCommands,
		LearnKey: "cmd:" + listTitle,
	}}, nil
}

// RootItems lists the newest copies at the empty root, when the root list shows them.
func (e Extension) RootItems(ctx api.Context) ([]api.Item, error) {
	options := e.options()
	if !options.RootList {
		return nil, nil
	}
	entries, err := ctx.Clipboard().List(api.ClipboardQuery{Limit: rootCopyLimit})
	if err != nil {
		return nil, err
	}
	rows := make([]api.Item, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, copyRow(entry, scoreRootCopy, lookOf(ctx, options)))
	}
	return rows, nil
}

// SearchItems lists the copies that contain the query, as text. Only text copies are searched, as 1.x does.
func (e Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	options := e.options()
	if query == "" || !options.RootSearch {
		return nil, nil
	}
	entries, err := ctx.Clipboard().List(api.ClipboardQuery{Text: query, Limit: rootSearchLimit, Types: textTypes})
	if err != nil {
		return nil, err
	}
	rows := make([]api.Item, 0, len(entries))
	for i, entry := range entries {
		rows = append(rows, copyRow(entry, float64(scoreSearchCopy-i), lookOf(ctx, options)))
	}
	return rows, nil
}

// textTypes are the copies the root search finds: text, with or without markup.
var textTypes = []string{"text", "html", "rtf"}

// lookOf is what the rows of one call read: the call's instant, whether the platform can paste, and the settings.
func lookOf(ctx api.Context, options Options) rowLook {
	return rowLook{Now: ctx.Clock().Now(), CanPaste: ctx.System().HasCapability("clipboard.paste"), Options: options}
}

// RunAction runs the list command, the rows' actions, the removals and the list's search and refresh.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	args := in.Ref.Args
	switch in.Ref.ID {
	case ListCommandID:
		view := e.listView(ctx, e.options())
		return ctx.Navigation().Push(&view), nil
	case refCopy:
		return copyEntry(ctx, args[argID])
	case refPreview:
		return previewEntry(ctx, args[argID])
	case refRemove:
		return removeEntry(ctx, args[argID])
	case refRemoveWindow:
		return removeWindow(ctx, args[argWindow])
	case refRemoveAll:
		return removeAll(ctx)
	case refRemoveLast:
		return pushRemoveLast(ctx), nil
	case refSearch:
		return searchResult(ctx, in.Value, e.options())
	case refRefresh:
		return refreshResult(ctx, e.options()), nil
	}
	return api.Result{}, fmt.Errorf("the clipboard built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}
