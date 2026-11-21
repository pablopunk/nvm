package extview

import (
	"net/url"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Link card statuses. The host sets them; off means the user turned link previews off, and the card is not drawn.
const (
	linkLoading     = "loading"
	linkReady       = "ready"
	linkUnavailable = "unavailable"
	linkOff         = "off"
)

// Geometry and type sizes of the link card.
const (
	linkCardRadius   = 12
	linkCardPadding  = 10
	linkCardInfoPt   = 11
	linkCardTitlePt  = 13
	linkCardTextPt   = 12
	linkCardImageFit = ui.Contain
)

// Words the card shows while the preview is not there yet, or will not come.
const (
	linkTitleLoading     = "Loading preview…"
	linkTitleFailed      = "Preview unavailable"
	linkTitleNone        = "No title on this page"
	linkDescriptionFails = "This page could not be previewed."
)

// drawLinkCard draws the preview of a web address: its picture, site and title, and a description. It shows the
// address's host at once and fills in as the host reads the page.
func (s *Session) drawLinkCard(c *ui.Context, link api.LinkCard) {
	card := s.linkStatus(link)
	if card.Status == linkOff {
		return
	}
	t := design.Tokens(c)
	picture := s.bitmap(&api.Image{Src: card.Image})
	ui.Column(c).Padding(linkCardPadding).Radius(linkCardRadius).Background(t.Raised).Border(1, t.EdgeSubtle).Children(func() {
		if picture != nil {
			ui.Image(c, picture).Fill().Fit(linkCardImageFit)
		}
		ui.Text(c, linkSite(card)).FontSize(linkCardInfoPt).TextColor(t.Tertiary)
		ui.Text(c, linkTitle(card)).FontSize(linkCardTitlePt).TextColor(t.Strong)
		if description := linkDescription(card); description != "" {
			ui.Text(c, description).FontSize(linkCardTextPt).TextColor(t.Secondary)
		}
	})
}

// linkStatus asks the host for the current state of a card's address. Without a host reader the card is off.
func (s *Session) linkStatus(link api.LinkCard) api.LinkCard {
	if s.deps.LinkCard == nil {
		return api.LinkCard{URL: link.URL, Status: linkOff}
	}
	return s.deps.LinkCard(link.URL)
}

// linkSite names the site of a card: the site name the page gave, or the host of the address without "www.".
func linkSite(card api.LinkCard) string {
	if card.SiteName != "" {
		return card.SiteName
	}
	parsed, err := url.Parse(card.URL)
	if err != nil {
		return card.URL
	}
	return trimWWW(parsed.Hostname())
}

// trimWWW drops a leading "www." from a host name.
func trimWWW(host string) string {
	const prefix = "www."
	if len(host) > len(prefix) && host[:len(prefix)] == prefix {
		return host[len(prefix):]
	}
	return host
}

// linkTitle is the headline of a card in each state.
func linkTitle(card api.LinkCard) string {
	switch card.Status {
	case linkReady:
		if card.Title != "" {
			return card.Title
		}
		return linkTitleNone
	case linkUnavailable:
		return linkTitleFailed
	}
	return linkTitleLoading
}

// linkDescription is the second line of a card, when it has one.
func linkDescription(card api.LinkCard) string {
	switch card.Status {
	case linkReady:
		return card.Description
	case linkUnavailable:
		return linkDescriptionFails
	}
	return ""
}
