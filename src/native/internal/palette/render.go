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
	frame := navigationFrame{}
	if p.opts.OnNavigation != nil {
		frame = p.captureNavigationFrame()
	}
	theme := design.WindowTheme(os.Getenv("NVM_THEME") == "dark" || c.Theme().Dark)
	theme.Background = ui.Transparent
	ui.Themed(c, theme).Fill().Children(func() { p.viewThemed(c) })
	p.revealRenderedWindow()
	if p.opts.OnNavigation != nil {
		p.reportNavigationFrame(frame)
	}
}

func (p *Palette) viewThemed(c *ui.Context) {
	v := p.cur()
	p.keys(c, v)
	c.Root().Background(ui.Transparent)
	design.PalettePane(c, func() {
		if p.drawsWindowHeader() {
			p.windowHeader(c, v)
		}
		p.header(c, v)
		ui.Column(c).Grow(1).MinHeight(0).Clip().Children(func() {
			p.splitBody(c, v, func() {
				if v.Body != nil {
					v.Body(c, p)
				} else {
					p.listBody(c)
				}
			})
		})
		if v.Body != nil || v.sideNow() != nil {
			ui.Box(c).Height(design.FooterClearance)
		}
		p.footer(c, v)
	})
	if side := v.sideNow(); side.Focused() && side.shell != nil && side.shell.actionsOpen {
		side.shell.actionPanel(c)
	} else if p.actionsOpen {
		p.actionPanel(c)
	}
}

// header is the top of the pane: the search field and the notices under it. A page that draws its own input has no
// search field, only the notices, unless it also hides its field and keeps its title row.
func (p *Palette) header(c *ui.Context, v *View) {
	if (v.Input != nil && !v.HideInput) || (p.drawsWindowHeader() && v.HideInput) {
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
		OnKey:      p.onSearchInputKey,
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
	events := design.SearchRow(c, data)
	p.searchSpan = events.Selection
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
	// SearchRow has committed this frame's text and dispatched its change/submit events.
	// Only now can a deferred Enter safely use the selected row for that query.
	p.resolveDefaultEnter(v, strings.TrimSpace(p.query), p.shownView == v && p.shownQuery == strings.TrimSpace(p.query) && p.settled)
}

func (p *Palette) listBody(c *ui.Context) {
	p.interact.pressing, p.interact.pressingNow = p.interact.pressingNow, false
	p.interact.clickedThisFrame = false
	v := p.cur()
	loading := p.loadingMore()
	if len(p.results) == 0 && loading {
		p.coldLoading(c, v.Grid)
		return
	}
	if len(p.results) == 0 && !loading {
		if !p.settled {
			return
		}
		title, sub := emptyText(v)
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
	// A controlled single selection tells the list when ranking chose a new item at the same index.
	p.listChoice.Clear()
	if p.chosen >= 0 && p.chosen < len(p.results) {
		p.listChoice.Add(listRowKey(p.results, p.chosen).(string))
	}
	p.list.Selection = &p.listChoice
	if len(p.results) == 0 {
		p.list.Selected = nil
	}
	p.list.Key = func(i int) any { return listRowKey(p.results, i) }
	p.list.Label = func(i int) string { return listRowLabel(p.results, i) }
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
		previousChoice := p.chosen
		var list ui.Element
		ui.Themed(c, &quiet).Grow(1).FillHeight().Children(func() {
			list = ui.List(c, &p.list, listRowCount(p.results, p.pendingBatch()), func(i int) { p.drawListRow(c, i) }).Grow(1).FillHeight().Padding(6, 6, 6+p.footerOverlayClearance(), 6)
			p.prepareFooterReveal(c, list, v)
		})
		if p.chosen != previousChoice {
			p.choicePinned = true
		}
		p.focusListIfRequested(c)
		if list.Submitted() && !p.interact.clickedThisFrame {
			p.runDefault()
		}
		if p.chosen < 0 && len(p.results) > 0 {
			p.chosen = 0
		}
		if p.showDetail(v, it) {
			detailColumn(c, v).FillHeight().Padding(8, 8, 8+p.footerOverlayClearance(), 2).Children(func() {
				ui.Column(c).Grow(1).Padding(14).Radius(12).Background(k.Wash).Border(1, k.Lo.Alpha(0.5)).InsetShadow(0, 1, 4, 0, k.Lo).Children(func() {
					it.Detail(c, p)
				})
			})
		}
	})
}

// listRowCount is the number of list rows: one per result, and one loading row after them while a batch arrives.
func listRowCount(results []search.Item, loading bool) int {
	if loading {
		return len(results) + 1
	}
	return len(results)
}

// listRowKey keeps an identified result mounted when its title or rank changes. Anonymous rows use their position
// with a learning key or title; the loading row after the results has a key of its own.
func listRowKey(results []search.Item, i int) any {
	if i >= len(results) {
		return "loading"
	}
	item := results[i]
	if item.ID != "" {
		return fmt.Sprintf("id:%q:%q", item.Kind, item.ID)
	}
	if item.LearnKey != "" {
		return fmt.Sprintf("learn:%d:%q:%q", i, item.Kind, item.LearnKey)
	}
	return fmt.Sprint(i, item.Kind, item.Title)
}

// listRowLabel is the assistive name of list row i. The loading row names what it waits for.
func listRowLabel(results []search.Item, i int) string {
	if i >= len(results) {
		return "Loading more results"
	}
	return results[i].Title
}

// drawListRow draws result i, or the loading row after the results while a batch arrives. The loading row is not a
// result: no choice or click reaches it.
func (p *Palette) drawListRow(c *ui.Context, i int) {
	if i >= len(p.results) {
		design.LoadingRow(c)
		return
	}
	p.row(c, i)
}

// sidePreviewMinWidth and sidePreviewWidthPercent size the detail of a side-preview list, as 1.x's side-preview card
// does: at least 300 points, and 40% of the pane when that is wider.
const (
	sidePreviewMinWidth     = 300
	sidePreviewWidthPercent = 40
	narrowDetailWidth       = 320
)

// detailColumn is the column of a chosen row's detail. A side-preview list sizes the detail beside its rows, and any
// other list keeps the detail at its narrow width.
func detailColumn(c *ui.Context, v *View) ui.Element {
	if v.Presentation == SidePreview {
		return ui.Column(c).WidthPercent(sidePreviewWidthPercent).MinWidth(sidePreviewMinWidth)
	}
	return ui.Column(c).Width(narrowDetailWidth)
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
		ShowEnter:  sel && hasDefaultAction(it) && !it.Disabled,
	}
	if it.Section != "" && (i == 0 || p.results[i-1].Section != it.Section) {
		data.Section = it.Section
	}
	row := ui.Column(c)
	row.Children(func() { p.listRow(c, i, func() { design.Row(c, data) }) })
	if sel {
		p.revealFooterRow(c, row)
	}
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
	root := len(p.stack) == 1
	if root {
		name = ""
	}
	loading := p.loadingActionOrView()
	if loading {
		status := "Loading…"
		if len(p.results) > 0 {
			status = "Refreshing…"
		}
		if name != "" {
			name += " · "
		}
		name += status
	}
	data := design.FooterData{Name: name, HideName: root && !loading, PrimaryAction: v.FooterPrimary}
	if v.FooterControlVisible == nil || v.FooterControlVisible() {
		data.Control = v.FooterControl
	}
	if data.Control == nil && v.Accessory != nil && len(v.Accessory.Options) > 0 {
		data.Control = p.filterControl(v.Accessory)
	}
	if it := p.actionableItem(); it != nil && len(it.Actions) > 0 {
		if action, ok := it.DefaultAction(); ok {
			data.Primary = &design.KeyHint{Title: action.Title, Key: KeyLabel("↵")}
		}
		if len(p.menuActions()) > 1 {
			data.Secondary = actionsKeyHint()
		}
		if v.Primary != nil {
			data.Secondary = v.Primary()
		}
	} else if v.Body != nil && (v.Primary != nil || len(p.menuActions()) > 0) && !v.HintReplacesActions {
		p.bodyFooterKeys(v, &data)
	} else if hint := viewHint(v); hint != "" {
		data.Hint = KeyLabel(hint)
	}
	if data.Secondary != nil && data.Secondary.Title == actionsKeyHint().Title {
		data.SecondaryAction = p.openActionsFromFooter
	}
	design.Footer(c, data)
}

// bodyFooterKeys says what the primary control of a Body page does and that its action menu opens with Command-K, as
// a list's footer does for its rows.
func (p *Palette) bodyFooterKeys(v *View, data *design.FooterData) {
	if v.Primary != nil {
		data.Primary = v.Primary()
	}
	if len(p.menuActions()) > 0 {
		data.Secondary = actionsKeyHint()
	}
}

// actionsKeyHint is the footer's "Actions" entry: the key that opens the action menu.
func actionsKeyHint() *design.KeyHint {
	return &design.KeyHint{Title: "Actions", Key: KeyLabel("⌃K")}
}

// filterControl is the footer button of a view's filter: the current option, and a menu of the options to choose from.
// A view has no control in its search field; the filter lives here, beside the footer's other buttons.
func (p *Palette) filterControl(filter *Accessory) func(*ui.Context) {
	return func(c *ui.Context) {
		choice := design.Select(c, design.SelectData{Options: filter.Titles(), Selected: filter.SelectedIndex()})
		if choice >= 0 {
			filter.Choose(choice)
			p.search()
		}
	}
}

// emptyText is the empty state's title and subtitle of a view: its EmptyFn when it has one, its fixed texts otherwise.
func emptyText(v *View) (string, string) {
	if v.EmptyFn != nil {
		return v.EmptyFn()
	}
	return v.EmptyTitle, v.EmptySub
}

// viewHint is the right footer text of a view: its HintFn when it has one, its fixed Hint otherwise.
func viewHint(v *View) string {
	if v.HintFn != nil {
		return v.HintFn()
	}
	return v.Hint
}

// openActionsFromFooter is a click on the footer's Actions hint: it opens the menu as the action-menu key does.
func (p *Palette) openActionsFromFooter() {
	if !p.actionsOpen && len(p.menuActions()) > 0 {
		p.openActionMenu(0)
	}
}

// showDetail reports whether the chosen row's side pane is drawn. A view that streams rows from providers decides it
// only when the list is settled or the user chose the row; in between it keeps the last decision, so rows that
// arrive while the user types cannot open or close the pane.
func (p *Palette) showDetail(v *View, it *search.Item) bool {
	has := it != nil && it.Detail != nil
	if v.Providers == nil || p.settled || p.choicePinned {
		p.detailOpen = has
	}
	return has && p.detailOpen
}
