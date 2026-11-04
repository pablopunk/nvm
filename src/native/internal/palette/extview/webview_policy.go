package extview

import (
	"net/url"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// pagePermissions maps the webview permissions an extension may declare to the page permissions the system asks
// for. The others (display capture, clipboard) have no page request in the system web view, so they are never
// granted: a page that asks for them is refused.
var pagePermissions = map[api.WebviewPermission]string{
	api.WebviewCamera:     "camera",
	api.WebviewMicrophone: "microphone",
}

// webviewGrant decides the page's permission requests from the permissions the view declares. A request is granted
// when every kind in it is declared, which is the rule of the iframe's allow list.
func webviewGrant(declared []api.WebviewPermission) func(kinds []string) bool {
	allowed := map[string]bool{}
	for _, permission := range declared {
		if kind, ok := pagePermissions[permission]; ok {
			allowed[kind] = true
		}
	}
	return func(kinds []string) bool {
		if len(kinds) == 0 {
			return false
		}
		for _, kind := range kinds {
			if !allowed[kind] {
				return false
			}
		}
		return true
	}
}

// webviewNavigationAllowed keeps an untrusted page in its own document, with or without a fragment. A page's first
// blank document is about:blank, which is its own too. Every other URL, including another origin, goes to the system.
func webviewNavigationAllowed(rawURL string) bool {
	document, _, _ := strings.Cut(rawURL, "#")
	if document == "about:blank" {
		return true
	}
	return strings.TrimSuffix(document, "/") == strings.TrimSuffix(platform.WebPageDocumentURL, "/")
}

// safeExternalURL returns the URL the system may open outside the app: http and https with a host, and mailto. Any
// other scheme is refused, as Electron refuses it.
func safeExternalURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	switch parsed.Scheme {
	case "http", "https":
		if parsed.Host == "" {
			return "", false
		}
		return parsed.String(), true
	case "mailto":
		return parsed.String(), true
	}
	return "", false
}

// externalOpener opens only the URLs safeExternalURL accepts, so a page cannot reach a file or a custom scheme by
// asking for it.
func externalOpener(open func(url string)) func(raw string) {
	return func(raw string) {
		if safe, ok := safeExternalURL(raw); ok {
			open(safe)
		}
	}
}
