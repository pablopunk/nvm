package app

import (
	"strings"

	settingsfeat "github.com/pablopunk/nvm/native/internal/features/settings"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// guideKeywords are the words that find the guide's root row besides its title.
const guideKeywords = "help getting started welcome how to use commands"

// guideRootRows is the Welcome row of the root until the user dismisses it, and the Help row after that. An empty
// query shows it at its entry score; a query shows it when the query is part of its title or one of its words.
func guideRootRows(p *App, _ *rootInput, q string) []search.Item {
	title, subtitle, score := settingsfeat.GuideEntry(p.S.GuideDone)
	lq := strings.ToLower(q)
	if lq != "" {
		if !matchesGuide(title, lq) {
			return nil
		}
		score = scoreCommandAlias
	}
	item := search.Item{Kind: search.KindCommands, Title: title, Subtitle: subtitle, Icon: "sparkles", Score: score,
		LearnKey: "guide", Actions: []search.Action{{Title: guideOpenTitle(p.S.GuideDone), Run: func(h search.Host) { h.Push(p.guideView()) }}}}
	if !p.S.GuideDone {
		item.Actions = append(item.Actions, search.Action{Title: "Dismiss welcome", Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.completeGuide()
			h.Refresh()
		}})
	}
	return []search.Item{item}
}

// matchesGuide reports whether a lowercase query names the guide: it is part of the row's title, or of its keywords
// when it has two characters or more, as command aliases match.
func matchesGuide(title, lq string) bool {
	if strings.Contains(strings.ToLower(title), lq) {
		return true
	}
	return len(lq) >= 2 && strings.Contains(guideKeywords, lq)
}

// guideOpenTitle names the action that opens the guide.
func guideOpenTitle(done bool) string {
	if done {
		return "Open Help"
	}
	return "Get Started"
}

// completeGuide records that the user is done with the guide, so the root shows Help from now on.
func (p *App) completeGuide() {
	if p.S.GuideDone {
		return
	}
	p.S.GuideDone = true
	p.S.save()
}

// guideView shows the guide's items. The shortcut item opens the hotkey editor; Done for now completes the guide
// and returns to the root.
func (p *App) guideView() *palette.View {
	return &palette.View{
		Title: "Getting Started", Placeholder: "Search tips",
		Search: func(string) []search.Item {
			rows := settingsfeat.GuideRows(p.guideShortcutLabel())
			items := make([]search.Item, 0, len(rows))
			for _, row := range rows {
				items = append(items, p.guideItem(row))
			}
			return items
		},
	}
}

// guideShortcutLabel is the palette shortcut the guide's shortcut row names, as the user sees it, or "" when none is
// bound. It reads the live binding: the shortcut the user changed in the recorder is the one the guide shows.
func (p *App) guideShortcutLabel() string {
	return shortcutLabelOf(p.shortcutsManager().PaletteAccelerator())
}

// guideItem maps one guide row onto a palette row. Information rows have no action.
func (p *App) guideItem(row settingsfeat.GuideRow) search.Item {
	item := search.Item{ID: row.ID, Title: row.Title, Subtitle: row.Subtitle, Icon: row.Icon}
	switch row.Action {
	case settingsfeat.GuideSetShortcut:
		item.Actions = []search.Action{{Title: "Set Shortcut", Run: func(h search.Host) { h.Push(p.paletteRecorderView()) }}}
	case settingsfeat.GuideFinish:
		item.Actions = []search.Action{{Title: "Done for now", Run: func(h search.Host) {
			p.completeGuide()
			h.Pop()
		}}}
	}
	return item
}
