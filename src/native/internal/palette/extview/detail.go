package extview

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// detailPane is the inspector beside the list for an item that has a detail. A nil detail has no pane.
func (s *Session) detailPane(detail *api.Detail) func(c *ui.Context, h search.Host) {
	if detail == nil {
		return nil
	}
	return func(c *ui.Context, _ search.Host) {
		drawDetail(c, *detail, s.markdown, s.images[imageRef(detail.Image)])
	}
}

// drawDetail shows a detail's title, subtitle, body (Markdown first, then plain text) and metadata rows, in that order.
func drawDetail(c *ui.Context, detail api.Detail, md *markdown.Options, image *ui.Bitmap) {
	t := design.Tokens(c)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			if image != nil {
				ui.Image(c, image).Height(120)
			}
			if detail.Title != "" {
				ui.Text(c, detail.Title).FontSize(13).TextColor(t.Strong)
			}
			if detail.Subtitle != "" {
				ui.Text(c, detail.Subtitle).FontSize(11).TextColor(t.Tertiary)
			}
			drawDetailBody(c, detail, md)
			drawFacts(c, detail.Facts)
			if detail.Video != "" {
				ui.Text(c, "Video is not drawn in this build.").FontSize(11).TextColor(t.Tertiary)
			}
			for _, row := range detail.Metadata {
				drawMetadata(c, row)
			}
		})
	})
}

// DetailFor is the detail pane of a row that is not in a view: a root row or a search row of extension extID. Nil when
// the item has no detail.
func DetailFor(deps Deps, extID string, detail *api.Detail) func(c *ui.Context, h search.Host) {
	return NewSession(deps, extID, api.View{}).detailPane(detail)
}

// drawFacts draws the facts of a detail as a column of labels over large values, the layout of a calculator answer.
func drawFacts(c *ui.Context, facts []api.Fact) {
	if len(facts) == 0 {
		return
	}
	t := design.Tokens(c)
	ui.Column(c).Gap(10).Children(func() {
		for _, fact := range facts {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, fact.Label).FontSize(11).TextColor(t.Muted)
				ui.Text(c, fact.Value).FontSize(13).TextColor(t.Strong)
			})
		}
	})
}

func drawDetailBody(c *ui.Context, detail api.Detail, md *markdown.Options) {
	t := design.Tokens(c)
	switch {
	case detail.Markdown != "":
		markdown.View(c, detail.Markdown, md)
	case detail.Text != "":
		ui.Text(c, detail.Text).FontSize(11).TextColor(t.Primary)
	}
}

// drawMetadata draws one labelled row. A separator is a divider; a link keeps its URL as its value.
func drawMetadata(c *ui.Context, row api.Metadata) {
	t := design.Tokens(c)
	if row.Kind == api.MetadataSeparator {
		ui.Divider(c)
		return
	}
	if row.Label != "" {
		ui.Text(c, row.Label).FontSize(11).TextColor(t.Tertiary)
	}
	value := row.Value
	if row.Kind == api.MetadataLink && row.URL != "" && value == "" {
		value = row.URL
	}
	ui.Text(c, value).FontSize(11).TextColor(t.Secondary)
}
