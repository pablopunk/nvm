package clipboard

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
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

// LinkStatus is one link preview as the palette shows it. ImageRef names a picture in the image store; the
// host decodes it, so a remote address never reaches the renderer.
type LinkStatus struct {
	URL         string
	State       LinkState
	Title       string
	Description string
	SiteName    string
	ImageRef    string
}

// Page is an HTML document fetched for a link preview. FinalURL is the address after redirects.
type Page struct {
	HTML     string
	FinalURL string
}

// LinkFetcher is the network port of link previews. The app's implementation (LinkFetcher in linkfetch.go)
// enforces public addresses, timeouts and size limits; this package only calls it. FetchHTML reports ok=false
// for a page that is not HTML. FetchImage returns a PNG the host has decoded and downsized.
type LinkFetcher interface {
	FetchHTML(url string) (page Page, ok bool, err error)
	FetchImage(url string) (png []byte, err error)
}

// LinkImages saves the pictures of link previews. The image store implements it.
type LinkImages interface {
	Save(hash string, png []byte) (string, error)
}

// LinkStore keeps settled previews with the history, so a preview is fetched once and survives a restart. The
// history implements it.
type LinkStore interface {
	PreviewFor(url string) (LinkPreview, bool)
	SetPreview(url string, preview LinkPreview) error
}

// LinkDeps are the collaborators of a LinkService. Clock is required. Store and Images may be nil, which keeps
// previews in memory for the session only. OnSettled may be nil.
type LinkDeps struct {
	Fetcher   LinkFetcher
	Images    LinkImages
	Store     LinkStore
	Clock     platform.Clock
	OnSettled func(url string)
}

// LinkService serves link previews without blocking. Peek never touches the network; Load starts one
// background fetch per URL and reports each settlement through OnSettled, so an open view refreshes in place.
// The refetch rule is the 1.x one (electron/clipboard/link-preview.ts): a URL is fetched once per
// session, a settled preview (ready or unavailable) is never fetched again, and a failure is never retried.
// A ready preview is also kept with its history entry and is not fetched again after a restart; FetchedAt
// records when it was fetched.
type LinkService struct {
	deps  LinkDeps
	limit int

	mu       sync.Mutex
	cache    map[string]LinkStatus
	order    []string
	inflight map[string]bool
}

// NewLinkService returns a service over deps.
func NewLinkService(deps LinkDeps) *LinkService {
	return &LinkService{
		deps:     deps,
		limit:    LinkCacheLimit,
		cache:    map[string]LinkStatus{},
		inflight: map[string]bool{},
	}
}

// Peek returns the preview of url: the cached one, else the one saved with the history, else a loading
// placeholder. It never starts a fetch.
func (s *LinkService) Peek(url string) LinkStatus {
	s.mu.Lock()
	if status, ok := s.cache[url]; ok {
		s.mu.Unlock()
		return status
	}
	s.mu.Unlock()
	if status, ok := s.saved(url); ok {
		return status
	}
	return LinkStatus{URL: url, State: LinkLoading}
}

// Load starts the background fetch of url unless its preview is known or already loading.
func (s *LinkService) Load(url string) {
	if s.Peek(url).State != LinkLoading {
		return
	}
	s.mu.Lock()
	if s.inflight[url] {
		s.mu.Unlock()
		return
	}
	s.inflight[url] = true
	s.mu.Unlock()
	safego.Go(linkLoadName, func(context.Context) {
		s.fetchAndRemember(url)
	}, safego.WithReporter(silentReporter{}))
}

// saved returns the ready preview kept with the history for url, and caches it for the next Peek.
func (s *LinkService) saved(url string) (LinkStatus, bool) {
	if s.deps.Store == nil {
		return LinkStatus{}, false
	}
	preview, ok := s.deps.Store.PreviewFor(url)
	if !ok {
		return LinkStatus{}, false
	}
	status := LinkStatus{
		URL: url, State: LinkReady, Title: preview.Title, Description: preview.Description,
		SiteName: preview.SiteName, ImageRef: preview.ImageRef,
	}
	s.remember(url, status)
	return status, true
}

// fetchAndRemember fetches the page and stores its metadata, announcing the first settlement. Then it fetches
// the preview image, stores it and announces again. A ready preview is saved with the history before the
// announcement that completes it, so whoever reacts to a settlement reads what was saved.
func (s *LinkService) fetchAndRemember(url string) {
	defer s.clearInflight(url)
	status, remoteImage := s.fetchMetadata(url)
	s.remember(url, status)
	if status.State != LinkReady {
		s.announce(url)
		return
	}
	if remoteImage == "" {
		s.persist(status)
		s.announce(url)
		return
	}
	s.announce(url)
	status.ImageRef = s.fetchImage(remoteImage)
	s.persist(status)
	if status.ImageRef != "" {
		s.remember(url, status)
		s.announce(url)
	}
}

// fetchImage fetches and saves a preview image. It returns the stored name, or "" when the image is absent.
func (s *LinkService) fetchImage(remoteImage string) string {
	if s.deps.Fetcher == nil || s.deps.Images == nil {
		return ""
	}
	png, err := s.deps.Fetcher.FetchImage(remoteImage)
	if err != nil || len(png) == 0 {
		return ""
	}
	name, err := s.deps.Images.Save(hashHex(png), png)
	if err != nil {
		return ""
	}
	return name
}

// persist saves a ready preview with the history, with the time it was fetched.
func (s *LinkService) persist(status LinkStatus) {
	if s.deps.Store == nil {
		return
	}
	_ = s.deps.Store.SetPreview(status.URL, LinkPreview{
		URL: status.URL, Title: status.Title, Description: status.Description, SiteName: status.SiteName,
		ImageRef: status.ImageRef, FetchedAt: s.now(),
	})
}

// now reads the clock port.
func (s *LinkService) now() time.Time {
	return s.deps.Clock.Now()
}

// fetchMetadata returns the ready status of url, or an unavailable one, and the remote image address to
// fetch next. A fetch error or a page without metadata is unavailable.
func (s *LinkService) fetchMetadata(url string) (LinkStatus, string) {
	if s.deps.Fetcher == nil {
		return LinkStatus{URL: url, State: LinkUnavailable}, ""
	}
	page, ok, err := s.deps.Fetcher.FetchHTML(url)
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
	if s.deps.OnSettled != nil {
		s.deps.OnSettled(url)
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
