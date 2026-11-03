package app

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
)

// Geometry of the link card. The card is drawn with the design tokens; only its size and corner are literals.
const (
	linkCardRadius    = 12
	linkCardImageMax  = 160
	linkCardPadding   = 10
	clipDetailTextPt  = 13
	clipDetailInfoPt  = 11
	clipDetailCardPt  = 12
	clipDetailTitlePt = 13
)

// Words the detail pane shows while a picture or a preview is not there yet, or will not come.
const (
	clipImageLoading     = "Loading image…"
	clipImageFailed      = "Image unavailable"
	clipPreviewLoading   = "Loading preview…"
	clipPreviewFailed    = "Preview unavailable"
	clipPreviewNoTitle   = "No title on this page"
	clipPreviewFailedSub = "This page could not be previewed."
)

// clipDetail draws the side preview of an entry: its content, the link card of a single web address, and a line
// that says when it was copied and how big it is.
func (p *App) clipDetail(c *ui.Context, e clipboard.Entry) {
	t := design.Tokens(c)
	switch e.Kind {
	case clipboard.KindImage:
		p.clipImageDetail(c, e)
	default:
		link := clipboard.SingleWebURL(e.Text)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Text(c, e.Text).FontSize(clipDetailTextPt).TextColor(t.Primary)
			if link != "" {
				p.clipLinkCard(c, link)
			}
		})
	}
	ui.Divider(c)
	ui.Text(c, clipDetailInfo(e)).FontSize(clipDetailInfoPt).TextColor(t.Tertiary)
}

// clipImageDetail draws a picture aspect-fitted in the pane, or the state it is in.
func (p *App) clipImageDetail(c *ui.Context, e clipboard.Entry) {
	bitmap, state := p.clip.pics.bitmap(e.ImageRef)
	if bitmap != nil {
		ui.Image(c, bitmap).Fill().Fit(ui.Contain)
		return
	}
	t := design.Tokens(c)
	message := clipImageFailed
	if state == picLoading {
		message = clipImageLoading
	}
	ui.Text(c, message).FontSize(clipDetailTextPt).TextColor(t.Tertiary)
}

// clipLinkCard draws the preview of a web address. It shows the host at once and fills in as the page settles.
// A loading card asks for the preview; the request is idempotent, so drawing again costs nothing.
func (p *App) clipLinkCard(c *ui.Context, rawURL string) {
	if !p.clip.linksOn {
		return
	}
	status := p.clip.links.Peek(rawURL)
	if status.State == clipboard.LinkLoading {
		p.clip.links.Load(rawURL)
	}
	t := design.Tokens(c)
	bitmap, _ := p.clip.pics.bitmap(status.ImageRef)
	site := linkSite(rawURL, status.SiteName)
	ui.Column(c).Padding(linkCardPadding).Radius(linkCardRadius).Background(t.Raised).Border(1, t.EdgeSubtle).Children(func() {
		if bitmap != nil {
			ui.Image(c, bitmap).Fill().Fit(ui.Contain)
		}
		ui.Text(c, site).FontSize(clipDetailInfoPt).TextColor(t.Tertiary)
		ui.Text(c, linkTitle(status)).FontSize(clipDetailTitlePt).TextColor(t.Strong)
		if description := linkDescription(status); description != "" {
			ui.Text(c, description).FontSize(clipDetailCardPt).TextColor(t.Secondary)
		}
	})
}

// linkSite names the site of a preview: the page's site name, or the host of the address.
func linkSite(rawURL, siteName string) string {
	if siteName != "" {
		return siteName
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

// linkTitle is the headline of a preview in each state.
func linkTitle(status clipboard.LinkStatus) string {
	switch status.State {
	case clipboard.LinkReady:
		if status.Title != "" {
			return status.Title
		}
		return clipPreviewNoTitle
	case clipboard.LinkUnavailable:
		return clipPreviewFailed
	}
	return clipPreviewLoading
}

// linkDescription is the second line of a preview, when it has one.
func linkDescription(status clipboard.LinkStatus) string {
	switch status.State {
	case clipboard.LinkReady:
		return status.Description
	case clipboard.LinkUnavailable:
		return clipPreviewFailedSub
	}
	return ""
}

// clipDetailInfo is the footer line of the pane: when the entry was copied, and its size.
func clipDetailInfo(e clipboard.Entry) string {
	info := fmt.Sprintf("%s · %s", ago(e.CreatedAt), e.CreatedAt.Format("Jan 2 15:04"))
	switch e.Kind {
	case clipboard.KindImage:
		return info + fmt.Sprintf(" · %d×%d", e.Width, e.Height)
	case clipboard.KindFiles:
		return info + fmt.Sprintf(" · %d files", len(e.Paths))
	}
	return info + fmt.Sprintf(" · %d chars", len([]rune(e.Text)))
}
