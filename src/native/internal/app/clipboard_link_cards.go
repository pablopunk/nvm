package app

import (
	"net/url"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
)

// linkCardOf is the state of a link card for the detail panes: the page's title, description, site and picture, read by
// the link service. It starts the read when there is none, and says off while the user has link previews turned off.
func (p *App) linkCardOf(rawURL string) api.LinkCard {
	card := api.LinkCard{URL: rawURL, Status: "off"}
	if p.clip == nil || !p.clip.linksOn {
		return card
	}
	status := p.clip.links.Peek(rawURL)
	if status.State == clipboard.LinkLoading {
		p.clip.links.Load(rawURL)
	}
	card.Status = linkStatusName(status.State)
	card.Title, card.Description, card.SiteName = status.Title, status.Description, status.SiteName
	if path, err := p.clip.images.FilePath(status.ImageRef); err == nil && status.ImageRef != "" {
		card.Image = (&url.URL{Scheme: "file", Path: path}).String()
	}
	return card
}

// linkStatusName is the status word of a link state, as the card carries it.
func linkStatusName(state clipboard.LinkState) string {
	switch state {
	case clipboard.LinkReady:
		return "ready"
	case clipboard.LinkUnavailable:
		return "unavailable"
	}
	return "loading"
}
