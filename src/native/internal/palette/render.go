package palette

import (
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

var rootPlaceholders = []string{"Watcha gonna do?", "I cannot do that... Oh, nevermind.", "Make it happen."}

// View draws the palette in the window. It is the content of the window the host creates.
func (p *Palette) View(c *ui.Context) {
	theme := design.WindowTheme(os.Getenv("NVM_THEME") == "dark" || c.Theme().Dark)
	theme.Background = ui.Transparent
	ui.Themed(c, theme).Fill().Children(func() { p.viewThemed(c) })
}

func (p *Palette) viewThemed(c *ui.Context) {
	v := p.cur()
	p.keys(c, v)
	c.Root().Background(ui.Transparent)
	design.Pane(c, func() {
		p.header(c, v)
		ui.Column(c).Grow(1).MinHeight(0).Clip().Children(func() {
			if v.Body != nil {
				v.Body(c, p)
			} else {
				p.listBody(c)
			}
		})
		design.Etched(c)
		p.footer(c, v)
	})
	if p.actionsOpen {
		p.actionPanel(c)
	}
}

// header is the top of the pane: the search field and the notices under it. A page that draws its own input has no
// search field, only the notices.
func (p *Palette) header(c *ui.Context, v *View) {
	if v.Input != nil {
		p.noticeRow(c)
		return
	}
	p.searchRow(c, v)
	p.noticeRow(c)
	design.Etched(c)
}

// searchRow is the field at the top of the pane; the design package draws it and reports what happened.
func (p *Palette) searchRow(c *ui.Context, v *View) {
	data := design.SearchData{
		HideInput:  v.HideInput,
		Back:       len(p.stack) > 1,
		Query:      &p.query,
		Input:      &p.input,
		CaretAtEnd: p.takeCaretToEnd(),
		AIHint:     len(p.stack) == 1 && strings.TrimSpace(p.query) != "",
		Concealed:  v.Concealed,
		OnKey:      p.onSearchKey,
	}
	if v.HideInput {
		data.Title = v.Title
		if v.TitleFn != nil {
			data.Title = v.TitleFn()
		}
	} else {
		data.Placeholder = v.Placeholder
		if data.Placeholder == "" {
			data.Placeholder = rootPlaceholders[p.phIndex%len(rootPlaceholders)]
		}
	}
	data.AccessoryOptions, data.AccessorySelected = accessoryChoices(v)
	events := design.SearchRow(c, data)
	p.searchSpan = events.Selection
	if v.Accessory != nil && events.AccessoryChoice >= 0 {
		v.Accessory.Choose(events.AccessoryChoice)
		p.search()
	}
	if events.Back {
		p.Pop()
	}
	if events.Changed {
		p.resetRootHistory()
		p.search()
	}
	if events.Submitted {
		p.onEnter()
	}
}

func (p *Palette) listBody(c *ui.Context) {
	v := p.cur()
	if len(p.results) == 0 {
		if !p.settled {
			return
		}
		title, sub := v.EmptyTitle, v.EmptySub
		if title == "" {
			title, sub = "No Results", "Try a different search."
		}
		design.EmptyState(c, design.EmptyData{Title: title, Subtitle: sub})
		return
	}
	if g := v.Grid; g != nil {
		p.gridBody(c, g)
		return
	}
	p.list.Selected = &p.chosen
	p.list.Key = func(i int) any { return fmt.Sprint(i, p.results[i].ID, p.results[i].Title) }
	p.list.Label = func(i int) string { return p.results[i].Title }
	if p.chosen < 0 {
		p.chosen = 0
	}
	it := p.chosenItem()
	if it != nil {
		p.announceSelection(c, it)
	}
	ui.Row(c).Grow(1).MinHeight(0).Children(func() {
		k := design.Tokens(c)
		quiet := *c.Theme()
		quiet.Background = ui.Transparent
		quiet.Accent, quiet.AccentHover, quiet.AccentPressed, quiet.AccentText = ui.Transparent, ui.Transparent, ui.Transparent, k.Strong
		var list ui.Element
		ui.Themed(c, &quiet).Grow(1).FillHeight().Children(func() {
			list = ui.List(c, &p.list, len(p.results), func(i int) { p.row(c, i) }).Grow(1).FillHeight().Padding(6)
		})
		p.focusListIfRequested(c)
		if list.Submitted() {
			p.runDefault()
		}
		if p.chosen < 0 && len(p.results) > 0 {
			p.chosen = 0
		}
		if it != nil && it.Detail != nil {
			ui.Column(c).Width(320).FillHeight().Padding(8, 8, 8, 2).Children(func() {
				ui.Column(c).Grow(1).Padding(14).Radius(12).Background(k.Wash).Border(1, k.Lo.Alpha(0.5)).InsetShadow(0, 1, 4, 0, k.Lo).Children(func() {
					it.Detail(c, p)
				})
			})
		}
	})
}

// selection identifies the row a keyboard user is on: the view, the query it came from, and the cursor.
type selection struct {
	view   *View
	query  string
	chosen int
}

// announceSelection tells assistive technology the title of the chosen row whenever the choice moves, so a
// keyboard user hears what is selected (PLAN.md J27). It announces only on a change.
func (p *Palette) announceSelection(c *ui.Context, it *search.Item) {
	now := selection{p.shownView, p.shownQuery, p.chosen}
	if now == p.announced {
		return
	}
	p.announced = now
	c.Announce(it.Title)
}

// row draws result i. A section header goes above the first row of each run of one section.
func (p *Palette) row(c *ui.Context, i int) {
	it := p.results[i]
	sel := i == p.chosen
	data := design.RowData{
		Title:      it.Title,
		Subtitle:   it.Subtitle,
		Accessory:  it.Accessory,
		Tile:       p.tileData(it),
		Tone:       it.Tone,
		TintTitle:  it.TintTitle,
		Prominent:  it.Prominent,
		Selected:   sel,
		Disabled:   it.Disabled,
		Contextual: it.Contextual,
		ShowEnter:  sel && len(it.Actions) > 0 && !it.Disabled,
	}
	if it.Section != "" && (i == 0 || p.results[i-1].Section != it.Section) {
		data.Section = it.Section
	}
	if len(it.DragPaths) > 0 && p.opts.DragRow != nil {
		p.opts.DragRow(c, it.DragPaths, func() { design.Row(c, data) })
		return
	}
	design.Row(c, data)
}

// tileData resolves a result's icon: a preview image, else the system icon of its file, else the Lucide icon
// or letters the design tile draws.
func (p *Palette) tileData(it search.Item) design.TileData {
	data := design.TileData{Image: it.Image, Icon: it.Icon, Glyph: it.Glyph}
	if it.Image == nil && it.IconPath != "" && p.opts.Icon != nil {
		data.FileIcon = p.opts.Icon(it.IconPath)
	}
	if data.Icon == "" {
		data.Icon = it.IconFallback
	}
	return data
}

// footer is the status bar: where you are, and what Enter and the action menu do.
func (p *Palette) footer(c *ui.Context, v *View) {
	name := v.Title
	if v.TitleFn != nil {
		name = v.TitleFn()
	}
	if v.Footer != nil {
		name = v.Footer(p.chosenItem())
	}
	if len(p.stack) == 1 {
		name = "Nevermind"
	}
	data := design.FooterData{Name: name}
	if it := p.chosenItem(); it != nil && len(it.Actions) > 0 {
		data.Primary = &design.KeyHint{Title: it.Actions[0].Title, Key: KeyLabel("↵")}
		if len(it.Actions) > 1 {
			data.Secondary = actionsKeyHint()
		}
	} else if v.Body != nil && (v.Primary != nil || len(p.actions()) > 0) {
		p.bodyFooterKeys(v, &data)
	} else if hint := viewHint(v); hint != "" {
		data.Hint = KeyLabel(hint)
	}
	design.Footer(c, data)
}

// bodyFooterKeys says what the primary control of a Body page does and that its action menu opens with Command-K, as
// a list's footer does for its rows.
func (p *Palette) bodyFooterKeys(v *View, data *design.FooterData) {
	if v.Primary != nil {
		data.Primary = v.Primary()
	}
	if len(p.actions()) > 0 {
		data.Secondary = actionsKeyHint()
	}
}

// actionsKeyHint is the footer's "Actions" entry: the key that opens the action menu.
func actionsKeyHint() *design.KeyHint {
	return &design.KeyHint{Title: "Actions", Key: KeyLabel("⌃K")}
}

// actionPanel is the action menu, a popover over the pane.
func (p *Palette) actionPanel(c *ui.Context) {
	acts := p.actions()
	p.actList.Selected = &p.actChosen
	rows := make([]design.MenuRow, len(acts))
	for i, a := range acts {
		rows[i] = design.MenuRow{Title: a.Title, Key: KeyLabel(a.Label), Enter: a.Label == "" && i == 0, Selected: i == p.actChosen,
			Disabled: a.Disabled != "", Subtitle: a.Disabled}
	}
	events := design.ActionMenu(c, rows)
	if events.Hovered >= 0 {
		p.actChosen = events.Hovered
	}
	if events.Clicked >= 0 {
		p.runMenuAction(events.Clicked)
	}
}

// accessoryChoices are the picker's option titles and the index of its current option; no titles when the view has
// no picker.
func accessoryChoices(v *View) ([]string, int) {
	if v.Accessory == nil {
		return nil, 0
	}
	return v.Accessory.Titles(), v.Accessory.SelectedIndex()
}

// viewHint is the right footer text of a view: its HintFn when it has one, its fixed Hint otherwise.
func viewHint(v *View) string {
	if v.HintFn != nil {
		return v.HintFn()
	}
	return v.Hint
}
