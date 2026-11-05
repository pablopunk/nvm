package extview

import (
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionTone is the tint of every extension row's tile, the same as the rows the extension list shows.
const extensionTone = "purple"

// rowTone is the tint of an item's tile and title: the item's Appearance color, the extension tone when it names none,
// or no tint when it names the neutral foreground.
func rowTone(item api.Item) string {
	if item.Appearance == nil || item.Appearance.Foreground == "" {
		return extensionTone
	}
	if item.Appearance.Foreground == api.ColorNone {
		return ""
	}
	return string(item.Appearance.Foreground)
}

// defaultIcon draws an item that names no icon.
const defaultIcon = "puzzle"

// rowIcon is the icon of an item's tile. A tile-glyph item draws its glyph in place of an icon.
func rowIcon(item api.Item) string {
	if item.TileGlyph && item.Glyph != "" {
		return ""
	}
	return iconOr(item.Icon, defaultIcon)
}

// row is the palette row of one item: its tile, title, subtitle, the first accessory, its detail pane and its actions.
// A disabled item keeps its text but has no actions.
func (s *Session) row(item api.Item) search.Item {
	row := search.Item{
		ID: item.ID, Title: item.Title, Subtitle: item.Subtitle, Glyph: item.Glyph,
		Icon: rowIcon(item), Tone: rowTone(item), TintTitle: item.TintTitle,
		Accessory: accessoryText(item.Accessories),
		Detail:    s.detailPane(item.Detail),
		Image:     s.bitmap(item.Image),
		Disabled:  item.Disabled, Contextual: item.Contextual,
		Prominent: item.Appearance != nil && item.Appearance.Prominence == api.ProminencePrimary,
		DragPaths: api.DragPaths(item), DragImage: api.DragImagePath(item),
	}
	if item.Detail != nil {
		s.bitmap(item.Detail.Image)
	}
	if item.Disabled {
		return row
	}
	row.Actions = s.itemActions(item)
	return row
}

// itemActions is the item's primary action first, then its other actions. An action repeated in the list is shown once.
func (s *Session) itemActions(item api.Item) []search.Action {
	var wire []api.Action
	if item.Primary != nil {
		wire = append(wire, *item.Primary)
	}
	for _, action := range item.Actions {
		if item.Primary == nil || !sameAction(*item.Primary, action) {
			wire = append(wire, action)
		}
	}
	if item.Shortcut != "" && len(wire) > 0 && wire[0].Shortcut == "" {
		wire[0].Shortcut = item.Shortcut
	}
	out := make([]search.Action, 0, len(wire))
	for i, action := range wire {
		out = append(out, s.actionAt(item.ID, i, action))
	}
	return out
}

// sameAction is true when two wire actions do the same thing, so a Primary repeated in Actions is not listed twice.
func sameAction(a, b api.Action) bool {
	if a.Kind != b.Kind || a.Title != b.Title {
		return false
	}
	if a.Ref != nil && b.Ref != nil {
		return a.Ref.ID == b.Ref.ID
	}
	return a.Ref == nil && b.Ref == nil && a.URL == b.URL && a.Path == b.Path && a.Text == b.Text
}

// accessoryText is the first accessory's text, with a count of the others, because a row has one trailing label.
func accessoryText(accessories []api.Accessory) string {
	var first string
	for _, accessory := range accessories {
		if accessory.Text != "" {
			first = accessory.Text
			break
		}
	}
	if extra := len(accessories) - 1; extra > 0 && first != "" {
		return first + " +" + strconv.Itoa(extra)
	}
	return first
}

// matchesQuery is true for an empty query and for one that the item's title, subtitle, keywords or aliases match.
func matchesQuery(item api.Item, query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return true
	}
	text := strings.Join(append([]string{item.Title, item.Subtitle}, append(item.Keywords, item.Aliases...)...), " ")
	return search.Score(text, q) > 0
}

// collectionPage is the host-owned CRUD list: an add row, then one row per record with preview, edit and remove.
func (s *Session) collectionPage() *palette.View {
	spec := s.view.Collection
	s.items = collectionItems(spec)
	page := s.listPage()
	page.EmptyTitle, page.EmptySub = orDefault(spec.EmptyTitle, "No items"), spec.EmptySubtitle
	page.EmptyFn = func() (string, string) { return page.EmptyTitle, page.EmptySub }
	page.Placeholder = orDefault(spec.SearchPlaceholder, page.Placeholder)
	return page
}

// collectionItems turns records into items. The Preview action is the primary one, Edit and Remove follow, and Remove
// asks first: a record's removal is destructive, so it is never one keystroke away.
func collectionItems(spec *api.CollectionSpec) []api.Item {
	items := make([]api.Item, 0, len(spec.Items)+1)
	if spec.Add != nil {
		add := *spec.Add
		add.Title = orDefault(add.Title, "Create item")
		items = append(items, api.Item{ID: collectionAddID, Title: add.Title, Icon: "plus", Primary: &add})
	}
	for _, record := range spec.Items {
		items = append(items, collectionItem(record))
	}
	return items
}

// editShortcut and removeShortcut are the record shortcuts of a collection: Cmd or Ctrl with E, and with Backspace.
const (
	editShortcut   = "CommandOrControl+E"
	removeShortcut = "CommandOrControl+Backspace"
)

// collectionAddID is the reserved id of the add row; records may not use it (api.CollectionSpec.Validate).
const collectionAddID = "__nvm:collection-create"

func collectionItem(record api.CrudItem) api.Item {
	item := api.Item{
		ID: record.ID, Title: record.Title, Subtitle: record.Subtitle, Icon: record.Icon,
		Accessories: record.Accessories, Keywords: record.Keywords, Aliases: record.Aliases, Detail: record.Detail,
	}
	if record.Preview != nil {
		item.Primary = record.Preview
	}
	if record.Edit != nil {
		edit := *record.Edit
		edit.Shortcut = orDefault(edit.Shortcut, editShortcut)
		item.Actions = append(item.Actions, edit)
	}
	if record.Remove != nil {
		remove := *record.Remove
		remove.Shortcut = orDefault(remove.Shortcut, removeShortcut)
		if remove.Confirm == nil {
			remove.Confirm = &api.Confirmation{
				Title: "Remove “" + record.Title + "”?", ConfirmLabel: "Remove", Destructive: true,
			}
		}
		item.Actions = append(item.Actions, remove)
	}
	if item.Primary == nil && len(item.Actions) > 0 {
		primary := item.Actions[0]
		item.Primary = &primary
		item.Actions = item.Actions[1:]
	}
	return item
}

func iconOr(icon, fallback string) string {
	if icon != "" {
		return icon
	}
	return fallback
}

func orDefault(text, fallback string) string {
	if text != "" {
		return text
	}
	return fallback
}
