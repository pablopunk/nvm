package extview

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestWebviewNavigationTable proves an untrusted page stays in its own document: any other URL goes to the system.
func TestWebviewNavigationTable(t *testing.T) {
	cases := map[string]bool{
		"about:blank":                         true,
		"about:blank#section":                 true,
		"https://page.nvm.invalid/":           true,
		"https://page.nvm.invalid":            true,
		"https://page.nvm.invalid/#x":         true,
		"https://page.nvm.invalid/x":          false,
		"https://page.nvm.invalid.evil.test/": false,
		"about:blankx":                        false,
		"https://example.test/":               false,
		"http://example.test/":                false,
		"file:///etc/passwd":                  false,
		"javascript:alert(1)":                 false,
		"data:text/html,<p>hi</p>":            false,
		"blob:https://example.test/x":         false,
		"":                                    false,
	}
	for url, want := range cases {
		if got := webviewNavigationAllowed(url); got != want {
			t.Errorf("webviewNavigationAllowed(%q) = %v, want %v", url, got, want)
		}
	}
}

// TestSafeExternalURLTable proves only http, https with a host, and mailto leave the app, as Electron allows.
func TestSafeExternalURLTable(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"https://example.test/a?b=1", "https://example.test/a?b=1", true},
		{"  http://example.test  ", "http://example.test", true},
		{"mailto:someone@example.test", "mailto:someone@example.test", true},
		{"https://", "", false},
		{"file:///etc/hosts", "", false},
		{"javascript:alert(1)", "", false},
		{"nvm://account", "", false},
		{"not a url", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := safeExternalURL(c.raw)
		if ok != c.ok || got != c.want {
			t.Errorf("safeExternalURL(%q) = (%q, %v), want (%q, %v)", c.raw, got, ok, c.want, c.ok)
		}
	}
}

// TestExternalOpenerOnlyOpensSafeURLs proves a refused URL never reaches the system opener.
func TestExternalOpenerOnlyOpensSafeURLs(t *testing.T) {
	var opened []string
	open := externalOpener(func(url string) { opened = append(opened, url) })
	open("https://example.test/")
	open("file:///Users/someone/secret.txt")
	open("javascript:alert(1)")
	if want := []string{"https://example.test/"}; !slices.Equal(opened, want) {
		t.Fatalf("opened = %v, want %v", opened, want)
	}
}

// TestWebviewGrantFollowsDeclaredPermissions proves a page gets a request only when every kind it asks for was
// declared, and that kinds with no system counterpart are never granted.
func TestWebviewGrantFollowsDeclaredPermissions(t *testing.T) {
	none := webviewGrant(nil)
	if none([]string{"camera"}) || none(nil) {
		t.Fatal("an undeclared page was granted a permission")
	}
	camera := webviewGrant([]api.WebviewPermission{api.WebviewCamera})
	if !camera([]string{"camera"}) {
		t.Fatal("a declared camera request was refused")
	}
	if camera([]string{"camera", "microphone"}) {
		t.Fatal("a request for an undeclared microphone was granted with the camera")
	}
	if camera([]string{}) {
		t.Fatal("an empty request was granted")
	}
	clipboard := webviewGrant([]api.WebviewPermission{api.WebviewClipboardRead, api.WebviewDisplayCapture})
	if clipboard([]string{"clipboard-read"}) || clipboard([]string{"display-capture"}) {
		t.Fatal("a permission with no page request was granted")
	}
}
