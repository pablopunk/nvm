package app

import (
	"fmt"
	"github.com/pablopunk/nvm/native/internal/search"
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/design"
)

const (
	width, height = cardW, paneH
	iconSize      = 28
)

// The palette is the search.Host every action runs against.
var _ search.Host = (*Palette)(nil)

// Palette is the whole app: one panel window, a stack of views, and the feature stores.
type Palette struct {
	panel *mygo.Window
	S     *Settings

	stack   []*View
	query   string
	results []search.Item
	chosen  int
	list    ui.ListState
	input   ui.Handle

	actionsOpen bool
	actChosen   int
	actList     ui.ListState

	icons    map[string]*ui.Bitmap
	toastMsg string
	md       markdown.Options

	apps     []mygo.InstalledApp
	exts     []loadedExt
	extTrays []*extTray
	runs     map[string]*chatRun
	tray     *mygo.Tray
	clip     *ClipStore
	snips    *SnippetStore
	notes    *NoteStore
	files    *FileIndex
	chats    *ChatStore
	emoji    emojiState
	timer    *Timer
	phIndex  int

	// settled is false while the providers of the shown query are still answering: the list then shows the
	// rows it has, and no empty state.
	settled    bool
	shownView  *View
	shownQuery string
	// syncSearch runs the root's providers in the calling goroutine: the goldens and the benchmarks need the
	// complete rows when a query returns.
	syncSearch bool
}

func (p *Palette) cur() *View { return p.stack[len(p.stack)-1] }

// Push opens a view above the current one. It implements search.Host, so the argument is the palette's
// *View; anything else is a programming error.
func (p *Palette) Push(v any) {
	view, ok := v.(*View)
	if !ok {
		panic("palette: Push wants a *View")
	}
	p.stack = append(p.stack, view)
	p.query = ""
	p.actionsOpen = false
	p.search()
	p.input.Focus()
}

// Query returns the text of the search field (search.Host).
func (p *Palette) Query() string { return p.query }

// SetQuery replaces the search text and searches again (search.Host).
func (p *Palette) SetQuery(q string) {
	p.query = q
	p.search()
}

func (p *Palette) Pop() {
	if len(p.stack) < 2 {
		return
	}
	p.leave(p.cur())
	p.stack = p.stack[:len(p.stack)-1]
	p.query = ""
	p.actionsOpen = false
	p.search()
	p.input.Focus()
}

func (p *Palette) Toast(msg string) { p.toastMsg = msg }

func (p *Palette) Hide() { p.panel.Hide() }

// Refresh re-runs the current search keeping the cursor, from any goroutine.
func (p *Palette) Refresh() {
	p.panel.Update(func() {
		keep := p.chosen
		p.invalidateRows()
		p.search()
		p.choose(keep)
	})
}

// search shows the rows of the view on top for the search text. A sub-view searches on the UI thread. The
// root's providers run on the view's coordinator: rows ready now are shown now, and the others arrive through
// applySnapshot.
func (p *Palette) search() {
	v := p.cur()
	q := strings.TrimSpace(p.query)
	if v.Providers == nil {
		var rows []search.Item
		if v.Search != nil {
			rows = v.Search(q)
		}
		p.showRows(v, q, rows, true)
		return
	}
	if snap, ready := p.coordinator(v).Query(q, v.Providers()); ready {
		p.showRows(v, q, snap.Items, snap.Settled)
	}
}

func (p *Palette) choose(i int) {
	p.chosen = max(0, min(i, len(p.results)-1))
	if len(p.results) > 0 {
		p.list.ScrollIntoView(p.chosen)
	}
}

func (p *Palette) chosenItem() *search.Item {
	if p.cur().Body != nil || p.chosen < 0 || p.chosen >= len(p.results) {
		return nil
	}
	return &p.results[p.chosen]
}

func (p *Palette) runDefault() {
	if it := p.chosenItem(); it != nil && len(it.Actions) > 0 {
		it.Actions[0].Run(p)
	}
}

// ---- view ----

const (
	winMargin    = 0
	cardW        = 720
	paneH        = 470
	cornerRadius = design.PaneRadius
)

var rootPlaceholders = []string{"Watcha gonna do?", "I cannot do that... Oh, nevermind.", "Make it happen."}

func (p *Palette) view(c *ui.Context) {
	if os.Getenv("NVM_THEME") == "dark" {
		dark := *ui.DarkTheme()
		dark.Background = ui.Transparent
		ui.Themed(c, &dark).Fill().Children(func() { p.viewThemed(c) })
		return
	}
	p.viewThemed(c)
}

func (p *Palette) viewThemed(c *ui.Context) {
	v := p.cur()
	if p.toastMsg != "" {
		design.Toast(c, p.toastMsg)
		p.toastMsg = ""
	}
	p.keys(c, v)
	c.Root().Background(ui.Transparent)
	design.Pane(c, func() {
		p.searchRow(c, v)
		design.Etched(c)
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

// searchRow is the field at the top of the pane; the design package draws it and reports what happened.
func (p *Palette) searchRow(c *ui.Context, v *View) {
	data := design.SearchData{
		HideInput: v.HideInput,
		Back:      len(p.stack) > 1,
		Query:     &p.query,
		Input:     &p.input,
		AIHint:    len(p.stack) == 1 && strings.TrimSpace(p.query) != "",
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
	if events.Back {
		p.Pop()
	}
	if events.Changed {
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
	p.list.Selected = &p.chosen
	p.list.Key = func(i int) any { return fmt.Sprint(i, p.results[i].ID, p.results[i].Title) }
	p.list.Label = func(i int) string { return p.results[i].Title }
	if p.chosen < 0 {
		p.chosen = 0
	}
	it := p.chosenItem()
	ui.Row(c).Grow(1).MinHeight(0).Children(func() {
		k := design.Tokens(c)
		quiet := *c.Theme()
		quiet.Background = ui.Transparent
		quiet.Accent, quiet.AccentHover, quiet.AccentPressed, quiet.AccentText = ui.Transparent, ui.Transparent, ui.Transparent, k.Strong
		var list ui.Element
		ui.Themed(c, &quiet).Grow(1).FillHeight().Children(func() {
			list = ui.List(c, &p.list, len(p.results), func(i int) { p.row(c, i) }).Grow(1).FillHeight().Padding(6)
		})
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

// row draws result i. A section header goes above the first row of each run of one section.
func (p *Palette) row(c *ui.Context, i int) {
	it := p.results[i]
	sel := i == p.chosen
	data := design.RowData{
		Title:     it.Title,
		Subtitle:  it.Subtitle,
		Accessory: it.Accessory,
		Tile:      p.tileData(it),
		Tone:      it.Tone,
		TintTitle: it.TintTitle,
		Prominent: it.Prominent,
		Selected:  sel,
		ShowEnter: sel && len(it.Actions) > 0,
	}
	if it.Section != "" && (i == 0 || p.results[i-1].Section != it.Section) {
		data.Section = it.Section
	}
	design.Row(c, data)
}

// tileData resolves a result's icon: a preview image, else the system icon of its file (decoded once and
// cached), else the Lucide icon or letters the design tile draws.
func (p *Palette) tileData(it search.Item) design.TileData {
	data := design.TileData{Image: it.Image, Icon: it.Icon, Glyph: it.Glyph}
	if it.Image == nil && it.IconPath != "" {
		data.FileIcon = p.icon(it.IconPath)
	}
	return data
}

func (p *Palette) icon(path string) *ui.Bitmap {
	if b, ok := p.icons[path]; ok {
		return b
	}
	if len(p.icons) > 400 {
		p.icons = map[string]*ui.Bitmap{}
	}
	var bm *ui.Bitmap
	if data, err := mygo.Shell.FileIcon(path, iconSize*2); err == nil {
		bm, _ = ui.DecodeBitmap(data)
	}
	p.icons[path] = bm
	return bm
}

// footer is the status bar: where you are, and what Enter and the action menu do.
func (p *Palette) footer(c *ui.Context, v *View) {
	name := v.Title
	if v.TitleFn != nil {
		name = v.TitleFn()
	}
	if len(p.stack) == 1 {
		name = "Nevermind"
	}
	data := design.FooterData{Name: name}
	if it := p.chosenItem(); it != nil && len(it.Actions) > 0 {
		data.Primary = &design.KeyHint{Title: it.Actions[0].Title, Key: kb("↵")}
		if len(it.Actions) > 1 {
			data.Secondary = &design.KeyHint{Title: "Actions", Key: kb("⌃K")}
		}
	} else if v.Hint != "" {
		data.Hint = kb(v.Hint)
	}
	design.Footer(c, data)
}

// actionPanel is the action menu, a popover over the pane.
func (p *Palette) actionPanel(c *ui.Context) {
	acts := p.actions()
	p.actList.Selected = &p.actChosen
	rows := make([]design.MenuRow, len(acts))
	for i, a := range acts {
		rows[i] = design.MenuRow{Title: a.Title, Key: kb(a.Label), Enter: a.Label == "" && i == 0, Selected: i == p.actChosen}
	}
	design.ActionMenu(c, rows)
}
