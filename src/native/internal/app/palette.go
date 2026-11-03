package app

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	width, height = cardW, paneH
	iconSize      = 28
)

// Palette is the whole app: one panel window, a stack of views, and the feature stores.
type Palette struct {
	panel *mygo.Window
	S     *Settings

	stack   []*View
	query   string
	results []Item
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
}

func (p *Palette) cur() *View { return p.stack[len(p.stack)-1] }

func (p *Palette) Push(v *View) {
	p.stack = append(p.stack, v)
	p.query = ""
	p.actionsOpen = false
	p.search()
	p.input.Focus()
}

func (p *Palette) Pop() {
	if len(p.stack) < 2 {
		return
	}
	if f := p.cur().OnLeave; f != nil {
		f()
	}
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
		p.search()
		p.choose(keep)
	})
}

func (p *Palette) search() {
	v := p.cur()
	p.results = nil
	if v.Search != nil {
		p.results = v.Search(strings.TrimSpace(p.query))
	}
	p.choose(0)
}

func (p *Palette) choose(i int) {
	p.chosen = max(0, min(i, len(p.results)-1))
	if len(p.results) > 0 {
		p.list.ScrollIntoView(p.chosen)
	}
}

func (p *Palette) chosenItem() *Item {
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
	cornerRadius = 18
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
	k := tk(c)
	v := p.cur()
	if p.toastMsg != "" {
		c.Toast(p.toastMsg)
		p.toastMsg = ""
	}
	p.keys(c, v)
	c.Root().Background(ui.Transparent)
	// A system material (macOS glass / Windows acrylic) shows through; elsewhere the pane is
	// drawn in the theme's colors, slightly see-through where the compositor allows.
	pane := ui.Column(c).Fill().Radius(cornerRadius).Clip().Border(1, k.edgeStrong).InsetShadow(0, 1, 0, 0, k.hi)
	if !c.Vibrancy() {
		pane.Background(k.bg.Alpha(0.96))
	}
	pane.Children(func() {
		p.searchRow(c, v, k)
		etched(c, k)
		ui.Column(c).Grow(1).MinHeight(0).Clip().Children(func() {
			if v.Body != nil {
				v.Body(c, p)
			} else {
				p.listBody(c, k)
			}
		})
		etched(c, k)
		p.footer(c, v, k)
	})
	if p.actionsOpen {
		p.actionPanel(c, k)
	}
}

func (p *Palette) keys(c *ui.Context, v *View) {
	if p.actionsOpen {
		acts := p.actions()
		switch {
		case c.Shortcut(0, ui.KeyEscape), c.Shortcut(mod, ui.KeyK):
			p.actionsOpen = false
		case c.Shortcut(0, ui.KeyDown):
			p.actChosen = min(p.actChosen+1, len(acts)-1)
		case c.Shortcut(0, ui.KeyUp):
			p.actChosen = max(p.actChosen-1, 0)
		}
		return
	}
	if v.OnKey != nil && v.OnKey(c, p) {
		return
	}
	if c.Shortcut(mod, ui.KeyK) && len(p.actions()) > 0 {
		p.actionsOpen, p.actChosen = true, 0
		return
	}
	if len(p.stack) == 1 && strings.TrimSpace(p.query) != "" && c.Shortcut(0, ui.KeyTab) {
		p.Push(p.newChatView(strings.TrimSpace(p.query)))
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		switch {
		case p.query != "":
			p.query = ""
			p.search()
		case len(p.stack) > 1:
			p.Pop()
		default:
			p.Hide()
		}
		return
	}
	if p.query == "" && len(p.stack) > 1 && !v.HideInput && v.Body == nil && c.Shortcut(0, ui.KeyBackspace) {
		p.Pop()
		return
	}
	if v.Body != nil {
		return
	}
	if c.Shortcut(0, ui.KeyDown) {
		p.choose(p.chosen + 1)
	}
	if c.Shortcut(0, ui.KeyUp) {
		p.choose(p.chosen - 1)
	}
	if it := p.chosenItem(); it != nil {
		for _, a := range it.Actions[min(1, len(it.Actions)):] {
			if a.Key != 0 && c.Shortcut(a.Mods, a.Key) {
				a.Run(p)
				return
			}
		}
	}
}

// onEnter is Enter in the input: run the chosen action, or submit the text in input-driven views.
func (p *Palette) onEnter() {
	v := p.cur()
	switch {
	case p.actionsOpen:
		acts := p.actions()
		p.actionsOpen = false
		if p.actChosen < len(acts) {
			acts[p.actChosen].Run(p)
		}
	case v.Body != nil:
		if v.OnSubmit != nil {
			q := strings.TrimSpace(p.query)
			if !v.KeepQuery {
				p.query = ""
			}
			v.OnSubmit(q)
		}
	default:
		p.runDefault()
	}
}

func (p *Palette) actions() []Action {
	if it := p.chosenItem(); it != nil {
		return it.Actions
	}
	return nil
}

// keycap draws a key as a small rounded cap, as macOS menus and Raycast do.
func keycap(c *ui.Context, k *tokens, s string) {
	ui.Row(c).Padding(1, 6).Radius(5).Background(k.surface).Border(1, k.edgeStrong).InsetShadow(0, 1, 0, 0, k.hi).Center().Children(func() {
		ui.Text(c, kb(s)).FontSize(11).FontWeight(500).TextColor(k.muted).SingleLine()
	})
}

// searchRow is the field at the top of the pane.
func (p *Palette) searchRow(c *ui.Context, v *View, k *tokens) {
	ui.Row(c).Padding(14, 18).Gap(12).AlignItems(ui.Center).Children(func() {
		bolt := ui.Icon(c, svgIcon("zap")).FontSize(21).TextColor(k.accent)
		if len(p.stack) > 1 && bolt.Clicked() {
			p.Pop()
		}
		if v.HideInput {
			title := v.Title
			if v.TitleFn != nil {
				title = v.TitleFn()
			}
			ui.Text(c, title).FontSize(21).TextColor(k.strong).Grow(1).SingleLine()
			return
		}
		ph := v.Placeholder
		if ph == "" {
			ph = rootPlaceholders[p.phIndex%len(rootPlaceholders)]
		}
		in := ui.TextInputBase(c, &p.query).Bind(&p.input).Grow(1).FontSize(21).TextColor(k.strong).Placeholder(ph).PlaceholderColor(k.faint).Label("Search")
		if in.Changed() {
			p.search()
		}
		if in.Submitted() {
			p.onEnter()
		}
		if len(p.stack) == 1 && strings.TrimSpace(p.query) != "" {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(3, 8).Radius(6).Background(k.surface).Border(1, k.edgeStrong).InsetShadow(0, 1, 0, 0, k.hi).Children(func() {
				ui.Icon(c, svgIcon("sparkles")).FontSize(12).TextColor(k.muted)
				ui.Text(c, "Tab").FontSize(12).FontWeight(600).TextColor(k.muted)
				ui.Text(c, "to ask AI").FontSize(12).TextColor(k.muted)
			})
		}
	})
}

func (p *Palette) listBody(c *ui.Context, k *tokens) {
	v := p.cur()
	if len(p.results) == 0 {
		ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
			ui.Icon(c, svgIcon("search")).FontSize(26).TextColor(k.faint)
			title, sub := v.EmptyTitle, v.EmptySub
			if title == "" {
				title, sub = "No Results", "Try a different search."
			}
			ui.Text(c, title).FontSize(15).FontWeight(600).TextColor(k.strong)
			ui.Text(c, sub).FontSize(12).TextColor(k.muted)
		})
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
		quiet := *c.Theme()
		quiet.Background = ui.Transparent
		quiet.Accent, quiet.AccentHover, quiet.AccentPressed, quiet.AccentText = ui.Transparent, ui.Transparent, ui.Transparent, k.strong
		var list ui.Element
		ui.Themed(c, &quiet).Grow(1).FillHeight().Children(func() {
			list = ui.List(c, &p.list, len(p.results), func(i int) { p.row(c, i, k) }).Grow(1).FillHeight().Padding(6)
		})
		if list.Submitted() {
			p.runDefault()
		}
		if p.chosen < 0 && len(p.results) > 0 {
			p.chosen = 0
		}
		if it != nil && it.Detail != nil {
			ui.Column(c).Width(320).FillHeight().Padding(8, 8, 8, 2).Children(func() {
				ui.Column(c).Grow(1).Padding(14).Radius(12).Background(k.wash).Border(1, k.lo.Alpha(0.5)).InsetShadow(0, 1, 4, 0, k.lo).Children(func() {
					it.Detail(c, p)
				})
			})
		}
	})
}

func (p *Palette) row(c *ui.Context, i int, k *tokens) {
	it := p.results[i]
	sel := i == p.chosen
	ui.Column(c).Children(func() {
		if it.Section != "" && (i == 0 || p.results[i-1].Section != it.Section) {
			ui.Text(c, it.Section).FontSize(11).FontWeight(600).TextColor(k.muted).Padding(8, 10, 4, 10)
		}
		title, sub := k.strong, k.tertiary
		if it.TintTitle || it.Prominent {
			if col, ok := k.tones[it.Tone]; ok {
				title = col
			}
		}
		rowEl := ui.Row(c).Padding(5, 8).Gap(10).AlignItems(ui.Center).Radius(12)
		if it.Prominent && !sel {
			rowEl.Background(k.accentSoft)
		}
		if sel {
			selectedFill(rowEl, k, it.Prominent)
		}
		rowEl.Children(func() {
			p.tile(c, it, k)
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, it.Title).FontSize(13).FontWeight(600).TextColor(title).SingleLine()
				if it.Subtitle != "" {
					ui.Text(c, it.Subtitle).FontSize(11).TextColor(sub).SingleLine()
				}
			})
			if it.Accessory != "" {
				ui.Text(c, it.Accessory).FontSize(12).TextColor(k.muted).SingleLine()
			}
			if sel && len(it.Actions) > 0 {
				ui.Icon(c, svgIcon("corner-down-left")).FontSize(13).TextColor(k.muted)
			}
		})
	})
}

// tile is the icon of a row: the file's own icon, or a tinted glyph on a small raised key.
func (p *Palette) tile(c *ui.Context, it Item, k *tokens) {
	const size = 32
	switch {
	case it.Image != nil:
		ui.Image(c, it.Image).Size(size, size).Fit(ui.Cover).Radius(8).Clip()
	case it.IconPath != "":
		if bm := p.icon(it.IconPath); bm != nil {
			ui.Row(c).Size(size, size).Center().Children(func() { ui.Image(c, bm).Size(28, 28) })
			return
		}
		fallthrough
	default:
		fg := k.secondary
		if col, ok := k.tones[it.Tone]; ok {
			fg = col
		}
		el := ui.Row(c).Size(size, size).Center()
		if it.Prominent {
			glossy(el, k.accentFill, 8, 0)
			fg = k.accentContrast
		} else {
			el.Radius(8).Background(k.interactive).Border(1, k.edgeSubtle).
				InsetShadow(0, 1, 0, 0, k.hi)
		}
		el.Children(func() {
			if it.Icon != "" {
				ui.Icon(c, svgIcon(it.Icon)).FontSize(17).TextColor(fg)
				return
			}
			g := it.Glyph
			if g == "" {
				g = it.Title
			}
			r := []rune(strings.ToUpper(g))
			ui.Text(c, string(r[:min(2, len(r))])).FontSize(12).FontWeight(700).TextColor(fg)
		})
	}
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
func (p *Palette) footer(c *ui.Context, v *View, k *tokens) {
	ui.Row(c).Padding(8, 14).Gap(14).AlignItems(ui.Center).Background(k.wash).Children(func() {
		ui.Icon(c, svgIcon("zap")).FontSize(14).TextColor(k.muted)
		name := v.Title
		if v.TitleFn != nil {
			name = v.TitleFn()
		}
		if len(p.stack) == 1 {
			name = "Nevermind"
		}
		ui.Text(c, name).FontSize(12).TextColor(k.muted).Grow(1).SingleLine()
		if it := p.chosenItem(); it != nil && len(it.Actions) > 0 {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, it.Actions[0].Title).FontSize(12).FontWeight(500).TextColor(k.strong)
				keycap(c, k, "↵")
			})
			if len(it.Actions) > 1 {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Actions").FontSize(12).FontWeight(500).TextColor(k.strong)
					keycap(c, k, "⌃K")
				})
			}
		} else if v.Hint != "" {
			ui.Text(c, kb(v.Hint)).FontSize(12).TextColor(k.muted)
		}
	})
}

// actionPanel is the action menu, a popover over the pane.
func (p *Palette) actionPanel(c *ui.Context, k *tokens) {
	acts := p.actions()
	p.actList.Selected = &p.actChosen
	ui.Column(c).Absolute().Right(12).Bottom(44).Width(290).Radius(12).Background(k.raised).Border(1, k.edgeStrong).InsetShadow(0, 1, 0, 0, k.hi).Shadow(0, 14, 34, 0, k.shadow).Shadow(0, 2, 6, 0, k.lo).Padding(5).Children(func() {
		for i, a := range acts {
			fg := k.strong
			rowEl := ui.Row(c).Padding(6, 10).Radius(8).AlignItems(ui.Center).Gap(8)
			if i == p.actChosen {
				selectedFill(rowEl, k, false)
			}
			rowEl.Children(func() {
				ui.Text(c, a.Title).FontSize(13).TextColor(fg).Grow(1).SingleLine()
				if a.Label != "" {
					ui.Text(c, kb(a.Label)).FontSize(11).FontWeight(500).TextColor(k.muted).SingleLine()
				} else if i == 0 {
					ui.Icon(c, svgIcon("corner-down-left")).FontSize(12).TextColor(k.muted)
				}
			})
		}
	})
}

// ---- window ----

func (p *Palette) show() {
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	w, h := p.panel.Size()
	p.panel.SetPosition(d.WorkArea.X+(d.WorkArea.Width-w)/2, d.WorkArea.Y+(d.WorkArea.Height-h)/3)
	p.phIndex++
	for len(p.stack) > 1 {
		if f := p.cur().OnLeave; f != nil {
			f()
		}
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.query = ""
	p.actionsOpen = false
	p.search()
	go func() {
		apps, err := mygo.Shell.InstalledApps()
		if err != nil {
			log.Println(err)
			return
		}
		p.panel.Update(func() { p.apps = apps; p.search() })
	}()
	p.input.Focus()
	p.panel.Show()
}

func (p *Palette) toggle() {
	if p.panel.IsVisible() {
		p.panel.Hide()
	} else {
		p.show()
	}
}

// open shows the palette directly in a sub-view (from the tray or a hotkey).
func (p *Palette) open(v *View) {
	p.show()
	p.Push(v)
}

// pasteText copies text and sends Ctrl+V to whatever had focus before.
func pasteText(text string) {
	mygo.Clipboard.WriteText(text)
	go func() {
		time.Sleep(180 * time.Millisecond)
		if path, err := exec.LookPath("xdotool"); err == nil {
			_ = exec.Command(path, "key", "--clearmodifiers", "ctrl+v").Run()
		}
	}()
}
