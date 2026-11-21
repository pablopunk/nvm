package extview

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// detailPane is the inspector beside the list for an item that has a detail. A nil detail has no pane. In a side-preview
// list the metadata is pinned to the pane's foot, as 1.x draws it.
func (s *Session) detailPane(detail *api.Detail) func(c *ui.Context, h search.Host) {
	if detail == nil {
		return nil
	}
	pinned := s.view.Presentation == api.PresentSidePreview
	return func(c *ui.Context, _ search.Host) {
		if detail.File != nil {
			s.drawFileDetail(c, *detail.File)
			return
		}
		s.drawDetail(c, *detail, s.images[imageRef(detail.Image)], pinned)
	}
}

// drawDetail shows a detail's title, subtitle, body (Markdown first, then plain text), facts, link card and video note,
// then its metadata rows. A pinned pane keeps the metadata below the scrolling body.
func (s *Session) drawDetail(c *ui.Context, detail api.Detail, image *ui.Bitmap, pinned bool) {
	ui.Scroll(c).Grow(1).Children(func() {
		s.drawDetailBody(c, detail, image, !pinned)
	})
	if pinned {
		drawMetadataFooter(c, detail.Metadata)
	}
}

// drawDetailBody is the scrolling part of a detail pane. Its metadata rows follow the body unless they are pinned.
func (s *Session) drawDetailBody(c *ui.Context, detail api.Detail, image *ui.Bitmap, withMetadata bool) {
	t := design.Tokens(c)
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
		drawDetailText(c, detail, s.markdown)
		drawFacts(c, detail.Facts)
		if detail.Link != nil {
			s.drawLinkCard(c, *detail.Link)
		}
		if detail.Video != "" {
			ui.Text(c, "Video is not drawn in this build.").FontSize(11).TextColor(t.Tertiary)
		}
		if withMetadata {
			drawMetadataRows(c, detail.Metadata)
		}
	})
}

// drawMetadataFooter is the metadata of a pinned pane: a rule and the rows, under the scrolling body.
func drawMetadataFooter(c *ui.Context, rows []api.Metadata) {
	if len(rows) == 0 {
		return
	}
	ui.Column(c).Gap(8).Padding(8, 0, 0, 0).Children(func() {
		drawMetadataRows(c, rows)
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

// drawDetailText draws the body of a detail: its Markdown when it has any, else its plain text at the palette's body size.
func drawDetailText(c *ui.Context, detail api.Detail, md *markdown.Options) {
	t := design.Tokens(c)
	switch {
	case detail.Markdown != "":
		markdown.View(c, detail.Markdown, md)
	case detail.Text != "":
		ui.Text(c, detail.Text).FontSize(13).TextColor(t.Primary)
	}
}

// drawMetadataRows draws the metadata rows in order.
func drawMetadataRows(c *ui.Context, rows []api.Metadata) {
	for _, row := range rows {
		drawMetadata(c, row)
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
	ui.Text(c, value).FontSize(12).TextColor(t.Secondary)
}
