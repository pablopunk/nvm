package extview

import (
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"testing"
)

// The same host session renders cold, cached-refresh and settled grids in both appearances.
func TestGridAndLoadingGoldens(t *testing.T) {
	for _, layout := range []string{"list", "grid", "compact"} {
		for _, state := range []string{"cold", "warm", "settled"} {
			for _, width := range []int{420, 720} {
				name := layout + "-" + state
				if width == 420 {
					name += "-narrow"
				}
				t.Run(name, func(t *testing.T) {
					view := api.View{Kind: api.ViewList, Title: "Repositories", Placeholder: "Filter repositories", IsLoading: state != "settled"}
					if layout != "list" {
						view.Kind = api.ViewGrid
						view.Grid = &api.GridSpec{Columns: 4, Layout: layout}
					}
					if state != "cold" {
						for _, title := range []string{"Nevermind", "An unusually long repository name that must be clipped", "MyGo", "Palette", "Fixtures", "Design", "Window", "Search"} {
							glyph := "◆"
							if layout == "compact" {
								glyph = "😀"
							}
							view.Items = append(view.Items, api.Item{ID: title, Title: title, Subtitle: "pablopunk / Go", Icon: "folder", Glyph: glyph})
						}
					}
					s := NewSession(Deps{}, "fixture.loading", view)
					p := palette.New(palette.Options{Root: &palette.View{Title: "Nevermind"}, Synchronous: true})
					p.Push(s.Palette())
					p.SetQuery("")
					p.Choose(2)
					golden.AssertMatrix(t, name, p.View, golden.Options{Group: "loading-grid", Width: width, Height: 470})
				})
			}
		}
	}
}
