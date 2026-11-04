package services

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

const (
	maxFetchURLBytes = 2048
	maxFetchHeaders  = 32
)

var (
	allowedFetchMethods = map[string]bool{
		http.MethodGet: true, http.MethodHead: true, http.MethodPost: true,
		http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true,
	}
	headerNamePattern = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]+$")
	// forbiddenFetchHeaders are the hop-by-hop headers and the ones the transport sets itself.
	forbiddenFetchHeaders = map[string]bool{
		"Host": true, "Content-Length": true, "Connection": true, "Keep-Alive": true, "Transfer-Encoding": true,
		"Upgrade": true, "Te": true, "Trailer": true, "Proxy-Authorization": true, "Proxy-Connection": true,
	}
)

// fetchPlan is a validated request: what to send, where, with which limits.
type fetchPlan struct {
	method   string
	target   *url.URL
	headers  http.Header
	body     []byte
	timeout  time.Duration
	maxBytes int64
}

// planFetch validates a request and fills in its defaults and caps. Nothing is sent until the plan is valid.
func planFetch(request api.FetchRequest) (fetchPlan, error) {
	method := strings.ToUpper(request.Method)
	if method == "" {
		method = http.MethodGet
	}
	if !allowedFetchMethods[method] {
		return fetchPlan{}, invalid("network method %q is not allowed; use GET, HEAD, POST, PUT, PATCH or DELETE", request.Method)
	}
	target, err := parseFetchURL(request.URL)
	if err != nil {
		return fetchPlan{}, err
	}
	if len(request.Body) > maxFetchBodyBytes {
		return fetchPlan{}, quota(fmt.Errorf("a request body may be at most %d KiB", maxFetchBodyBytes>>10))
	}
	if len(request.Body) > 0 && (method == http.MethodGet || method == http.MethodHead) {
		return fetchPlan{}, invalid("a %s request has no body", method)
	}
	headers, err := planFetchHeaders(request.Headers)
	if err != nil {
		return fetchPlan{}, err
	}
	timeout, err := boundedDuration(request.TimeoutMs, defaultFetchTimeout, maxFetchTimeout)
	if err != nil {
		return fetchPlan{}, err
	}
	maxBytes, err := boundedBytes(request.MaxBytes, defaultFetchBytes, maxFetchBytes)
	if err != nil {
		return fetchPlan{}, err
	}
	return fetchPlan{
		method:   method,
		target:   target,
		headers:  headers,
		body:     []byte(request.Body),
		timeout:  timeout,
		maxBytes: maxBytes,
	}, nil
}

// parseFetchURL accepts an absolute http or https address with a host and no user name.
func parseFetchURL(raw string) (*url.URL, error) {
	if raw == "" || len(raw) > maxFetchURLBytes {
		return nil, invalid("a network address must be non-empty and at most %d bytes", maxFetchURLBytes)
	}
	target, err := url.Parse(raw)
	if err != nil {
		return nil, invalid("the network address is not a valid URL")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, invalid("only http and https addresses can be fetched")
	}
	if target.Host == "" {
		return nil, invalid("the network address has no host")
	}
	if target.User != nil {
		return nil, invalid("network addresses with a user name are refused")
	}
	return target, nil
}

func planFetchHeaders(raw map[string]string) (http.Header, error) {
	if len(raw) > maxFetchHeaders {
		return nil, invalid("a request may carry at most %d headers", maxFetchHeaders)
	}
	headers := http.Header{}
	for name, value := range raw {
		canonical := http.CanonicalHeaderKey(name)
		if !headerNamePattern.MatchString(name) || forbiddenFetchHeaders[canonical] {
			return nil, invalid("the header %q cannot be set by an extension", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, invalid("the header %q has a line break in its value", name)
		}
		headers.Set(canonical, value)
	}
	return headers, nil
}

func boundedDuration(ms int64, fallback, limit time.Duration) (time.Duration, error) {
	if ms < 0 {
		return 0, invalid("a timeout cannot be negative")
	}
	if ms == 0 {
		return fallback, nil
	}
	return min(time.Duration(ms)*time.Millisecond, limit), nil
}

func boundedBytes(requested, fallback, limit int64) (int64, error) {
	if requested < 0 {
		return 0, invalid("a size limit cannot be negative")
	}
	if requested == 0 {
		return fallback, nil
	}
	return min(requested, limit), nil
}

// redirect returns the plan for the next hop. The address must pass the same checks as the first one. A redirect
// that changes the method drops the body; a change of host drops the Authorization header.
func (p fetchPlan) redirect(status int, location string) (fetchPlan, error) {
	next, err := p.target.Parse(location)
	if err != nil {
		return fetchPlan{}, unavailable("the redirect target is not a valid address")
	}
	next, err = parseFetchURL(next.String())
	if err != nil {
		return fetchPlan{}, unavailable("the redirect target is refused: %s", err.Error())
	}
	following := p
	following.target = next
	following.headers = p.headers.Clone()
	if next.Host != p.target.Host {
		following.headers.Del("Authorization")
	}
	if status == http.StatusSeeOther || ((status == http.StatusMovedPermanently || status == http.StatusFound) && p.method == http.MethodPost) {
		following.method = http.MethodGet
		following.body = nil
	}
	return following, nil
}
