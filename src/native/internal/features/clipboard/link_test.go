package clipboard

import (
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
)

// Cases ported from src/app/electron/clipboard/link-preview.test.ts. The fetcher is a fake: this package
// never touches the network, and the app's implementation enforces the public-address rules.

// TestSingleWebURLAcceptsOnlyOnePublicURL ports 'singleWebUrl accepts only one public http(s) URL'.
func TestSingleWebURLAcceptsOnlyOnePublicURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{" https://github.com/a/b/pull/1 ", "https://github.com/a/b/pull/1"},
		{"see https://github.com", ""},
		{"ftp://example.com/file", ""},
		{"http://localhost:3000", ""},
		{"http://192.168.1.2/admin", ""},
		{"http://intranet/page", ""},
		{"http://[::1]/", ""},
		{"http://169.254.169.254/latest", ""},
		{"http://100.64.0.1/", ""},
		{"http://8.8.8.8/", "http://8.8.8.8/"},
	}
	for _, c := range cases {
		if got := SingleWebURL(c.in); got != c.want {
			t.Errorf("SingleWebURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestPublicAddressPolicyCoversMappedAddresses: an IPv4-mapped IPv6 address is judged by its IPv4 form.
func TestPublicAddressPolicyCoversMappedAddresses(t *testing.T) {
	if IsPublicAddress(netip.MustParseAddr("::ffff:10.0.0.1")) {
		t.Fatal("mapped private address judged public")
	}
	if !IsPublicAddress(netip.MustParseAddr("::ffff:8.8.8.8")) {
		t.Fatal("mapped public address judged private")
	}
}

// TestParseLinkMetadataPrefersOpenGraph ports 'parseLinkMetadata prefers Open Graph and resolves relative images'.
func TestParseLinkMetadataPrefersOpenGraph(t *testing.T) {
	html := `<head><title>Fallback &amp; title</title>
    <meta property="og:title" content="PR &quot;40902&quot;">
    <meta content="Adds a thing" property="og:description">
    <meta property="og:site_name" content="GitHub">
    <meta property="og:image" content="/img/card.png"></head>`
	got := ParseLinkMetadata(html, "https://github.com/a/b")
	want := LinkMetadata{Title: `PR "40902"`, Description: "Adds a thing", SiteName: "GitHub", ImageURL: "https://github.com/img/card.png"}
	if got != want {
		t.Fatalf("metadata = %+v, want %+v", got, want)
	}
}

// TestParseLinkMetadataHandlesQuotedGreaterThan ports 'parseLinkMetadata handles quoted > and mixed attribute styles'.
func TestParseLinkMetadataHandlesQuotedGreaterThan(t *testing.T) {
	html := `<meta property="og:title" content="A > B">
    <meta name='description' content='x > "y"'>
    <meta content=plain property=og:site_name>
    <meta property="og:image"
          content="https://cdn.example.com/a.png" />`
	got := ParseLinkMetadata(html, "https://a.dev/")
	want := LinkMetadata{Title: "A > B", Description: `x > "y"`, SiteName: "plain", ImageURL: "https://cdn.example.com/a.png"}
	if got != want {
		t.Fatalf("metadata = %+v, want %+v", got, want)
	}
}

// TestParseLinkMetadataFallsBackToDocumentTitle ports 'parseLinkMetadata falls back to the document title'.
func TestParseLinkMetadataFallsBackToDocumentTitle(t *testing.T) {
	if got := ParseLinkMetadata("<title> Hi </title>", "https://a.dev").Title; got != "Hi" {
		t.Fatalf("title = %q, want Hi", got)
	}
}

// fakeFetcher is a LinkFetcher whose behavior each test sets. It counts page fetches.
type fakeFetcher struct {
	html       func(url string) (Page, bool, error)
	image      func(url string) (string, error)
	htmlCalls  atomic.Int32
	imageCalls chan string
}

// FetchHTML delegates to the test's page function and counts the call.
func (f *fakeFetcher) FetchHTML(url string) (Page, bool, error) {
	f.htmlCalls.Add(1)
	return f.html(url)
}

// FetchImage delegates to the test's image function and reports the address asked for.
func (f *fakeFetcher) FetchImage(url string) (string, error) {
	if f.imageCalls != nil {
		f.imageCalls <- url
	}
	if f.image == nil {
		return "", errors.New("no image")
	}
	return f.image(url)
}

// TestLinkServiceShowsLoadingLoadsOnceAndReportsSettlement ports 'service returns a loading placeholder, loads
// once, and reports settlement'.
func TestLinkServiceShowsLoadingLoadsOnceAndReportsSettlement(t *testing.T) {
	settled := make(chan string, 4)
	fetcher := &fakeFetcher{html: func(string) (Page, bool, error) {
		return Page{HTML: `<meta property="og:title" content="Hello">`, FinalURL: "https://a.dev/"}, true, nil
	}}
	service := NewLinkService(fetcher, func(url string) { settled <- url })
	if got := service.Peek("https://a.dev/").State; got != LinkLoading {
		t.Fatalf("placeholder state = %s", got)
	}
	service.Load("https://a.dev/")
	service.Load("https://a.dev/")
	if got := <-settled; got != "https://a.dev/" {
		t.Fatalf("settled %q", got)
	}
	if calls := fetcher.htmlCalls.Load(); calls != 1 {
		t.Fatalf("page fetched %d times, want once", calls)
	}
	if got := service.Peek("https://a.dev/").Title; got != "Hello" {
		t.Fatalf("title = %q", got)
	}
}

// TestLinkServiceMarksFailuresUnavailableWithoutRetry ports 'service marks failures unavailable without retrying'.
func TestLinkServiceMarksFailuresUnavailableWithoutRetry(t *testing.T) {
	settled := make(chan string, 4)
	fetcher := &fakeFetcher{html: func(string) (Page, bool, error) {
		return Page{}, false, errors.New("offline")
	}}
	service := NewLinkService(fetcher, func(url string) { settled <- url })
	service.Load("https://a.dev/")
	<-settled
	service.Load("https://a.dev/")
	if got := service.Peek("https://a.dev/").State; got != LinkUnavailable {
		t.Fatalf("state = %s, want unavailable", got)
	}
	if calls := fetcher.htmlCalls.Load(); calls != 1 {
		t.Fatalf("page fetched %d times, want once", calls)
	}
}

// TestLinkServiceShowsMetadataThenImage ports 'service shows metadata first, then swaps in the host-fetched image':
// the first settlement has no image and the second carries the data URL.
func TestLinkServiceShowsMetadataThenImage(t *testing.T) {
	images := make(chan string, 4)
	var service *LinkService
	fetcher := &fakeFetcher{
		html: func(string) (Page, bool, error) {
			return Page{HTML: `<meta property="og:title" content="Hi"><meta property="og:image" content="https://cdn.dev/a.png">`, FinalURL: "https://a.dev/"}, true, nil
		},
		image: func(url string) (string, error) {
			if url != "https://cdn.dev/a.png" {
				t.Errorf("image fetched from %q", url)
			}
			return "data:image/png;base64,AAAA", nil
		},
	}
	service = NewLinkService(fetcher, func(url string) { images <- service.Peek(url).Image })
	service.Load("https://a.dev/")
	if first := <-images; first != "" {
		t.Fatalf("first settlement carried an image: %q", first)
	}
	if second := <-images; second != "data:image/png;base64,AAAA" {
		t.Fatalf("second settlement image = %q", second)
	}
}

// TestLinkServiceNeverExposesRemoteImageURL ports 'service never exposes a remote image URL to the renderer': when
// the host cannot fetch the image, the preview has its text and no image.
func TestLinkServiceNeverExposesRemoteImageURL(t *testing.T) {
	settled := make(chan string, 4)
	fetcher := &fakeFetcher{
		html: func(string) (Page, bool, error) {
			return Page{HTML: `<meta property="og:title" content="Hi"><meta property="og:image" content="http://127.0.0.1/x">`, FinalURL: "https://a.dev/"}, true, nil
		},
		image:      func(string) (string, error) { return "", errors.New("blocked") },
		imageCalls: make(chan string, 4),
	}
	service := NewLinkService(fetcher, func(url string) { settled <- url })
	service.Load("https://a.dev/")
	<-settled
	if asked := <-fetcher.imageCalls; asked != "http://127.0.0.1/x" {
		t.Fatalf("image asked for %q", asked)
	}
	got := service.Peek("https://a.dev/")
	if got.Image != "" || got.Title != "Hi" {
		t.Fatalf("status = %+v", got)
	}
}
