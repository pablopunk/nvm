package clipboard

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fetchHarness is a NetLinkFetcher over the scripted public client and the fake resolver. Requests records every
// request that reached the client, so a refusal that sent nothing is visible.
type fetchHarness struct {
	http     *fake.HTTP
	resolver *fake.Resolver
	fetcher  *NetLinkFetcher
}

func newFetchHarness() fetchHarness {
	h := fake.NewHTTP()
	r := fake.NewResolver()
	r.Map("public.example", "93.184.216.34")
	r.Map("mixed.example", "93.184.216.34", "10.0.0.5")
	r.Map("internal.example", "10.1.2.3")
	r.Map("metadata.example", "169.254.169.254")
	return fetchHarness{http: h, resolver: r, fetcher: NewLinkFetcher(h, r)}
}

// htmlResponse is a successful HTML answer.
func htmlResponse(body string) fake.Response {
	return fake.Response{Status: http.StatusOK, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: []byte(body)}
}

// redirectTo is a 302 answer that sends the client to location.
func redirectTo(location string) fake.Response {
	return fake.Response{Status: http.StatusFound, Header: http.Header{"Location": {location}}}
}

// TestLinkFetchRefusesNonPublicDestinationsBeforeAnyRequest covers the SSRF vectors named in the port contract:
// IPv4 loopback, private, link-local and metadata literals, IPv6 loopback and unique-local literals, the
// localhost name, a .local name, and names that resolve to private addresses (alone or among public ones).
func TestLinkFetchRefusesNonPublicDestinationsBeforeAnyRequest(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/admin",
		"http://10.1.2.3/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"http://[fd00::5]/",
		"http://localhost:8080/",
		"http://printer.local/",
		"http://internal.example/",
		"http://metadata.example/",
		"http://mixed.example/",
		"http://0.0.0.0/",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			h := newFetchHarness()
			_, _, err := h.fetcher.FetchHTML(raw)
			if !errors.Is(err, platform.ErrNonPublicAddress) {
				t.Fatalf("FetchHTML(%s) = %v, want ErrNonPublicAddress", raw, err)
			}
			if got := h.http.Requests(); len(got) != 0 {
				t.Fatalf("a refused destination sent %d requests", len(got))
			}
		})
	}
}

// TestLinkFetchRefusesSchemesAndCredentials: only http and https are fetched, and an address with credentials is
// refused.
func TestLinkFetchRefusesSchemesAndCredentials(t *testing.T) {
	h := newFetchHarness()
	for _, raw := range []string{"ftp://public.example/file", "file:///etc/passwd", "gopher://public.example/", "https://user:pass@public.example/"} {
		if _, _, err := h.fetcher.FetchHTML(raw); err == nil {
			t.Fatalf("FetchHTML(%s) succeeded", raw)
		}
	}
	if got := h.http.Requests(); len(got) != 0 {
		t.Fatalf("refused addresses sent %d requests", len(got))
	}
}

// TestLinkFetchReturnsHTMLFromPublicHost: a public name is resolved, checked and fetched; the page comes back with
// its final address.
func TestLinkFetchReturnsHTMLFromPublicHost(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(htmlResponse(`<title>Hi</title>`))
	page, ok, err := h.fetcher.FetchHTML("https://public.example/post")
	if err != nil || !ok {
		t.Fatalf("FetchHTML = %v, %v", ok, err)
	}
	if page.HTML != `<title>Hi</title>` || page.FinalURL != "https://public.example/post" {
		t.Fatalf("page = %+v", page)
	}
	if got := h.resolver.Lookups(); len(got) != 1 || got[0] != "public.example" {
		t.Fatalf("lookups = %v", got)
	}
}

// TestLinkFetchRejectsRedirectToPrivateAddress: the first hop is public and answers with a redirect to the
// metadata address; the second hop is refused before any request is sent.
func TestLinkFetchRejectsRedirectToPrivateAddress(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(redirectTo("http://169.254.169.254/latest/meta-data/"))
	_, _, err := h.fetcher.FetchHTML("https://public.example/go")
	if !errors.Is(err, platform.ErrNonPublicAddress) {
		t.Fatalf("FetchHTML = %v, want ErrNonPublicAddress", err)
	}
	if got := h.http.Requests(); len(got) != 1 {
		t.Fatalf("requests = %d, want only the public first hop", len(got))
	}
}

// TestLinkFetchRejectsRedirectToNameThatResolvesPrivate: a redirect to a name is checked by resolving it.
func TestLinkFetchRejectsRedirectToNameThatResolvesPrivate(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(redirectTo("https://internal.example/admin"))
	_, _, err := h.fetcher.FetchHTML("https://public.example/go")
	if !errors.Is(err, platform.ErrNonPublicAddress) {
		t.Fatalf("FetchHTML = %v, want ErrNonPublicAddress", err)
	}
	if got := h.http.Requests(); len(got) != 1 {
		t.Fatalf("requests = %d, want 1", len(got))
	}
}

// TestLinkFetchFollowsThreeRedirectsAndNoMore: three redirects are followed; a fourth fails.
func TestLinkFetchFollowsThreeRedirectsAndNoMore(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(redirectTo("/a"), redirectTo("/b"), redirectTo("/c"), htmlResponse(`<title>end</title>`))
	page, ok, err := h.fetcher.FetchHTML("https://public.example/start")
	if err != nil || !ok || page.FinalURL != "https://public.example/c" {
		t.Fatalf("three redirects: page %+v ok %v err %v", page, ok, err)
	}

	h = newFetchHarness()
	h.http.Script(redirectTo("/a"), redirectTo("/b"), redirectTo("/c"), redirectTo("/d"), htmlResponse(`<title>x</title>`))
	if _, _, err := h.fetcher.FetchHTML("https://public.example/start"); !errors.Is(err, errLinkRedirects) {
		t.Fatalf("four redirects: %v, want errLinkRedirects", err)
	}
}

// TestLinkFetchIgnoresNonHTMLAndRefusesFailures: a non-HTML answer is not a page, and a non-success status is an
// error.
func TestLinkFetchIgnoresNonHTMLAndRefusesFailures(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(fake.Response{Status: http.StatusOK, Header: http.Header{"Content-Type": {"application/pdf"}}, Body: []byte("%PDF")})
	if _, ok, err := h.fetcher.FetchHTML("https://public.example/doc.pdf"); ok || err != nil {
		t.Fatalf("pdf: ok %v err %v, want not ok and no error", ok, err)
	}

	h = newFetchHarness()
	h.http.Script(fake.Response{Status: http.StatusNotFound, Header: http.Header{"Content-Type": {"text/html"}}, Body: []byte("gone")})
	if _, _, err := h.fetcher.FetchHTML("https://public.example/missing"); !errors.Is(err, errLinkStatus) {
		t.Fatalf("404: %v, want errLinkStatus", err)
	}
}

// TestLinkFetchCapsHTMLBodyAtTheLimit: a page longer than LinkMaxBodyBytes is cut at the limit, so the head is
// read and the rest is never kept.
func TestLinkFetchCapsHTMLBodyAtTheLimit(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(htmlResponse(strings.Repeat("x", LinkMaxBodyBytes+4096)))
	page, ok, err := h.fetcher.FetchHTML("https://public.example/big")
	if err != nil || !ok {
		t.Fatalf("FetchHTML = %v, %v", ok, err)
	}
	if len(page.HTML) != LinkMaxBodyBytes {
		t.Fatalf("kept %d bytes, want %d", len(page.HTML), LinkMaxBodyBytes)
	}
}

// TestLinkFetchLimitsMatchThePlan pins the numbers of the plan: 5 s, 512 KB, three redirects.
func TestLinkFetchLimitsMatchThePlan(t *testing.T) {
	if LinkFetchTimeout != 5*time.Second || LinkMaxBodyBytes != 512<<10 || LinkMaxRedirects != 3 {
		t.Fatalf("limits = %v, %d, %d", LinkFetchTimeout, LinkMaxBodyBytes, LinkMaxRedirects)
	}
}

// pngOf encodes a w by h picture with a solid fill.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestLinkFetchImageIsDownsizedPNG: a wide picture comes back as a PNG whose longer side is linkImageEdge, and its
// aspect ratio is kept.
func TestLinkFetchImageIsDownsizedPNG(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(fake.Response{Status: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}}, Body: pngOf(t, 1000, 500)})
	data, err := h.fetcher.FetchImage("https://public.example/cover.png")
	if err != nil {
		t.Fatalf("FetchImage: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("result is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != linkImageEdge || b.Dy() != 240 {
		t.Fatalf("size = %dx%d, want %dx240", b.Dx(), b.Dy(), linkImageEdge)
	}
}

// TestLinkFetchImageRefusesOtherTypesAndOversizedBodies: a picture type the decoder does not read, and a body over
// the limit, are refused.
func TestLinkFetchImageRefusesOtherTypesAndOversizedBodies(t *testing.T) {
	h := newFetchHarness()
	h.http.Script(fake.Response{Status: http.StatusOK, Header: http.Header{"Content-Type": {"image/svg+xml"}}, Body: []byte("<svg/>")})
	if _, err := h.fetcher.FetchImage("https://public.example/a.svg"); !errors.Is(err, errLinkImageType) {
		t.Fatalf("svg: %v, want errLinkImageType", err)
	}

	h = newFetchHarness()
	huge := bytes.Repeat([]byte{0}, LinkMaxBodyBytes+1)
	h.http.Script(fake.Response{Status: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}}, Body: huge})
	if _, err := h.fetcher.FetchImage("https://public.example/huge.png"); !errors.Is(err, errLinkImageTooBig) {
		t.Fatalf("oversized: %v, want errLinkImageTooBig", err)
	}
}

// TestLinkFetchImageFromPrivateAddressIsRefused: the image host goes through the same check as the page.
func TestLinkFetchImageFromPrivateAddressIsRefused(t *testing.T) {
	h := newFetchHarness()
	if _, err := h.fetcher.FetchImage("http://10.0.0.9/x.png"); !errors.Is(err, platform.ErrNonPublicAddress) {
		t.Fatalf("FetchImage = %v, want ErrNonPublicAddress", err)
	}
	if len(h.http.Requests()) != 0 {
		t.Fatal("a private image address was contacted")
	}
}

// TestCheckPublicTargetCoversEveryAddressOfAName checks the name path directly: every resolved address must be
// public, and an empty answer is refused.
func TestCheckPublicTargetCoversEveryAddressOfAName(t *testing.T) {
	r := fake.NewResolver()
	r.Map("empty.example")
	if err := checkPublicTarget(context.Background(), r, "https://empty.example/"); !errors.Is(err, errLinkNoAddress) {
		t.Fatalf("empty answer: %v", err)
	}
	r.Map("two.example", "8.8.8.8", "::1")
	if err := checkPublicTarget(context.Background(), r, "https://two.example/"); !errors.Is(err, platform.ErrNonPublicAddress) {
		t.Fatalf("mixed answer: %v", err)
	}
	if err := checkPublicTarget(context.Background(), r, "https://unknown.example/"); err == nil {
		t.Fatal("an unmapped name passed")
	}
}
