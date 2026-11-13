package clipboard

import (
	"testing"
)

// Cases ported from 1.x/electron/clipboard/link-preview.test.ts. The fetcher is a fake: this package
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
