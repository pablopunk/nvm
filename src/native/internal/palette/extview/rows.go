package extview

import (
	"slices"
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
	if !s.snapshotPending {
		item = WithPreviewActions(item)
		item = WithRevealShortcut(item, s.deps.RevealTitle)
	}
	row := search.Item{
		ID: item.ID, Title: item.Title, Subtitle: item.Subtitle, Glyph: item.Glyph,
		Icon: rowIcon(item), Tone: rowTone(item), TintTitle: item.TintTitle,
		Accessory:  accessoryText(item.Accessories),
		Detail:     s.detailPane(item.Detail),
		Image:      s.bitmap(item.Image),
		ImageShape: imageShape(item.Image),
		Disabled:   item.Disabled, Contextual: item.Contextual,
		Prominent: item.Appearance != nil && item.Appearance.Prominence == api.ProminencePrimary,
		DragPaths: api.DragPaths(item), DragImage: api.DragImagePath(item),
	}
	if item.Detail != nil {
		s.bitmap(item.Detail.Image)
	}
	if item.Disabled || s.snapshotPending {
		return row
	}
	row.Actions = s.itemActions(item)
	return row
}

// itemActions is the item's menu: its primary action first, then its action panel's actions (its actions when the
// panel is empty), then the view's actions the item does not repeat, under the View group. The view's actions are
// menu-only, so Enter still runs the item's own first action. A hidden item panel keeps the primary and the shortcuts
// but no menu, and a hidden view panel keeps its actions' shortcuts out of the menu.
func (s *Session) itemActions(item api.Item) []search.Action {
	own := ItemMenuSections(item)
	if item.Shortcut != "" && len(own) > 0 && own[0].Actions[0].Shortcut == "" && !sectionsHaveShortcut(own, item.Shortcut) {
		own[0].Actions[0].Shortcut = item.Shortcut
	}
	out := s.sectionActions(item.ID, 0, own)
	view := []MenuSection{{Title: viewGroupTitle, Actions: s.viewActionsNotIn(MenuWireActions(item.Primary, item.ActionPanel, item.Actions))}}
	for _, mapped := range s.sectionActions(item.ID, len(flattenSections(own)), view) {
		mapped.MenuHidden, mapped.MenuOnly = s.viewHidesMenu(), true
		out = append(out, mapped)
	}
	out = s.withAccessoryActions(out)
	if item.ActionPanelVisibility == api.PanelHidden {
		return HideFromMenu(out)
	}
	return out
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

// matchesQuery is true for an empty query and for one that the item's title, subtitle, keywords or aliases hold as a
// phrase, a substring or a word prefix. Scattered letters do not match a list row, as 1.x's list views require.
func matchesQuery(item api.Item, query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return true
	}
	text := strings.Join(append([]string{item.Title, item.Subtitle}, append(item.Keywords, item.Aliases...)...), " ")
	return search.Score(text, q) >= search.PhraseScoreFloor
}

// matchingItems returns the items that match query, an exact title first and then an exact keyword, as 1.x sorts a
// list's matches. Items of one rank keep their order.
func matchingItems(items []api.Item, query string) []api.Item {
	matches := slices.DeleteFunc(slices.Clone(items), func(item api.Item) bool { return !matchesQuery(item, query) })
	slices.SortStableFunc(matches, func(a, b api.Item) int { return queryRank(a, query) - queryRank(b, query) })
	return matches
}

// queryRank is 0 for an item whose title is the query, 1 for one with a keyword that is, and 2 for every other match.
func queryRank(item api.Item, query string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if strings.ToLower(strings.TrimSpace(item.Title)) == q {
		return 0
	}
	if slices.ContainsFunc(item.Keywords, func(keyword string) bool { return strings.ToLower(strings.TrimSpace(keyword)) == q }) {
		return 1
	}
	return 2
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

func sectionsHaveShortcut(sections []MenuSection, shortcut string) bool {
	mods, key, ok := parseShortcut(shortcut)
	if !ok {
		return false
	}
	for _, section := range sections {
		for _, action := range section.Actions {
			m, k, valid := parseShortcut(action.Shortcut)
			if valid && m == mods && k == key {
				return true
			}
		}
	}
	return false
}
