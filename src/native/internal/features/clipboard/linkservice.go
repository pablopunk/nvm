package clipboard

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/safego"
)

// LinkCacheLimit is how many link previews stay in memory; the least recently stored go first.
const LinkCacheLimit = 200

// linkLoadName names the background fetch for panic reporting.
const linkLoadName = "clipboard.link-preview"

// LinkState is where a link preview stands.
type LinkState string

// Link preview states. Loading is the placeholder shown before the first fetch settles.
const (
	LinkLoading     LinkState = "loading"
	LinkReady       LinkState = "ready"
	LinkUnavailable LinkState = "unavailable"
)

// LinkStatus is one link preview as the palette shows it. Image is a data URL the host downloaded and
// downsized; a remote address never reaches the renderer.
type LinkStatus struct {
	URL         string
	State       LinkState
	Title       string
	Description string
	SiteName    string
	Image       string
}

// Page is an HTML document fetched for a link preview. FinalURL is the address after redirects.
type Page struct {
	HTML     string
	FinalURL string
}

// LinkFetcher is the network port of link previews. The app's implementation enforces public addresses,
// timeouts and size limits; this package only calls it. FetchHTML reports ok=false for a page that is not
// HTML, and FetchImage returns a small self-contained data URL.
type LinkFetcher interface {
	FetchHTML(url string) (page Page, ok bool, err error)
	FetchImage(url string) (dataURL string, err error)
}

// LinkService serves link previews without blocking. Peek never touches the network; Load starts one
// background fetch per URL and reports each settlement through the callback, so an open view refreshes
// in place. A failure is remembered as unavailable and never retried.
type LinkService struct {
	fetcher   LinkFetcher
	onSettled func(url string)
	limit     int

	mu       sync.Mutex
	cache    map[string]LinkStatus
	order    []string
	inflight map[string]bool
}

// NewLinkService returns a service over fetcher. onSettled may be nil.
func NewLinkService(fetcher LinkFetcher, onSettled func(url string)) *LinkService {
	return &LinkService{
		fetcher:   fetcher,
		onSettled: onSettled,
		limit:     LinkCacheLimit,
		cache:     map[string]LinkStatus{},
		inflight:  map[string]bool{},
	}
}

// Peek returns the cached preview of url, or a loading placeholder when none is cached.
func (s *LinkService) Peek(url string) LinkStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if status, ok := s.cache[url]; ok {
		return status
	}
	return LinkStatus{URL: url, State: LinkLoading}
}

// Load starts the background fetch of url unless the preview is cached or already loading.
func (s *LinkService) Load(url string) {
	s.mu.Lock()
	if _, cached := s.cache[url]; cached || s.inflight[url] {
		s.mu.Unlock()
		return
	}
	s.inflight[url] = true
	s.mu.Unlock()
	safego.Go(linkLoadName, func(context.Context) {
		s.fetchAndRemember(url)
	}, safego.WithReporter(silentReporter{}))
}

// fetchAndRemember fetches the page, stores its metadata without the image, then fetches the image and stores
// it. Each stored state is announced.
func (s *LinkService) fetchAndRemember(url string) {
	defer s.clearInflight(url)
	status, remoteImage := s.fetchMetadata(url)
	s.remember(url, status)
	s.announce(url)
	if remoteImage == "" {
		return
	}
	dataURL, err := s.fetcher.FetchImage(remoteImage)
	if err != nil || dataURL == "" {
		return
	}
	status.Image = dataURL
	s.remember(url, status)
	s.announce(url)
}

// fetchMetadata returns the ready status of url, or an unavailable one, and the remote image address to
// fetch next. A fetch error or a page without metadata is unavailable.
func (s *LinkService) fetchMetadata(url string) (LinkStatus, string) {
	page, ok, err := s.fetcher.FetchHTML(url)
	if err != nil || !ok {
		return LinkStatus{URL: url, State: LinkUnavailable}, ""
	}
	meta := ParseLinkMetadata(page.HTML, page.FinalURL)
	if meta.Title == "" && meta.Description == "" && meta.ImageURL == "" {
		return LinkStatus{URL: url, State: LinkUnavailable}, ""
	}
	return LinkStatus{
		URL:         url,
		State:       LinkReady,
		Title:       meta.Title,
		Description: meta.Description,
		SiteName:    meta.SiteName,
	}, meta.ImageURL
}

// remember stores status as the newest cache entry and evicts the oldest beyond the limit.
func (s *LinkService) remember(url string, status LinkStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.order = removeString(s.order, url)
	s.order = append(s.order, url)
	s.cache[url] = status
	for len(s.order) > s.limit {
		delete(s.cache, s.order[0])
		s.order = s.order[1:]
	}
}

// clearInflight forgets that url is being fetched.
func (s *LinkService) clearInflight(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inflight, url)
}

// announce tells the callback that url changed.
func (s *LinkService) announce(url string) {
	if s.onSettled != nil {
		s.onSettled(url)
	}
}

// removeString returns list without the first occurrence of value.
func removeString(list []string, value string) []string {
	for i, item := range list {
		if item == value {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}
