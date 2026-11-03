package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

type emojiEntry struct{ Glyph, Name string }

type emojiState struct {
	all      []emojiEntry
	filtered []emojiEntry
	lastQ    string
	chosen   int
	list     ui.ListState
}

const emojiCols = 12

func (s *emojiState) load() {
	for _, l := range strings.Split(emojiData, "\n") {
		g, name, ok := strings.Cut(l, " ")
		if ok {
			s.all = append(s.all, emojiEntry{g, name})
		}
	}
}

func (s *emojiState) refilter(q string) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == s.lastQ && s.filtered != nil {
		return
	}
	s.lastQ, s.chosen = q, 0
	s.filtered = s.filtered[:0]
	for _, e := range s.all {
		if q == "" || strings.Contains(e.Name, q) {
			s.filtered = append(s.filtered, e)
		}
	}
}

func (p *Palette) emojiView() *View {
	s := &p.emoji
	s.lastQ, s.filtered = "\x00", nil
	pick := func(paste bool) {
		if s.chosen < len(s.filtered) {
			g := s.filtered[s.chosen].Glyph
			p.Hide()
			if paste {
				pasteText(g)
			} else {
				mygo.Clipboard.WriteText(g)
			}
		}
	}
	return &View{
		Title: "Emoji & Symbols", Placeholder: "Search Emoji & Symbols",
		Hint: "↵ Copy   ⌃↵ Paste   ←↑↓→ Move", KeepQuery: true,
		OnSubmit: func(string) { pick(false) },
		Status: func() string {
			if s.chosen < len(s.filtered) {
				e := s.filtered[s.chosen]
				return e.Glyph + "  " + strings.Fields(e.Name + " ")[0]
			}
			return "no match"
		},
		OnKey: func(c *ui.Context, p *Palette) bool {
			switch {
			case c.Shortcut(0, ui.KeyRight):
				s.chosen = min(s.chosen+1, len(s.filtered)-1)
			case c.Shortcut(0, ui.KeyLeft):
				s.chosen = max(s.chosen-1, 0)
			case c.Shortcut(0, ui.KeyDown):
				s.chosen = min(s.chosen+emojiCols, len(s.filtered)-1)
			case c.Shortcut(0, ui.KeyUp):
				s.chosen = max(s.chosen-emojiCols, 0)
			case c.Shortcut(mod, ui.KeyEnter):
				pick(true)
			default:
				return false
			}
			s.list.ScrollIntoView(s.chosen / emojiCols)
			return true
		},
		Body: func(c *ui.Context, p *Palette) {
			t := design.Tokens(c)
			s.refilter(p.query)
			rows := (len(s.filtered) + emojiCols - 1) / emojiCols
			ui.List(c, &s.list, rows, func(r int) {
				ui.Row(c).Padding(2, 8).Gap(2).Children(func() {
					for i := r * emojiCols; i < min((r+1)*emojiCols, len(s.filtered)); i++ {
						bg := ui.Transparent
						if i == s.chosen {
							bg = t.AccentMuted
						}
						cell := ui.Row(c).Width(52).Height(46).Radius(10).Background(bg).Center().Children(func() {
							ui.Text(c, s.filtered[i].Glyph).FontSize(26)
						})
						if cell.Clicked() {
							s.chosen = i
						}
						if cell.DoubleClicked() {
							pick(false)
						}
					}
				})
			}).Grow(1)
		},
	}
}
