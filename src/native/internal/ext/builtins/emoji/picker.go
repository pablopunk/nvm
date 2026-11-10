package emoji

import (
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
)

// pasteCapability is the system capability that says the frontmost app takes a paste; without it Enter copies.
const pasteCapability = "clipboard.paste"

// allCharacters is the category that lists every entry, as the catalog library names it.
const allCharacters = "All Characters"

// toneKey is the storage key of the remembered skin tone.
const toneKey = "skinTone"

// pickerView is the Emoji & Symbols grid: a compact grid of the catalog for the typed text, its categories on Tab, and
// the meaning batch below the catalog's matches.
func (e *Extension) pickerView(ctx api.Context) api.View {
	return api.View{
		Kind: api.ViewGrid, Title: Title, Placeholder: "Search emoji, symbols, or Unicode values",
		Hint: hint(ctx), Subtitle: allCharacters,
		EmptyView:       &api.EmptyState{Title: "No matching characters", Subtitle: "Try a name, keyword, character, or Unicode value."},
		Grid:            &api.GridSpec{Columns: 8, Layout: "compact", CategoryKeys: true, SelectionFooter: true},
		MaxVisibleItems: RowLimit,
		Search:          &api.ActionRef{ID: rowsAction},
		OnSearchChange:  &api.ActionRef{ID: meaningAction},
		Filter: &api.ViewFilter{
			ID: categoryArgName, Tooltip: "Character category", Value: emoji.AllCategory, Options: categoryOptions(),
			OnChange: &api.ActionRef{ID: categoryAction},
		},
	}
}

// categoryOptions lists All Characters, then the catalog's groups, in catalog order.
func categoryOptions() []api.ViewFilterOption {
	names := categoryNames()
	options := make([]api.ViewFilterOption, 0, len(names)+1)
	options = append(options, api.ViewFilterOption{Title: allCharacters, Value: emoji.AllCategory})
	for _, name := range names {
		options = append(options, api.ViewFilterOption{Title: name, Value: name})
	}
	return options
}

// hint names the keys of the picker: Enter pastes where the platform can, and copies otherwise.
func hint(ctx api.Context) string {
	if ctx.System().HasCapability(pasteCapability) {
		return "↵ Paste   ⌃↵ Copy   ⇥ Category"
	}
	return "↵ Copy   ⇥ Category"
}

// categoryOf is the category a request names, or All Characters when it names none.
func categoryOf(args map[string]string) string {
	if value := args[categoryArgName]; value != "" {
		return value
	}
	return emoji.AllCategory
}

// canPaste reports whether Enter pastes into the frontmost app.
func canPaste(ctx api.Context) bool {
	return ctx.System().HasCapability(pasteCapability)
}

// rows answers the typed text with the catalog's matches for the category, in the library's order, up to the grid's
// limit. An empty text lists the category in catalog order.
func (e *Extension) rows(ctx api.Context, text, category string) []api.Item {
	idx := e.catalog(ctx)
	tone := e.tone(ctx)
	paste := canPaste(ctx)
	entries := characterLibrary.Search(strings.TrimSpace(text), emoji.Options{Category: category, Limit: RowLimit})
	items := make([]api.Item, 0, len(entries))
	for _, entry := range entries {
		if record, ok := idx.records[entry.ID()]; ok {
			items = append(items, characterItem(record, tone, paste, text))
		}
	}
	return items
}

// characterItem is the row of one character. Enter pastes where the platform can, and copies otherwise. The row's
// actions are Copy (with Ctrl+Enter, when Enter pastes), Skin Tone for characters with variants, and Copy Unicode.
func characterItem(record api.BundleRecord, tone emoji.SkinTone, paste bool, query string) api.Item {
	glyph := record.Fields["glyph"]
	hasSkinTone := record.Fields["skinTone"] == "true"
	if hasSkinTone {
		glyph = emoji.ApplySkinTone(glyph, tone)
	}
	title := record.Title
	item := api.Item{ID: record.ID, Title: title, Subtitle: record.Subtitle, Glyph: glyph, Keywords: record.Keywords}
	var actions []api.Action
	if paste {
		item.Primary = api.Primary(api.PasteText("Paste "+title, glyph))
		actions = append(actions, withShortcut(api.CopyText("Copy "+title, glyph, false), "Control+Enter"))
	} else {
		item.Primary = api.Primary(api.CopyText("Copy "+title, glyph, false))
	}
	if hasSkinTone {
		actions = append(actions, api.Run(skinAction, "Skin Tone…", map[string]string{idArgName: record.ID, queryArgName: query}))
	}
	actions = append(actions, withShortcut(api.CopyText("Copy Unicode", record.Subtitle, false), "Control+Shift+Alt+C"))
	item.Actions = actions
	return item
}

// withShortcut gives an action the shortcut the native row showed for it.
func withShortcut(action api.Action, shortcut string) api.Action {
	action.Shortcut = shortcut
	return action
}

// recordOf returns the bundled record of an entry id, or false when the dataset does not hold it.
func (idx *index) recordOf(id string) (api.BundleRecord, bool) {
	record, ok := idx.records[id]
	return record, ok
}

// intArg reads an integer argument, or zero when it is missing or not a number.
func intArg(args map[string]string, name string) int {
	value, _ := strconv.Atoi(args[name])
	return value
}
