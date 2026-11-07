package design

import "github.com/egoist/mygo/ui"

// MenuRow is one action of an ActionMenu.
type MenuRow struct {
	Title string
	// Key is the action's shortcut, already spelled for the platform. Empty when it has none.
	Key string
	// Enter draws the return glyph in place of a key: the default action of the row.
	Enter    bool
	Selected bool
	// Disabled draws the action muted, with Subtitle beneath it as the reason it cannot run. Choosing it does nothing.
	Disabled bool
	Subtitle string
	// Header draws Title as the header of a menu group, such as View: muted, with no selection, hover or click.
	Header bool
}

// MenuEvents is what the pointer did on an ActionMenu this frame: the row clicked and the row under the pointer, each
// -1 when none.
type MenuEvents struct {
	Clicked   int
	Hovered   int
	Submitted bool
	X, Y      float32
}

// ActionMenu draws the action menu of a row (Ctrl+K) as a popover over the pane. Each action is a row; the selected one
// sits on the selection plate, a row under the pointer takes the wash of a list row, and the shortcut or return glyph sits
// at the end. It reports the row the pointer clicked or rests on, so the palette can run or choose it.
func ActionMenu(c *ui.Context, rows []MenuRow) MenuEvents {
	return actionMenu(c, func() []MenuRow { return rows }, nil)
}

// MenuSearch owns the search and scroll state of a searchable action popover.
type MenuSearch struct {
	Query     *string
	Input     *ui.Handle
	Scroll    *ui.ScrollState
	Reveal    *bool
	MaxHeight float32
	Changed   func()
	OnKey     func(ui.TextKey) bool
}

// SearchableActionMenu builds rows after the field changes, so filtering, selection and Enter share one snapshot.
func SearchableActionMenu(c *ui.Context, search MenuSearch, rows func() []MenuRow) MenuEvents {
	return actionMenu(c, rows, &search)
}

func actionMenu(c *ui.Context, rowsFor func() []MenuRow, search *MenuSearch) MenuEvents {
	k := Tokens(c)
	events := MenuEvents{Clicked: -1, Hovered: -1}
	Popover(c, ActionPlacement, func() {
		if search != nil {
			ui.Row(c).FillWidth().Padding(8, 10).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, Icon("search")).FontSize(13).TextColor(k.Muted)
				input := ui.TextInputBase(c, search.Query).Bind(search.Input).Grow(1).FontSize(13).TextColor(k.Strong).
					Placeholder("Search actions…").PlaceholderColor(k.Muted).Label("Search actions").OnTextKey(search.OnKey)
				if input.Changed() && search.Changed != nil {
					search.Changed()
				}
				events.Submitted = input.Submitted()
			})
			Etched(c)
		}
		rows := rowsFor()
		drawRows := func() {
			if len(rows) == 0 {
				ui.Text(c, "No matching actions").FontSize(12).TextColor(k.Muted).Padding(10)
			}
			for i, r := range rows {
				if r.Header {
					PlatePadding(ui.Row(c).AlignItems(ui.Center), false, 6, 10).Children(func() {
						ui.Text(c, r.Title).FontSize(11).FontWeight(600).TextColor(k.Muted).SingleLine()
					})
					continue
				}
				rowEl := PlatePadding(ui.Row(c).Radius(8).AlignItems(ui.Center).Gap(8), r.Selected, 6, 10)
				if search != nil && search.Reveal != nil && *search.Reveal && r.Selected {
					rowEl.ScrollIntoView()
				}
				hovered := rowEl.Hovered()
				switch {
				case r.Selected:
					SelectedFill(rowEl, k, false)
				case hovered:
					rowEl.Background(k.Wash)
				}
				if hovered {
					events.Hovered = i
					x, y, _ := rowEl.PointerPosition()
					bounds := rowEl.Bounds()
					events.X, events.Y = bounds.X+x, bounds.Y+y
				}
				rowEl.Children(func() {
					if r.Subtitle != "" {
						describedMenuContent(c, k, r)
						return
					}
					ui.Text(c, r.Title).FontSize(13).TextColor(actionTitleColor(k, r)).Grow(1).SingleLine()
					switch {
					case r.Key != "":
						ui.Text(c, r.Key).FontSize(11).FontWeight(500).TextColor(k.Muted).SingleLine()
					case r.Enter:
						ui.Icon(c, Icon("corner-down-left")).FontSize(12).TextColor(k.Muted)
					}
				})
				if rowEl.Clicked() {
					events.Clicked = i
				}
			}
		}
		if search == nil {
			drawRows()
		} else {
			ui.Scroll(c).FillWidth().MaxHeight(search.MaxHeight).MinHeight(0).Scrollbars(false).TrackScroll(search.Scroll).Children(drawRows)
		}
	})
	return events
}

// describedMenuContent keeps descriptive or disabled text beneath the action title.
func describedMenuContent(c *ui.Context, k *TokenSet, r MenuRow) {
	ui.Column(c).Grow(1).Gap(2).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, r.Title).FontSize(13).TextColor(actionTitleColor(k, r)).Grow(1).SingleLine()
			if r.Key != "" {
				ui.Text(c, r.Key).FontSize(11).TextColor(k.Muted)
			} else if r.Enter {
				ui.Icon(c, Icon("corner-down-left")).FontSize(12).TextColor(k.Muted)
			}
		})
		ui.Text(c, r.Subtitle).FontSize(11).TextColor(k.Muted).SingleLine()
	})
}

// actionTitleColor is the title color of a menu row: strong for an action that runs, muted for one that is disabled.
func actionTitleColor(k *TokenSet, r MenuRow) ui.Color {
	if r.Disabled {
		return k.Muted
	}
	return k.Strong
}
