package services

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	maxFetchURLBytes = 2048
	maxFetchHeaders  = 32
	defaultUserAgent = "Nevermind-Extension"
)

var (
	allowedFetchMethods = map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	headerNamePattern   = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]+$")
	// forbiddenFetchHeaders are the hop-by-hop headers and the ones the transport sets itself, lower case.
	forbiddenFetchHeaders = map[string]bool{
		"host": true, "content-length": true, "connection": true, "keep-alive": true, "transfer-encoding": true,
		"upgrade": true, "te": true, "trailer": true, "proxy-authorization": true, "proxy-connection": true,
	}
)

// fetchPlan is a validated request: what to send, where, with which limits.
type fetchPlan struct {
	method   string
	target   *url.URL
	headers  map[string]string
	body     []byte
	timeout  time.Duration
	maxBytes int64
}

// exchange is the platform request the plan describes. It always carries a user agent.
func (p fetchPlan) exchange() platform.Exchange {
	headers := make(map[string]string, len(p.headers)+1)
	for name, value := range p.headers {
		headers[name] = value
	}
	if !hasHeader(headers, "User-Agent") {
		headers["User-Agent"] = defaultUserAgent
	}
	return platform.Exchange{
		Method:       p.method,
		URL:          p.target.String(),
		Headers:      headers,
		Body:         p.body,
		MaxBytes:     p.maxBytes,
		MaxRedirects: maxFetchRedirects,
		Check:        checkFetchTarget,
	}
}

func hasHeader(headers map[string]string, name string) bool {
	for existing := range headers {
		if strings.EqualFold(existing, name) {
			return true
		}
	}
	return false
}

// planFetch validates a request and fills in its defaults and caps. Nothing is sent until the plan is valid.
func planFetch(request api.FetchRequest) (fetchPlan, error) {
	method := strings.ToUpper(request.Method)
	if method == "" {
		method = "GET"
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
	if len(request.Body) > 0 && (method == "GET" || method == "HEAD") {
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
	if err := validateFetchTarget(target); err != nil {
		return nil, err
	}
	return target, nil
}

// validateFetchTarget checks one address the way parseFetchURL checks the first one.
func validateFetchTarget(target *url.URL) error {
	if len(target.String()) > maxFetchURLBytes {
		return invalid("a network address must be at most %d bytes", maxFetchURLBytes)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return invalid("only http and https addresses can be fetched")
	}
	if target.Host == "" {
		return invalid("the network address has no host")
	}
	if target.User != nil {
		return invalid("network addresses with a user name are refused")
	}
	return nil
}

func planFetchHeaders(raw map[string]string) (map[string]string, error) {
	if len(raw) > maxFetchHeaders {
		return nil, invalid("a request may carry at most %d headers", maxFetchHeaders)
	}
	headers := make(map[string]string, len(raw))
	for name, value := range raw {
		if !headerNamePattern.MatchString(name) || forbiddenFetchHeaders[strings.ToLower(name)] {
			return nil, invalid("the header %q cannot be set by an extension", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, invalid("the header %q has a line break in its value", name)
		}
		headers[name] = value
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
