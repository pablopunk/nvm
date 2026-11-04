package real

import (
	"errors"
	"sync"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// errNoWindow is returned while the app has no window to put a page in yet.
var errNoWindow = errors.New("web pages need the palette window, which is not open yet")

// scriptQueueSize bounds the scripts waiting for a page. Views send a few commands a second at most, so a full
// queue means the page stopped answering; the script is then dropped rather than blocking the UI thread.
const scriptQueueSize = 256

// WebWindow is the part of a MyGo window a page needs: making a web view in it.
type WebWindow interface {
	NewWebView(opts mygo.WebViewOptions) (*mygo.WebView, error)
}

// webPages opens untrusted pages in the window that window returns.
type webPages struct {
	window func() WebWindow
}

// NewWebPages returns the MyGo web pages of the palette window. window is called on each Open, so the window may
// be created after the views are wired.
func NewWebPages(window func() WebWindow) platform.WebPages {
	return webPages{window: window}
}

// Open makes an untrusted web view in the window, filled with the palette surface until the page paints, and wires
// the spec's decisions to its page. The page starts blank; the view loads its document with Load.
func (w webPages) Open(spec platform.WebPageSpec) (platform.WebPage, error) {
	if !mygo.WebViewAvailable() {
		return nil, platform.ErrUnsupported
	}
	win := w.window()
	if win == nil {
		return nil, errNoWindow
	}
	view, err := win.NewWebView(mygo.WebViewOptions{
		Page:            mygo.PageOptions{Untrusted: true},
		BackgroundColor: spec.Background,
	})
	if err != nil {
		return nil, err
	}
	page := &webPage{view: view, scripts: make(chan string, scriptQueueSize)}
	installPageDecisions(view.Page(), spec)
	go page.runScripts()
	return page, nil
}

// installPageDecisions routes the page's permission requests, navigations, window opens and title changes to the
// spec. Each runs on the UI thread and must not block.
func installPageDecisions(p *mygo.Page, spec platform.WebPageSpec) {
	p.SetPermissionHandler(func(req mygo.PermissionRequest) bool {
		kinds := make([]string, 0, len(req.Permissions))
		for _, kind := range req.Permissions {
			kinds = append(kinds, string(kind))
		}
		return spec.Grant != nil && spec.Grant(kinds)
	})
	p.OnWillNavigate(func(e *mygo.NavigateEvent) {
		if spec.Navigate != nil && spec.Navigate(e.URL) {
			return
		}
		e.PreventDefault()
		if spec.OpenExternal != nil {
			spec.OpenExternal(e.URL)
		}
	})
	p.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		if spec.OpenExternal != nil {
			spec.OpenExternal(req.URL)
		}
		return nil
	})
	p.OnPageTitleUpdated(func(e *mygo.TitleEvent) {
		if spec.Title != nil {
			spec.Title(e.Title)
		}
	})
}

// webPage is a page of a MyGo web view. Scripts run on one goroutine, in the order they were queued, so the page
// never answers a command out of order and Eval never blocks the UI thread.
type webPage struct {
	view    *mygo.WebView
	scripts chan string

	mu        sync.Mutex
	destroyed bool
}

var _ platform.WebPage = (*webPage)(nil)

// Native returns the web view for a palette element to place, or nil once destroyed.
func (p *webPage) Native() any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.destroyed {
		return nil
	}
	return p.view
}

// Load replaces the document with html. The page keeps its trust and decisions.
func (p *webPage) Load(html string) {
	if !p.isDestroyed() {
		p.view.Page().LoadHTML(html, platform.WebPageDocumentURL)
	}
}

func (p *webPage) isDestroyed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.destroyed
}

// Run queues script behind the scripts already queued. A destroyed page ignores it, and a full queue drops it.
func (p *webPage) Run(script string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.destroyed {
		return
	}
	select {
	case p.scripts <- script:
	default:
	}
}

// Destroy closes the web view and stops the script goroutine. Scripts still queued are not run.
func (p *webPage) Destroy() {
	p.mu.Lock()
	if p.destroyed {
		p.mu.Unlock()
		return
	}
	p.destroyed = true
	close(p.scripts)
	p.mu.Unlock()
	p.view.Destroy()
}

func (p *webPage) runScripts() {
	for script := range p.scripts {
		if p.isDestroyed() {
			return
		}
		_, _ = p.view.Page().Eval(script)
	}
}
