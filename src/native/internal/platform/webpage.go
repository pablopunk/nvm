package platform

// WebPageDocumentURL is where every page's document lives. It is a secure origin that no site serves, so the
// media APIs of a page (which need a secure context) work the way they do in the Electron palette, and it is the one
// document a page may stay in: the navigation policy of a view compares against it.
const WebPageDocumentURL = "https://page.nvm.invalid/"

// WebPages is the intent "show a page of web content inside the app's window": a page the system renders beside
// native controls, which a view places where it draws it. The page is untrusted: it calls no Go code, and every
// permission, navigation, window open and title change is decided by the callbacks of its WebPageSpec. Systems
// without a web view return ErrUnsupported from Open, and capability CapabilityWebView is then absent.
type WebPages interface {
	Open(spec WebPageSpec) (WebPage, error)
}

// WebPageSpec configures one page. A nil callback denies (Grant, Navigate) or ignores (OpenExternal, Title).
type WebPageSpec struct {
	// Grant decides a request for the named permissions, such as "camera" or "microphone". The request is
	// granted only when Grant returns true for all of them.
	Grant func(kinds []string) bool
	// Navigate decides whether the page may load url in itself. A refused navigation goes to OpenExternal.
	Navigate func(url string) bool
	// OpenExternal receives a URL the page asked to open outside itself: a refused navigation or a new window.
	OpenExternal func(url string)
	// Title receives the document title each time the page sets it. It is the only way a page's own script
	// reports back to the host, so the title carries no more than the view reads from it.
	Title func(title string)
}

// WebPage is one page of a WebPages.Open. Its methods are used on the UI thread. Run queues its script behind the
// scripts already queued, so the page sees them in the order they were given.
type WebPage interface {
	// Native returns the system's view for a palette web view element to place, or nil once destroyed.
	Native() any
	// Load replaces the page's document with html.
	Load(html string)
	// Run evaluates script in the page. Its result is not read.
	Run(script string)
	// Destroy closes the page. It is safe to call more than once.
	Destroy()
}
