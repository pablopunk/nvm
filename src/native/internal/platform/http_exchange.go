package platform

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrResponseTooLarge is returned when a response body is longer than the exchange allows.
var ErrResponseTooLarge = errors.New("platform: response is larger than the limit")

// ErrTooManyRedirects is returned when an exchange follows more redirects than it allows.
var ErrTooManyRedirects = errors.New("platform: too many redirects")

// Exchange is one bounded HTTP request through an HTTP port. The exchange follows redirects itself, one hop at a
// time, so Check sees every destination before it is contacted and the port's address policy applies to each. The
// caller bounds the whole exchange through its context.
type Exchange struct {
	Method       string
	URL          string
	Headers      map[string]string
	Body         []byte
	MaxBytes     int64
	MaxRedirects int
	// Check validates a destination before it is requested, the first one included. An error stops the exchange and
	// is returned unchanged.
	Check func(target *url.URL) error
}

// ExchangeResult is the final response of an exchange. Header names are lower case and repeated values are joined
// with ", ". Body holds the bytes as read, within MaxBytes.
type ExchangeResult struct {
	Status   int
	FinalURL string
	Headers  map[string]string
	Body     []byte
}

// exchangeHop is one request the exchange sends: a method, an address and a body. Redirects produce new hops.
type exchangeHop struct {
	method  string
	target  *url.URL
	headers http.Header
	body    []byte
}

// FetchExchange runs ex through h and returns the final response. Redirects with a Location header are followed:
// 303 always, and 301 and 302 for a POST, become a GET without a body; 307 and 308 keep the method and body. An
// Authorization header is dropped when the host changes.
func FetchExchange(ctx context.Context, h HTTP, ex Exchange) (ExchangeResult, error) {
	response, target, err := OpenExchange(ctx, h, ex, true)
	if err != nil {
		return ExchangeResult{}, err
	}
	return readExchangeResult(response, target, ex.MaxBytes)
}

// OpenExchange applies the same per-hop policy as FetchExchange. The caller owns and closes the final body.
func OpenExchange(ctx context.Context, h HTTP, ex Exchange, follow bool) (*Response, *url.URL, error) {
	target, err := url.Parse(ex.URL)
	if err != nil {
		return nil, nil, err
	}
	current := exchangeHop{method: ex.Method, target: target, headers: headerFrom(ex.Headers), body: ex.Body}
	for hops := 0; ; hops++ {
		if err := checkDestination(ex, current.target); err != nil {
			return nil, nil, err
		}
		response, err := sendExchangeHop(ctx, h, current)
		if err != nil {
			return nil, nil, err
		}
		location := response.Header.Get("Location")
		if !follow || !isRedirectStatus(response.StatusCode) || location == "" {
			return response, current.target, nil
		}
		drainAndClose(response)
		if hops >= ex.MaxRedirects {
			return nil, nil, ErrTooManyRedirects
		}
		next, err := redirectedHop(current, response.StatusCode, location)
		if err != nil {
			return nil, nil, err
		}
		current = next
	}
}

func checkDestination(ex Exchange, target *url.URL) error {
	if ex.Check == nil {
		return nil
	}
	return ex.Check(target)
}

func headerFrom(values map[string]string) http.Header {
	headers := http.Header{}
	for name, value := range values {
		headers.Set(name, value)
	}
	return headers
}

func sendExchangeHop(ctx context.Context, h HTTP, hop exchangeHop) (*Response, error) {
	var body io.Reader
	if len(hop.body) > 0 {
		body = strings.NewReader(string(hop.body))
	}
	req, err := NewRequest(ctx, hop.method, hop.target.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header = hop.headers.Clone()
	return h.Do(req)
}

// readExchangeResult reads the final body within maxBytes. A longer body is ErrResponseTooLarge, never a cut answer.
func readExchangeResult(response *Response, target *url.URL, maxBytes int64) (ExchangeResult, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return ExchangeResult{}, err
	}
	if int64(len(body)) > maxBytes {
		return ExchangeResult{}, ErrResponseTooLarge
	}
	return ExchangeResult{
		Status:   response.StatusCode,
		FinalURL: target.String(),
		Headers:  flattenExchangeHeaders(response.Header),
		Body:     body,
	}, nil
}

func flattenExchangeHeaders(header http.Header) map[string]string {
	out := make(map[string]string, len(header))
	for name, values := range header {
		out[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return out
}

func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

// redirectedHop returns the next request for a redirect response. Its location is resolved against the current
// address; the method and body follow the rules in FetchExchange.
func redirectedHop(current exchangeHop, status int, location string) (exchangeHop, error) {
	next, err := current.target.Parse(location)
	if err != nil {
		return exchangeHop{}, err
	}
	following := current
	following.target = next
	following.headers = current.headers.Clone()
	if next.Host != current.target.Host {
		following.headers.Del("Authorization")
	}
	if status == http.StatusSeeOther || ((status == http.StatusMovedPermanently || status == http.StatusFound) && current.method == http.MethodPost) {
		following.method = http.MethodGet
		following.body = nil
	}
	return following, nil
}

// drainAndClose reads a little of a redirect body so the connection can be reused, then closes it.
func drainAndClose(response *Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
	_ = response.Body.Close()
}
