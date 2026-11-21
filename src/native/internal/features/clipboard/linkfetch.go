package clipboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Limits of a link preview fetch. The page is cut at LinkMaxBodyBytes (the metadata sits in the head); an image
// larger than that is refused. A fetch gives up after LinkFetchTimeout, and a chain of redirects longer than
// LinkMaxRedirects fails.
const (
	LinkFetchTimeout = 5 * time.Second
	LinkMaxBodyBytes = 512 << 10
	LinkMaxRedirects = 3
	linkImageEdge    = 480
	linkUserAgent    = "Nevermind-LinkPreview"
	linkAcceptHTML   = "text/html,application/xhtml+xml"
	linkAcceptImage  = "image/png,image/jpeg,image/gif"
)

// Errors of the link preview fetch. Every refusal happens before a request is sent, except a destination the
// connect-time check refuses, which the public port reports as platform.ErrNonPublicAddress.
var (
	errLinkScheme      = errors.New("clipboard: link preview supports only http and https")
	errLinkNoHost      = errors.New("clipboard: link preview address has no host")
	errLinkCredentials = errors.New("clipboard: link preview refuses addresses with credentials")
	errLinkNoAddress   = errors.New("clipboard: link preview host has no address")
	errLinkRedirects   = errors.New("clipboard: link preview followed too many redirects")
	errLinkNoLocation  = errors.New("clipboard: link preview redirect has no location")
	errLinkImageType   = errors.New("clipboard: link preview image is not a supported picture")
	errLinkImageTooBig = errors.New("clipboard: link preview image is larger than the limit")
	errLinkStatus      = errors.New("clipboard: link preview address did not answer with success")
)

// NetLinkFetcher is the production LinkFetcher. It sends every request through the public-address port, and
// checks each address the host resolves to before the request, and each redirect target the same way, so a
// preview never contacts a loopback, private, link-local or metadata address.
type NetLinkFetcher struct {
	http     platform.HTTP
	resolver platform.Resolver
}

var _ LinkFetcher = (*NetLinkFetcher)(nil)

// NewLinkFetcher returns a fetcher over the public-address client and the resolver.
func NewLinkFetcher(http platform.HTTP, resolver platform.Resolver) *NetLinkFetcher {
	return &NetLinkFetcher{http: http, resolver: resolver}
}

// fetched is one answer of the public client after its redirects.
type fetched struct {
	status      int
	contentType string
	finalURL    string
	body        []byte
}

// FetchHTML returns the page at url when it is HTML. ok is false for any other content type, and the body is
// cut at LinkMaxBodyBytes.
func (f *NetLinkFetcher) FetchHTML(raw string) (Page, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), LinkFetchTimeout)
	defer cancel()
	answer, err := f.get(ctx, raw, linkAcceptHTML, LinkMaxBodyBytes, true)
	if err != nil {
		return Page{}, false, err
	}
	if !isHTMLType(answer.contentType) {
		return Page{}, false, nil
	}
	return Page{HTML: string(answer.body), FinalURL: answer.finalURL}, true, nil
}

// FetchImage returns the picture at raw as a PNG, its longer side at most linkImageEdge. Only PNG, JPEG and GIF pictures
// are read; a larger one is refused.
func (f *NetLinkFetcher) FetchImage(raw string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), LinkFetchTimeout)
	defer cancel()
	answer, err := f.get(ctx, raw, linkAcceptImage, LinkMaxBodyBytes, false)
	if err != nil {
		return nil, err
	}
	if !isPictureType(answer.contentType) {
		return nil, errLinkImageType
	}
	return PreviewPNG(answer.body, linkImageEdge)
}

// get fetches raw, following at most LinkMaxRedirects redirects, and checks every address on the way. A body
// over limit is cut when truncate is set and refused otherwise.
func (f *NetLinkFetcher) get(ctx context.Context, raw, accept string, limit int64, truncate bool) (fetched, error) {
	target := raw
	for redirects := 0; ; redirects++ {
		if err := checkPublicTarget(ctx, f.resolver, target); err != nil {
			return fetched{}, err
		}
		req, err := platform.NewRequest(ctx, "GET", target, nil)
		if err != nil {
			return fetched{}, err
		}
		req.Header.Set("Accept", accept)
		req.Header.Set("User-Agent", linkUserAgent)
		resp, err := f.http.Do(req)
		if err != nil {
			return fetched{}, err
		}
		if isRedirectStatus(resp.StatusCode) {
			location := resp.Header.Get("Location")
			_ = resp.Body.Close()
			if location == "" {
				return fetched{}, errLinkNoLocation
			}
			if redirects >= LinkMaxRedirects {
				return fetched{}, errLinkRedirects
			}
			if target, err = resolveLocation(target, location); err != nil {
				return fetched{}, err
			}
			continue
		}
		return readAnswer(resp, target, limit, truncate)
	}
}

// readAnswer reads a final answer. A non-success status is an error; the body is read through the limit.
func readAnswer(resp *platform.Response, finalURL string, limit int64, truncate bool) (fetched, error) {
	defer resp.Body.Close()
	answer := fetched{status: resp.StatusCode, contentType: resp.Header.Get("Content-Type"), finalURL: finalURL}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return answer, errLinkStatus
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return answer, err
	}
	if int64(len(body)) > limit {
		if !truncate {
			return answer, errLinkImageTooBig
		}
		body = body[:limit]
	}
	answer.body = body
	return answer, nil
}

// checkPublicTarget refuses a target that is not http or https, carries credentials, or names a host that is
// not public. A name is resolved and every address it resolves to must be public.
func checkPublicTarget(ctx context.Context, resolver platform.Resolver, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errLinkScheme
	}
	if parsed.User != nil {
		return errLinkCredentials
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return errLinkNoHost
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return requirePublic(addr)
	}
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return platform.ErrNonPublicAddress
	}
	addrs, err := resolver.LookupAddrs(ctx, host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return errLinkNoAddress
	}
	for _, addr := range addrs {
		if err := requirePublic(addr); err != nil {
			return err
		}
	}
	return nil
}

// requirePublic returns platform.ErrNonPublicAddress unless addr is a public address.
func requirePublic(addr netip.Addr) error {
	if !platform.IsPublicAddress(addr) {
		return fmt.Errorf("%w: %s", platform.ErrNonPublicAddress, addr)
	}
	return nil
}

// resolveLocation returns the redirect target of location, resolved against the URL that answered it.
func resolveLocation(base, location string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(ref).String(), nil
}

// isRedirectStatus reports whether a status asks for another request at its Location.
func isRedirectStatus(status int) bool {
	switch status {
	case 301, 302, 303, 307, 308:
		return true
	}
	return false
}

// isHTMLType reports whether a content type is an HTML document.
func isHTMLType(contentType string) bool {
	media, _, err := mime.ParseMediaType(contentType)
	return err == nil && (media == "text/html" || media == "application/xhtml+xml")
}

// isPictureType reports whether a content type is one of the pictures the decoder reads.
func isPictureType(contentType string) bool {
	media, _, err := mime.ParseMediaType(contentType)
	return err == nil && (media == "image/png" || media == "image/jpeg" || media == "image/gif")
}
