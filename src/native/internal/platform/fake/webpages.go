package fake

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// WebPages is a platform.WebPages that records the pages opened and lets a test play the page's side: a
// permission request, a navigation, a window open and a title change. Available false makes Open return
// platform.ErrUnsupported, as on a system without a web view.
type WebPages struct {
	mu        sync.Mutex
	Available bool
	pages     []*WebPage
}

var _ platform.WebPages = (*WebPages)(nil)

// NewWebPages returns a web view system that opens pages.
func NewWebPages() *WebPages { return &WebPages{Available: true} }

// Open records a page built from spec, or reports ErrUnsupported when the system has no web view.
func (w *WebPages) Open(spec platform.WebPageSpec) (platform.WebPage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.Available {
		return nil, platform.ErrUnsupported
	}
	page := &WebPage{spec: spec}
	w.pages = append(w.pages, page)
	return page, nil
}

// Pages returns every page opened so far, oldest first.
func (w *WebPages) Pages() []*WebPage {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]*WebPage(nil), w.pages...)
}

// WebPage is one page of a WebPages. It keeps what the view loaded and ran, and whether it was destroyed.
type WebPage struct {
	spec      platform.WebPageSpec
	mu        sync.Mutex
	html      []string
	scripts   []string
	destroyed bool
}

var _ platform.WebPage = (*WebPage)(nil)

// Native returns nil: a fake page has no system view, so a palette draws its placeholder.
func (p *WebPage) Native() any { return nil }

// Load records the html the view loaded.
func (p *WebPage) Load(html string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.html = append(p.html, html)
}

// Run records a script the view ran.
func (p *WebPage) Run(script string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scripts = append(p.scripts, script)
}

// Destroy marks the page destroyed.
func (p *WebPage) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.destroyed = true
}

// Loaded returns the html loaded so far, oldest first.
func (p *WebPage) Loaded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.html...)
}

// Scripts returns the scripts run so far, oldest first.
func (p *WebPage) Scripts() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.scripts...)
}

// Destroyed reports whether Destroy was called.
func (p *WebPage) Destroyed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.destroyed
}

// Requests asks the page for permissions the way the system would, and returns the spec's answer.
func (p *WebPage) Requests(kinds ...string) bool {
	return p.spec.Grant != nil && p.spec.Grant(kinds)
}

// Navigate asks the page to load url in itself. A refused URL is handed to OpenExternal, as the host does.
func (p *WebPage) Navigate(url string) bool {
	if p.spec.Navigate != nil && p.spec.Navigate(url) {
		return true
	}
	if p.spec.OpenExternal != nil {
		p.spec.OpenExternal(url)
	}
	return false
}

// SetTitle plays a title change of the page's document.
func (p *WebPage) SetTitle(title string) {
	if p.spec.Title != nil {
		p.spec.Title(title)
	}
}
